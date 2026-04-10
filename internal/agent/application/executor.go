package application

import (
	"context"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type DockerRuntime interface {
	DeployContainer(context.Context, *grpcapi.TaskCommand) (*ContainerRuntime, error)
	InspectHealth(context.Context, string) (string, error)
	FindContainerByName(context.Context, string) (*ManagedContainer, error)
	ResolveListenAddress(context.Context, string, int) (string, error)
	RemoveContainer(context.Context, string) error
	ListManagedContainers(context.Context, string, string) ([]*ManagedContainer, error)
}

type ProxyRuntime interface {
	EnsureReady(context.Context) error
	ApplySnapshot(context.Context, *grpcapi.ProxyConfigSnapshot) error
	SetServerAddress(context.Context, string, string, string, int) error
	EnableServer(context.Context, string, string) error
	DisableServer(context.Context, string, string) error
	ShowStats(context.Context) ([]*grpcapi.BackendStatPoint, error)
}

type ContainerRuntime struct {
	ContainerID   string
	ListenAddress string
}

var ErrProxyNotReady = errors.New("proxy stack not ready")

type Executor struct {
	cfg       *config.AgentRuntimeConfig
	docker    DockerRuntime
	proxy     ProxyRuntime
	httpProbe func(string, string, int, int) error
}

type TaskExecutionError struct {
	Step string
	Err  error
}

func (e *TaskExecutionError) Error() string {
	return e.Err.Error()
}

func (e *TaskExecutionError) Unwrap() error {
	return e.Err
}

func NewExecutor(cfg *config.AgentRuntimeConfig, docker DockerRuntime, proxy ProxyRuntime) *Executor {
	return &Executor{
		cfg:       cfg,
		docker:    docker,
		proxy:     proxy,
		httpProbe: probeHTTP,
	}
}

func (e *Executor) Execute(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	if err := report(&grpcapi.TaskUpdate{
		TaskId: task.GetTaskId(),
		Status: grpcapi.TaskStatus_TASK_STATUS_RUNNING,
		Step:   "accepted",
	}); err != nil {
		return err
	}

	switch task.GetType() {
	case grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN:
		return e.executeDeploy(ctx, task, report)
	case grpcapi.TaskType_TASK_TYPE_SWITCH_TRAFFIC, grpcapi.TaskType_TASK_TYPE_ROLLBACK:
		return e.executeTrafficSwitch(ctx, task, report)
	case grpcapi.TaskType_TASK_TYPE_CLEANUP_OLD:
		return report(&grpcapi.TaskUpdate{
			TaskId: task.GetTaskId(),
			Status: grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
			Step:   "noop",
		})
	default:
		return fmt.Errorf("unknown task type: %s", task.GetType())
	}
}

func (e *Executor) CollectStats(ctx context.Context) ([]*grpcapi.BackendStatPoint, error) {
	return e.proxy.ShowStats(ctx)
}

func (e *Executor) executeDeploy(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	if err := e.proxy.EnsureReady(ctx); err != nil {
		return &TaskExecutionError{Step: "proxy_stack_not_ready", Err: err}
	}
	runtime, reused, err := e.ensureDeployContainer(ctx, task)
	if err != nil {
		return err
	}
	if reused {
		return report(&grpcapi.TaskUpdate{
			TaskId:        task.GetTaskId(),
			Status:        grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
			Step:          "healthy",
			ContainerId:   runtime.ContainerID,
			ListenAddress: runtime.ListenAddress,
			Slot:          task.GetTargetSlot(),
			ServerName:    task.GetServerName(),
		})
	}
	runtime, err = e.docker.DeployContainer(ctx, task)
	if err != nil {
		return err
	}
	if err := report(&grpcapi.TaskUpdate{
		TaskId:        task.GetTaskId(),
		Status:        grpcapi.TaskStatus_TASK_STATUS_RUNNING,
		Step:          "container_started",
		ContainerId:   runtime.ContainerID,
		ListenAddress: runtime.ListenAddress,
		Slot:          task.GetTargetSlot(),
		ServerName:    task.GetServerName(),
	}); err != nil {
		return err
	}

	if task.GetHttpTimeoutSecond() <= 0 {
		task.HttpTimeoutSecond = int32(e.cfg.HTTPProbeTimeoutS)
	}
	if err := e.waitForHealth(ctx, task, runtime); err != nil {
		return err
	}
	return report(&grpcapi.TaskUpdate{
		TaskId:        task.GetTaskId(),
		Status:        grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
		Step:          "healthy",
		ContainerId:   runtime.ContainerID,
		ListenAddress: runtime.ListenAddress,
		Slot:          task.GetTargetSlot(),
		ServerName:    task.GetServerName(),
	})
}

func (e *Executor) executeTrafficSwitch(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	if err := e.proxy.EnsureReady(ctx); err != nil {
		return &TaskExecutionError{Step: "proxy_stack_not_ready", Err: err}
	}
	if err := e.proxy.SetServerAddress(ctx, task.GetBackendName(), task.GetServerName(), ManagedContainerName(task.GetServiceKey(), task.GetTargetSlot()), int(task.GetContainerPort())); err != nil {
		return err
	}
	if err := e.proxy.EnableServer(ctx, task.GetBackendName(), task.GetServerName()); err != nil {
		return err
	}
	if task.GetPreviousServer() != "" {
		if err := e.proxy.DisableServer(ctx, task.GetBackendName(), task.GetPreviousServer()); err != nil {
			return err
		}
	}
	if removed, err := e.cleanupManagedContainers(ctx, task); err != nil {
		_ = report(&grpcapi.TaskUpdate{
			TaskId:       task.GetTaskId(),
			Status:       grpcapi.TaskStatus_TASK_STATUS_RUNNING,
			Step:         "cleanup_failed",
			ErrorMessage: err.Error(),
		})
	} else if removed > 0 {
		_ = report(&grpcapi.TaskUpdate{
			TaskId: task.GetTaskId(),
			Status: grpcapi.TaskStatus_TASK_STATUS_RUNNING,
			Step:   fmt.Sprintf("cleanup_pruned:%d", removed),
		})
	}
	return report(&grpcapi.TaskUpdate{
		TaskId:     task.GetTaskId(),
		Status:     grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
		Step:       "traffic_switched",
		Slot:       task.GetTargetSlot(),
		ServerName: task.GetServerName(),
	})
}

func (e *Executor) ensureDeployContainer(ctx context.Context, task *grpcapi.TaskCommand) (*ContainerRuntime, bool, error) {
	name := ManagedContainerName(task.GetServiceKey(), task.GetTargetSlot())
	existing, err := e.docker.FindContainerByName(ctx, name)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		return nil, false, nil
	}
	if !existing.Managed || existing.AgentID != task.GetAgentId() {
		return nil, false, &TaskExecutionError{
			Step: "managed_container_conflict",
			Err:  fmt.Errorf("managed container conflict: %s exists but is not owned by agent %s", name, task.GetAgentId()),
		}
	}
	if existing.ReleaseID == task.GetReleaseId() {
		listenAddress, err := e.docker.ResolveListenAddress(ctx, existing.ContainerID, int(task.GetContainerPort()))
		if err == nil {
			if err := e.verifyHealth(ctx, task, existing.ContainerID, listenAddress); err == nil {
				return &ContainerRuntime{
					ContainerID:   existing.ContainerID,
					ListenAddress: listenAddress,
				}, true, nil
			}
		}
	}
	if err := e.docker.RemoveContainer(ctx, existing.ContainerID); err != nil {
		return nil, false, err
	}
	return nil, false, nil
}

func (e *Executor) waitForHealth(ctx context.Context, task *grpcapi.TaskCommand, runtime *ContainerRuntime) error {
	if task.GetHttpHealthPath() == "" {
		task.HttpHealthPath = "/health"
	}
	if task.GetHttpExpectedCode() == 0 {
		task.HttpExpectedCode = http.StatusOK
	}
	deadline := time.Now().Add(time.Duration(task.GetHttpTimeoutSecond()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := e.verifyHealth(ctx, task, runtime.ContainerID, runtime.ListenAddress); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timeout for task %s", task.GetTaskId())
		}
		time.Sleep(time.Second)
	}
}

func (e *Executor) verifyHealth(ctx context.Context, task *grpcapi.TaskCommand, containerID string, listenAddress string) error {
	if task.GetDockerHealthCheck() {
		health, err := e.docker.InspectHealth(ctx, containerID)
		if err != nil {
			return err
		}
		if health != "" && health != "healthy" {
			return errors.New("docker health not ready")
		}
	}
	return e.httpProbe(listenAddress, defaultString(task.GetHttpHealthPath(), "/health"), defaultCode(task.GetHttpExpectedCode()), defaultTimeout(task.GetHttpTimeoutSecond(), e.cfg.HTTPProbeTimeoutS))
}

func (e *Executor) cleanupManagedContainers(ctx context.Context, task *grpcapi.TaskCommand) (int, error) {
	items, err := e.docker.ListManagedContainers(ctx, task.GetAgentId(), task.GetServiceKey())
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	preserve := map[string]struct{}{
		ManagedContainerName(task.GetServiceKey(), task.GetTargetSlot()):      {},
		ManagedContainerName(task.GetServiceKey(), task.GetCurrentLiveSlot()): {},
	}
	removed := 0
	var errs []error
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, ok := preserve[item.Name]; ok {
			continue
		}
		if err := e.docker.RemoveContainer(ctx, item.ContainerID); err != nil {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	if len(errs) > 0 {
		return removed, errors.Join(errs...)
	}
	return removed, nil
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultCode(value int32) int {
	if value == 0 {
		return http.StatusOK
	}
	return int(value)
}

func defaultTimeout(value int32, fallback int) int {
	if value > 0 {
		return int(value)
	}
	return fallback
}

func probeHTTP(listenAddress string, path string, expectedCode int, timeoutSeconds int) error {
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}
	resp, err := client.Get("http://" + listenAddress + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedCode {
		return fmt.Errorf("unexpected health status: %d", resp.StatusCode)
	}
	return nil
}

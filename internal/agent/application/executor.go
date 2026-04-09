package application

import (
	"context"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"fmt"
	"net/http"
	"time"
)

type DockerRuntime interface {
	DeployContainer(context.Context, *grpcapi.TaskCommand) (*ContainerRuntime, error)
	InspectHealth(context.Context, string) (string, error)
}

type HAProxyRuntime interface {
	SetServerAddress(context.Context, string, string, string, int) error
	EnableServer(context.Context, string, string) error
	DisableServer(context.Context, string, string) error
	ShowStats(context.Context) ([]*grpcapi.BackendStatPoint, error)
}

type ContainerRuntime struct {
	ContainerID   string
	ListenAddress string
}

type Executor struct {
	cfg     *config.AgentRuntimeConfig
	docker  DockerRuntime
	haproxy HAProxyRuntime
}

func NewExecutor(cfg *config.AgentRuntimeConfig, docker DockerRuntime, haproxy HAProxyRuntime) *Executor {
	return &Executor{
		cfg:     cfg,
		docker:  docker,
		haproxy: haproxy,
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
	return e.haproxy.ShowStats(ctx)
}

func (e *Executor) executeDeploy(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	runtime, err := e.docker.DeployContainer(ctx, task)
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
	if err := e.haproxy.SetServerAddress(ctx, task.GetBackendName(), task.GetServerName(), "127.0.0.1", int(task.GetHostPort())); err != nil {
		return err
	}
	if err := e.haproxy.EnableServer(ctx, task.GetBackendName(), task.GetServerName()); err != nil {
		return err
	}
	if task.GetPreviousServer() != "" {
		if err := e.haproxy.DisableServer(ctx, task.GetBackendName(), task.GetPreviousServer()); err != nil {
			return err
		}
	}
	return report(&grpcapi.TaskUpdate{
		TaskId:     task.GetTaskId(),
		Status:     grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
		Step:       "traffic_switched",
		Slot:       task.GetTargetSlot(),
		ServerName: task.GetServerName(),
	})
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
		health, err := e.docker.InspectHealth(ctx, runtime.ContainerID)
		if err == nil && (health == "" || health == "healthy") {
			if err := probeHTTP(runtime.ListenAddress, task.GetHttpHealthPath(), int(task.GetHttpExpectedCode()), int(task.GetHttpTimeoutSecond())); err == nil {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timeout for task %s", task.GetTaskId())
		}
		time.Sleep(time.Second)
	}
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

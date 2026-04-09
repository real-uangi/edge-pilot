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
	ShowStats(context.Context) ([]grpcapi.BackendStatPoint, error)
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
		TaskID: task.TaskID,
		Status: "running",
		Step:   "accepted",
	}); err != nil {
		return err
	}

	switch task.Type {
	case "deploy_green":
		return e.executeDeploy(ctx, task, report)
	case "switch_traffic", "rollback":
		return e.executeTrafficSwitch(ctx, task, report)
	case "cleanup_old":
		return report(&grpcapi.TaskUpdate{
			TaskID: task.TaskID,
			Status: "succeeded",
			Step:   "noop",
		})
	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}
}

func (e *Executor) CollectStats(ctx context.Context) ([]grpcapi.BackendStatPoint, error) {
	return e.haproxy.ShowStats(ctx)
}

func (e *Executor) executeDeploy(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	runtime, err := e.docker.DeployContainer(ctx, task)
	if err != nil {
		return err
	}
	if err := report(&grpcapi.TaskUpdate{
		TaskID:        task.TaskID,
		Status:        "running",
		Step:          "container_started",
		ContainerID:   runtime.ContainerID,
		ListenAddress: runtime.ListenAddress,
		Slot:          task.TargetSlot,
		ServerName:    task.ServerName,
	}); err != nil {
		return err
	}

	if task.HTTPTimeoutSecond <= 0 {
		task.HTTPTimeoutSecond = e.cfg.HTTPProbeTimeoutS
	}
	if err := e.waitForHealth(ctx, task, runtime); err != nil {
		return err
	}
	return report(&grpcapi.TaskUpdate{
		TaskID:        task.TaskID,
		Status:        "succeeded",
		Step:          "healthy",
		ContainerID:   runtime.ContainerID,
		ListenAddress: runtime.ListenAddress,
		Slot:          task.TargetSlot,
		ServerName:    task.ServerName,
	})
}

func (e *Executor) executeTrafficSwitch(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	if err := e.haproxy.SetServerAddress(ctx, task.BackendName, task.ServerName, "127.0.0.1", task.HostPort); err != nil {
		return err
	}
	if err := e.haproxy.EnableServer(ctx, task.BackendName, task.ServerName); err != nil {
		return err
	}
	if task.PreviousServer != "" {
		if err := e.haproxy.DisableServer(ctx, task.BackendName, task.PreviousServer); err != nil {
			return err
		}
	}
	return report(&grpcapi.TaskUpdate{
		TaskID:     task.TaskID,
		Status:     "succeeded",
		Step:       "traffic_switched",
		Slot:       task.TargetSlot,
		ServerName: task.ServerName,
	})
}

func (e *Executor) waitForHealth(ctx context.Context, task *grpcapi.TaskCommand, runtime *ContainerRuntime) error {
	if task.HTTPHealthPath == "" {
		task.HTTPHealthPath = "/health"
	}
	if task.HTTPExpectedCode == 0 {
		task.HTTPExpectedCode = http.StatusOK
	}
	deadline := time.Now().Add(time.Duration(task.HTTPTimeoutSecond) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		health, err := e.docker.InspectHealth(ctx, runtime.ContainerID)
		if err == nil && (health == "" || health == "healthy") {
			if err := probeHTTP(runtime.ListenAddress, task.HTTPHealthPath, task.HTTPExpectedCode, task.HTTPTimeoutSecond); err == nil {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timeout for task %s", task.TaskID)
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

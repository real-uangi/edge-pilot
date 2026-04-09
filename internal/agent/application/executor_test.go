package application

import (
	"context"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"testing"
)

func TestExecuteDeployReusesHealthyManagedContainer(t *testing.T) {
	docker := &fakeDockerRuntime{
		foundByName: map[string]*ManagedContainer{
			ManagedContainerName("svc-a", grpcapi.Slot_SLOT_GREEN): {
				ContainerRuntime: ContainerRuntime{
					ContainerID: "container-1",
				},
				Name:       ManagedContainerName("svc-a", grpcapi.Slot_SLOT_GREEN),
				Managed:    true,
				AgentID:    "agent-a",
				ServiceKey: "svc-a",
				ReleaseID:  "release-1",
			},
		},
		healthByID: map[string]string{
			"container-1": "",
		},
		listenByID: map[string]string{
			"container-1": "172.29.0.21:8080",
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a", HTTPProbeTimeoutS: 1}, docker, &fakeProxyRuntime{})
	executor.httpProbe = func(string, string, int, int) error { return nil }

	err := executor.Execute(context.Background(), &grpcapi.TaskCommand{
		TaskId:            "task-1",
		ReleaseId:         "release-1",
		ServiceKey:        "svc-a",
		AgentId:           "agent-a",
		Type:              grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN,
		TargetSlot:        grpcapi.Slot_SLOT_GREEN,
		ServerName:        "srv-green",
		ContainerPort:     8080,
		DockerHealthCheck: true,
		HttpHealthPath:    "/health",
		HttpExpectedCode:  0,
		HttpTimeoutSecond: 1,
	}, func(update *grpcapi.TaskUpdate) error { return nil })
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(docker.deployedTasks) != 0 {
		t.Fatalf("expected no new deployment when reusing managed container")
	}
	if len(docker.removedIDs) != 0 {
		t.Fatalf("expected no container removal when reusing managed container")
	}
}

func TestExecuteDeployFailsOnManagedContainerConflict(t *testing.T) {
	docker := &fakeDockerRuntime{
		foundByName: map[string]*ManagedContainer{
			ManagedContainerName("svc-a", grpcapi.Slot_SLOT_GREEN): {
				ContainerRuntime: ContainerRuntime{
					ContainerID: "container-2",
				},
				Name:    ManagedContainerName("svc-a", grpcapi.Slot_SLOT_GREEN),
				Managed: false,
			},
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a", HTTPProbeTimeoutS: 1}, docker, &fakeProxyRuntime{})

	err := executor.Execute(context.Background(), &grpcapi.TaskCommand{
		TaskId:        "task-2",
		ReleaseId:     "release-2",
		ServiceKey:    "svc-a",
		AgentId:       "agent-a",
		Type:          grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN,
		TargetSlot:    grpcapi.Slot_SLOT_GREEN,
		ServerName:    "srv-green",
		ContainerPort: 8080,
	}, func(update *grpcapi.TaskUpdate) error { return nil })
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	execErr, ok := err.(*TaskExecutionError)
	if !ok || execErr.Step != "managed_container_conflict" {
		t.Fatalf("expected managed_container_conflict error, got %#v", err)
	}
}

func TestExecuteTrafficSwitchCleansOnlyCurrentAgentManagedContainers(t *testing.T) {
	docker := &fakeDockerRuntime{
		managedItems: []*ManagedContainer{
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "keep-target"},
				Name:             ManagedContainerName("svc-a", grpcapi.Slot_SLOT_GREEN),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "keep-live"},
				Name:             ManagedContainerName("svc-a", grpcapi.Slot_SLOT_BLUE),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "remove-old"},
				Name:             "ep-svc-a-shadow",
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "other-agent"},
				Name:             "ep-svc-a-foreign",
				Managed:          true,
				AgentID:          "agent-b",
				ServiceKey:       "svc-a",
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "unmanaged"},
				Name:             "random-container",
				Managed:          false,
				ServiceKey:       "svc-a",
			},
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a", HTTPProbeTimeoutS: 1}, docker, &fakeProxyRuntime{})

	err := executor.Execute(context.Background(), &grpcapi.TaskCommand{
		TaskId:          "task-3",
		AgentId:         "agent-a",
		ServiceKey:      "svc-a",
		Type:            grpcapi.TaskType_TASK_TYPE_SWITCH_TRAFFIC,
		BackendName:     "be-api",
		ServerName:      "srv-green",
		PreviousServer:  "srv-blue",
		TargetSlot:      grpcapi.Slot_SLOT_GREEN,
		CurrentLiveSlot: grpcapi.Slot_SLOT_BLUE,
		ContainerPort:   8080,
	}, func(update *grpcapi.TaskUpdate) error { return nil })
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(docker.removedIDs) != 1 || docker.removedIDs[0] != "remove-old" {
		t.Fatalf("expected only stale managed container to be removed, got %#v", docker.removedIDs)
	}
}

func TestExecuteDeployCanSkipDockerHealthCheck(t *testing.T) {
	docker := &fakeDockerRuntime{}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a", HTTPProbeTimeoutS: 1}, docker, &fakeProxyRuntime{})
	executor.httpProbe = func(string, string, int, int) error { return nil }

	err := executor.Execute(context.Background(), &grpcapi.TaskCommand{
		TaskId:            "task-4",
		ReleaseId:         "release-4",
		ServiceKey:        "svc-a",
		AgentId:           "agent-a",
		Type:              grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN,
		TargetSlot:        grpcapi.Slot_SLOT_GREEN,
		ContainerPort:     8080,
		DockerHealthCheck: false,
	}, func(update *grpcapi.TaskUpdate) error { return nil })
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(docker.deployedTasks) != 1 {
		t.Fatalf("expected one deployment, got %d", len(docker.deployedTasks))
	}
}

type fakeDockerRuntime struct {
	foundByName   map[string]*ManagedContainer
	managedItems  []*ManagedContainer
	healthByID    map[string]string
	listenByID    map[string]string
	deployedTasks []*grpcapi.TaskCommand
	removedIDs    []string
}

func (f *fakeDockerRuntime) DeployContainer(ctx context.Context, task *grpcapi.TaskCommand) (*ContainerRuntime, error) {
	f.deployedTasks = append(f.deployedTasks, task)
	return &ContainerRuntime{ContainerID: "new-container", ListenAddress: "172.29.0.22:8080"}, nil
}

func (f *fakeDockerRuntime) InspectHealth(ctx context.Context, containerID string) (string, error) {
	if health, ok := f.healthByID[containerID]; ok {
		return health, nil
	}
	return "", nil
}

func (f *fakeDockerRuntime) FindContainerByName(ctx context.Context, name string) (*ManagedContainer, error) {
	return f.foundByName[name], nil
}

func (f *fakeDockerRuntime) ResolveListenAddress(ctx context.Context, containerID string, port int) (string, error) {
	if listen, ok := f.listenByID[containerID]; ok {
		return listen, nil
	}
	return "172.29.0.22:8080", nil
}

func (f *fakeDockerRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	f.removedIDs = append(f.removedIDs, containerID)
	return nil
}

func (f *fakeDockerRuntime) ListManagedContainers(ctx context.Context, agentID string, serviceKey string) ([]*ManagedContainer, error) {
	out := make([]*ManagedContainer, 0, len(f.managedItems))
	for _, item := range f.managedItems {
		if item == nil {
			continue
		}
		if !item.Managed {
			continue
		}
		if item.AgentID != agentID {
			continue
		}
		if item.ServiceKey != serviceKey {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

type fakeProxyRuntime struct{}

func (f *fakeProxyRuntime) EnsureReady(context.Context) error {
	return nil
}

func (f *fakeProxyRuntime) ApplySnapshot(context.Context, *grpcapi.ProxyConfigSnapshot) error {
	return nil
}

func (f *fakeProxyRuntime) SetServerAddress(context.Context, string, string, string, int) error {
	return nil
}

func (f *fakeProxyRuntime) EnableServer(context.Context, string, string) error {
	return nil
}

func (f *fakeProxyRuntime) DisableServer(context.Context, string, string) error {
	return nil
}

func (f *fakeProxyRuntime) ShowStats(context.Context) ([]*grpcapi.BackendStatPoint, error) {
	return nil, nil
}

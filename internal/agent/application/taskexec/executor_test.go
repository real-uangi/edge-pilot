package taskexec

import (
	"context"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type ContainerRuntime = agentdomain.ContainerRuntime
type ContainerStatus = agentdomain.ContainerStatus
type ManagedContainer = agentdomain.ManagedContainer

func ManagedContainerName(serviceKey string, slot grpcapi.Slot) string {
	return agentdomain.ManagedContainerName(serviceKey, slot)
}

func ManagedContainerNameForRelease(serviceKey string, releaseID string) string {
	return agentdomain.ManagedContainerNameForTask(serviceKey, releaseID, grpcapi.Slot_SLOT_UNSPECIFIED)
}

func TestExecuteDeployReusesHealthyManagedContainer(t *testing.T) {
	docker := &fakeDockerRuntime{
		foundByName: map[string]*ManagedContainer{
			ManagedContainerNameForRelease("svc-a", "release-1"): {
				ContainerRuntime: ContainerRuntime{ContainerID: "container-1"},
				Name:             ManagedContainerNameForRelease("svc-a", "release-1"),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-1",
			},
		},
		statusByID: map[string]*ContainerStatus{
			"container-1": {State: "running", Running: true, Health: "healthy"},
		},
		listenByID: map[string]string{
			"container-1": "172.29.0.21:8080",
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)
	executor.httpProbe = func(string, string, map[string]string, int, int) error { return nil }

	err := executor.Execute(context.Background(), newDeployTaskCommand("release-1"), func(update *grpcapi.TaskUpdate) error { return nil })
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

func TestExecuteDeployPreservesCurrentReleaseContainerUntilHealthy(t *testing.T) {
	docker := &fakeDockerRuntime{
		foundByName: map[string]*ManagedContainer{
			ManagedContainerNameForRelease("svc-a", "release-2"): {
				ContainerRuntime: ContainerRuntime{ContainerID: "container-2"},
				Name:             ManagedContainerNameForRelease("svc-a", "release-2"),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-2",
			},
		},
		statusByID: map[string]*ContainerStatus{
			"container-2": {State: "running", Running: true, Health: "starting"},
		},
		listenByID: map[string]string{
			"container-2": "172.29.0.22:8080",
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)
	executor.httpProbe = func(string, string, map[string]string, int, int) error { return nil }

	err := executor.Execute(context.Background(), newDeployTaskCommand("release-2"), func(update *grpcapi.TaskUpdate) error { return nil })
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(docker.deployedTasks) != 0 {
		t.Fatalf("expected existing release container to be reused for waiting")
	}
	if len(docker.removedIDs) != 0 {
		t.Fatalf("expected current release container not to be removed")
	}
}

func TestExecuteDeployFailsOnManagedContainerConflict(t *testing.T) {
	docker := &fakeDockerRuntime{
		foundByName: map[string]*ManagedContainer{
			ManagedContainerNameForRelease("svc-a", "release-3"): {
				ContainerRuntime: ContainerRuntime{ContainerID: "container-3"},
				Name:             ManagedContainerNameForRelease("svc-a", "release-3"),
				Managed:          false,
			},
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)

	err := executor.Execute(context.Background(), newDeployTaskCommand("release-3"), func(update *grpcapi.TaskUpdate) error { return nil })
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	execErr, ok := err.(*TaskExecutionError)
	if !ok || execErr.Step != "managed_container_conflict" {
		t.Fatalf("expected managed_container_conflict error, got %#v", err)
	}
}

func TestExecuteDeployRetriesTransientHealthFailures(t *testing.T) {
	docker := &fakeDockerRuntime{
		statusByID: map[string]*ContainerStatus{
			"new-container": {State: "running", Running: true, Health: "healthy"},
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)
	attempts := 0
	var capturedHeaders map[string]string
	executor.httpProbe = func(_ string, _ string, headers map[string]string, _ int, _ int) error {
		attempts++
		capturedHeaders = headers
		if attempts == 1 {
			return errors.New("first probe failed")
		}
		return nil
	}

	err := executor.Execute(context.Background(), &grpcapi.TaskCommand{
		TaskId:                  "task-retry",
		ReleaseId:               "release-retry",
		ServiceKey:              "svc-a",
		AgentId:                 "agent-a",
		Type:                    grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN,
		TargetSlot:              grpcapi.Slot_SLOT_GREEN,
		ServerName:              "srv-green",
		ContainerPort:           8080,
		DockerHealthCheck:       true,
		HttpHealthPath:          "/health",
		HttpExpectedCode:        200,
		HttpTimeoutSecond:       5,
		StartupGraceSecond:      1,
		HttpProbeTimeoutSecond:  1,
		HttpProbeIntervalSecond: 1,
		HttpSuccessThreshold:    1,
		HttpHealthHeaders:       map[string]string{"X-Release-Trace": "trace-retry"},
	}, func(update *grpcapi.TaskUpdate) error { return nil })
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if attempts < 2 {
		t.Fatalf("expected at least two health probes, got %d", attempts)
	}
	if capturedHeaders["X-Release-Trace"] != "trace-retry" {
		t.Fatalf("expected custom health headers, got %#v", capturedHeaders)
	}
}

func TestExecuteDeployCollectsLogsAndCleansFailedContainer(t *testing.T) {
	docker := &fakeDockerRuntime{
		statusByID: map[string]*ContainerStatus{
			"new-container": {State: "exited", Running: false, Health: "unhealthy"},
		},
		logsByID: map[string]string{
			"new-container": "boot failed",
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)
	executor.httpProbe = func(string, string, map[string]string, int, int) error { return errors.New("probe failed") }

	err := executor.Execute(context.Background(), &grpcapi.TaskCommand{
		TaskId:                  "task-failed",
		ReleaseId:               "release-failed",
		ServiceKey:              "svc-a",
		AgentId:                 "agent-a",
		Type:                    grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN,
		TargetSlot:              grpcapi.Slot_SLOT_GREEN,
		ServerName:              "srv-green",
		ContainerPort:           8080,
		DockerHealthCheck:       true,
		HttpHealthPath:          "/health",
		HttpExpectedCode:        200,
		HttpTimeoutSecond:       2,
		StartupGraceSecond:      1,
		HttpProbeTimeoutSecond:  1,
		HttpProbeIntervalSecond: 1,
		HttpSuccessThreshold:    1,
	}, func(update *grpcapi.TaskUpdate) error { return nil })
	if err == nil {
		t.Fatalf("expected deploy failure")
	}
	execErr, ok := err.(*TaskExecutionError)
	if !ok || execErr.Diagnostic == nil {
		t.Fatalf("expected task execution error with diagnostic, got %#v", err)
	}
	if execErr.Diagnostic.FailureLogs != "boot failed" {
		t.Fatalf("expected failure logs to be captured, got %q", execErr.Diagnostic.FailureLogs)
	}
	if !execErr.Diagnostic.CleanupCompleted {
		t.Fatalf("expected cleanup to complete")
	}
	if len(docker.removedIDs) != 1 || docker.removedIDs[0] != "new-container" {
		t.Fatalf("expected failed container to be removed, got %#v", docker.removedIDs)
	}
}

func TestExecuteTrafficSwitchCleansOnlyCurrentAgentManagedContainers(t *testing.T) {
	docker := &fakeDockerRuntime{
		managedItems: []*ManagedContainer{
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "keep-target"},
				Name:             ManagedContainerNameForRelease("svc-a", "release-target"),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-target",
				Slot:             grpcapi.Slot_SLOT_GREEN,
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "keep-live"},
				Name:             ManagedContainerNameForRelease("svc-a", "release-live"),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-live",
				Slot:             grpcapi.Slot_SLOT_BLUE,
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "remove-old"},
				Name:             ManagedContainerNameForRelease("svc-a", "release-old"),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-old",
				Slot:             grpcapi.Slot_SLOT_GREEN,
			},
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)

	err := executor.Execute(context.Background(), &grpcapi.TaskCommand{
		TaskId:          "task-3",
		ReleaseId:       "release-target",
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

func TestExecuteTrafficSwitchWithoutReleaseIDFallsBackToSlotPreserve(t *testing.T) {
	docker := &fakeDockerRuntime{
		managedItems: []*ManagedContainer{
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "keep-target-slot"},
				Name:             ManagedContainerName("svc-a", grpcapi.Slot_SLOT_GREEN),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "",
				Slot:             grpcapi.Slot_SLOT_GREEN,
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "keep-live-slot"},
				Name:             ManagedContainerNameForRelease("svc-a", "release-live"),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-live",
				Slot:             grpcapi.Slot_SLOT_BLUE,
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "remove-stale"},
				Name:             ManagedContainerNameForRelease("svc-a", "release-stale"),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-stale",
				Slot:             grpcapi.Slot_SLOT_UNSPECIFIED,
			},
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)

	err := executor.Execute(context.Background(), &grpcapi.TaskCommand{
		TaskId:          "task-no-release-id",
		ReleaseId:       "",
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
	if len(docker.removedIDs) != 1 || docker.removedIDs[0] != "remove-stale" {
		t.Fatalf("expected only stale managed container to be removed, got %#v", docker.removedIDs)
	}
}

func TestReconcileManagedContainersOnStartupConservativeScan(t *testing.T) {
	docker := &fakeDockerRuntime{
		managedItems: []*ManagedContainer{
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "keep-running"},
				Name:             ManagedContainerName("svc-a", grpcapi.Slot_SLOT_GREEN),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-keep",
				Slot:             grpcapi.Slot_SLOT_GREEN,
				State:            "running",
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "remove-terminal"},
				Name:             ManagedContainerName("svc-a", grpcapi.Slot_SLOT_BLUE),
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-terminal",
				Slot:             grpcapi.Slot_SLOT_BLUE,
				State:            "exited",
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "remove-invalid-slot"},
				Name:             "ep-svc-a-invalid",
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "release-invalid",
				Slot:             grpcapi.Slot_SLOT_UNSPECIFIED,
				State:            "running",
			},
			{
				ContainerRuntime: ContainerRuntime{ContainerID: "remove-missing-release"},
				Name:             "ep-svc-a-missing-release",
				Managed:          true,
				AgentID:          "agent-a",
				ServiceKey:       "svc-a",
				ReleaseID:        "",
				Slot:             grpcapi.Slot_SLOT_GREEN,
				State:            "running",
			},
		},
		removeErrByID: map[string]error{
			"remove-terminal": errors.New("docker remove failed"),
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)

	stats, err := executor.ReconcileManagedContainersOnStartup(context.Background(), "agent-a")
	if err != nil {
		t.Fatalf("ReconcileManagedContainersOnStartup() error = %v", err)
	}
	if stats.Scanned != 4 || stats.Preserved != 1 || stats.Removed != 2 || stats.Failed != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if len(docker.removedIDs) != 3 {
		t.Fatalf("expected removal attempts to continue after failure, got %#v", docker.removedIDs)
	}
}

type fakeDockerRuntime struct {
	foundByName   map[string]*ManagedContainer
	managedItems  []*ManagedContainer
	statusByID    map[string]*ContainerStatus
	inspectCalls  map[string]int
	listenByID    map[string]string
	logsByID      map[string]string
	removeErrByID map[string]error
	deployedTasks []*grpcapi.TaskCommand
	removedIDs    []string
}

func (f *fakeDockerRuntime) DeployContainer(ctx context.Context, task *grpcapi.TaskCommand) (*ContainerRuntime, error) {
	f.deployedTasks = append(f.deployedTasks, task)
	return &ContainerRuntime{ContainerID: "new-container", ListenAddress: "172.29.0.22:8080"}, nil
}

func (f *fakeDockerRuntime) GetContainerDetails(ctx context.Context, containerID string) (*agentdomain.ContainerDetails, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeDockerRuntime) InspectContainer(ctx context.Context, containerID string) (*ContainerStatus, error) {
	if status, ok := f.statusByID[containerID]; ok {
		copyStatus := *status
		if strings.EqualFold(copyStatus.Health, "starting") {
			if f.inspectCalls == nil {
				f.inspectCalls = make(map[string]int)
			}
			f.inspectCalls[containerID]++
			if f.inspectCalls[containerID] > 1 {
				copyStatus.Health = "healthy"
				f.statusByID[containerID] = &copyStatus
			}
		}
		return &copyStatus, nil
	}
	return &ContainerStatus{State: "running", Running: true}, nil
}

func (f *fakeDockerRuntime) FindContainerByName(ctx context.Context, name string) (*ManagedContainer, error) {
	return f.foundByName[name], nil
}

func (f *fakeDockerRuntime) FindManagedContainerByIdentity(ctx context.Context, identity agentdomain.ManagedContainerIdentity) (*ManagedContainer, error) {
	name := agentdomain.ManagedContainerNameForTask(identity.ServiceKey, identity.ReleaseID, identity.Slot)
	return f.foundByName[name], nil
}

func (f *fakeDockerRuntime) ResolveListenAddress(ctx context.Context, containerID string, port int) (string, error) {
	if listen, ok := f.listenByID[containerID]; ok {
		return listen, nil
	}
	return "172.29.0.22:8080", nil
}

func (f *fakeDockerRuntime) ReadContainerLogs(ctx context.Context, containerID string, tailLines int, maxBytes int) (string, error) {
	if logs, ok := f.logsByID[containerID]; ok {
		return logs, nil
	}
	return "", nil
}

func (f *fakeDockerRuntime) StreamContainerLogs(ctx context.Context, containerID string, tailLines int, follow bool, timestamps bool, stderr bool) (io.ReadCloser, error) {
	return nil, nil
}

func (f *fakeDockerRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	f.removedIDs = append(f.removedIDs, containerID)
	if err, ok := f.removeErrByID[containerID]; ok {
		return err
	}
	return nil
}

func (f *fakeDockerRuntime) ListManagedContainers(ctx context.Context, agentID string, serviceKey string) ([]*ManagedContainer, error) {
	out := make([]*ManagedContainer, 0, len(f.managedItems))
	for _, item := range f.managedItems {
		if item == nil || !item.Managed || item.AgentID != agentID {
			continue
		}
		if serviceKey != "" && item.ServiceKey != serviceKey {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

type fakeProxyRuntime struct{}

func (f *fakeProxyRuntime) EnsureReady(context.Context) error { return nil }
func (f *fakeProxyRuntime) ApplySnapshot(context.Context, *grpcapi.ProxyConfigSnapshot) error {
	return nil
}
func (f *fakeProxyRuntime) SetServerAddress(context.Context, string, string, string, int) error {
	return nil
}
func (f *fakeProxyRuntime) EnableServer(context.Context, string, string) error { return nil }
func (f *fakeProxyRuntime) DisableServer(context.Context, string, string) error {
	return nil
}
func (f *fakeProxyRuntime) ShowStats(context.Context) ([]*grpcapi.BackendStatPoint, error) {
	return nil, nil
}
func (f *fakeProxyRuntime) ShowConfig(context.Context) (string, error) {
	return "", nil
}

func newDeployTaskCommand(releaseID string) *grpcapi.TaskCommand {
	return &grpcapi.TaskCommand{
		TaskId:                  "task-" + releaseID,
		ReleaseId:               releaseID,
		ServiceKey:              "svc-a",
		AgentId:                 "agent-a",
		Type:                    grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN,
		TargetSlot:              grpcapi.Slot_SLOT_GREEN,
		ServerName:              "srv-green",
		ContainerPort:           8080,
		DockerHealthCheck:       true,
		HttpHealthPath:          "/health",
		HttpExpectedCode:        200,
		HttpTimeoutSecond:       5,
		StartupGraceSecond:      1,
		HttpProbeTimeoutSecond:  1,
		HttpProbeIntervalSecond: 1,
		HttpSuccessThreshold:    1,
	}
}

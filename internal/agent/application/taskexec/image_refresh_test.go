package taskexec

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
)

func TestExecuteDeployProtectsRefreshedImageDuringSlotCleanup(t *testing.T) {
	for _, tc := range []struct {
		repo        string
		staleImage  string
		removeImage bool
	}{
		{repo: "ghcr.io/team/service", staleImage: "ghcr.io/team/service:latest"},
		{repo: "ghcr.io/team/service", staleImage: "ghcr.io/team/service:old", removeImage: true},
		{repo: "docker.io/library/nginx", staleImage: "nginx:latest"},
		{repo: "nginx", staleImage: "docker.io/nginx:latest"},
		{repo: "registry-1.docker.io/library/nginx", staleImage: "index.docker.io/nginx:latest"},
	} {
		t.Run(tc.staleImage, func(t *testing.T) {
			imageRef := tc.repo + ":latest"
			docker := &fakeDockerRuntime{
				managedItems: []*ManagedContainer{
					{ContainerRuntime: ContainerRuntime{ContainerID: "old-target"}, Managed: true, AgentID: "agent-a", ServiceKey: "svc-a", Slot: grpcapi.Slot_SLOT_GREEN, Image: tc.staleImage},
					{ContainerRuntime: ContainerRuntime{ContainerID: "live"}, Managed: true, AgentID: "agent-a", ServiceKey: "svc-a", Slot: grpcapi.Slot_SLOT_BLUE, Image: "ghcr.io/team/service:live"},
				},
				statusByID: map[string]*ContainerStatus{
					"new-container": {State: "running", Running: true, Health: "healthy"},
				},
			}
			executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)
			executor.httpProbe = func(string, string, map[string]string, int, int) error { return nil }
			task := newDeployTaskCommand("new-release")
			task.ImageRepo = tc.repo
			task.ImageTag = "latest"
			if err := executor.Execute(context.Background(), task, func(*grpcapi.TaskUpdate) error { return nil }); err != nil {
				t.Fatal(err)
			}
			want := []string{"pull:" + imageRef, "remove-container:old-target"}
			if tc.removeImage {
				want = append(want, "remove-image:"+tc.staleImage)
			}
			want = append(want, "deploy")
			if !reflect.DeepEqual(docker.events, want) {
				t.Fatalf("Docker operations = %v, want %v", docker.events, want)
			}
		})
	}
}

func TestExecuteDeployPullFailurePreservesPreviousContainers(t *testing.T) {
	pullErr := errors.New("registry unavailable")
	docker := &fakeDockerRuntime{
		ensureImageErr: pullErr,
		managedItems: []*ManagedContainer{
			{ContainerRuntime: ContainerRuntime{ContainerID: "old-target"}, Managed: true, AgentID: "agent-a", ServiceKey: "svc-a", Slot: grpcapi.Slot_SLOT_GREEN, Image: "ghcr.io/team/service:latest"},
		},
	}
	executor := NewExecutor(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker, &fakeProxyRuntime{}, nil)
	task := newDeployTaskCommand("new-release")
	task.ImageRepo = "ghcr.io/team/service"
	task.ImageTag = "latest"
	err := executor.Execute(context.Background(), task, func(*grpcapi.TaskUpdate) error { return nil })
	var execErr *TaskExecutionError
	if !errors.As(err, &execErr) || execErr.Step != "image_pull_failed" || !errors.Is(err, pullErr) {
		t.Fatalf("Execute() error = %v, want image_pull_failed", err)
	}
	if len(docker.removedIDs) != 0 || len(docker.removedImages) != 0 || len(docker.deployedTasks) != 0 || len(docker.pulledImages) != 1 {
		t.Fatalf("pull failure mutated containers: removed=%v images=%v deployed=%d pulls=%d", docker.removedIDs, docker.removedImages, len(docker.deployedTasks), len(docker.pulledImages))
	}
}

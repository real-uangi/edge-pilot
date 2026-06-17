package containerindex

import (
	"context"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"io"
	"testing"
)

func TestFindByIdentityReturnsConflictWhenDuplicate(t *testing.T) {
	docker := &fakeDockerRuntime{
		managedItems: []*agentdomain.ManagedContainer{
			{ServiceKey: "svc-a", ReleaseID: "release-1", Slot: grpcapi.Slot_SLOT_GREEN},
			{ServiceKey: "svc-a", ReleaseID: "release-1", Slot: grpcapi.Slot_SLOT_GREEN},
		},
	}
	index := NewManagedContainerIndex(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker)
	if err := index.RefreshNow(context.Background()); err != nil {
		t.Fatalf("RefreshNow() error = %v", err)
	}
	_, err := index.FindByIdentity(agentdomain.ManagedContainerIdentity{ServiceKey: "svc-a", ReleaseID: "release-1", Slot: grpcapi.Slot_SLOT_GREEN})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestFindByIdentityMatchesSlotFallback(t *testing.T) {
	docker := &fakeDockerRuntime{
		managedItems: []*agentdomain.ManagedContainer{
			{ServiceKey: "svc-a", ReleaseID: "", Slot: grpcapi.Slot_SLOT_BLUE},
		},
	}
	index := NewManagedContainerIndex(&config.AgentRuntimeConfig{AgentID: "agent-a"}, docker)
	if err := index.RefreshNow(context.Background()); err != nil {
		t.Fatalf("RefreshNow() error = %v", err)
	}
	item, err := index.FindByIdentity(agentdomain.ManagedContainerIdentity{ServiceKey: "svc-a", Slot: grpcapi.Slot_SLOT_BLUE})
	if err != nil {
		t.Fatalf("FindByIdentity() error = %v", err)
	}
	if item == nil {
		t.Fatal("expected slot fallback match")
	}
}

type fakeDockerRuntime struct {
	managedItems []*agentdomain.ManagedContainer
}

func (f *fakeDockerRuntime) EnsureImage(context.Context, string, *grpcapi.TaskCommand) error {
	panic("not implemented")
}

func (f *fakeDockerRuntime) DeployContainer(context.Context, *grpcapi.TaskCommand) (*agentdomain.ContainerRuntime, error) {
	panic("not implemented")
}

func (f *fakeDockerRuntime) GetContainerDetails(context.Context, string) (*agentdomain.ContainerDetails, error) {
	panic("not implemented")
}

func (f *fakeDockerRuntime) InspectContainer(context.Context, string) (*agentdomain.ContainerStatus, error) {
	panic("not implemented")
}

func (f *fakeDockerRuntime) FindContainerByName(context.Context, string) (*agentdomain.ManagedContainer, error) {
	panic("not implemented")
}

func (f *fakeDockerRuntime) FindManagedContainerByIdentity(context.Context, agentdomain.ManagedContainerIdentity) (*agentdomain.ManagedContainer, error) {
	panic("not implemented")
}

func (f *fakeDockerRuntime) ResolveListenAddress(context.Context, string, int) (string, error) {
	panic("not implemented")
}

func (f *fakeDockerRuntime) ReadContainerLogs(context.Context, string, int, int) (string, error) {
	panic("not implemented")
}

func (f *fakeDockerRuntime) StreamContainerLogs(context.Context, string, int, bool, bool, bool) (io.ReadCloser, error) {
	panic("not implemented")
}

func (f *fakeDockerRuntime) RemoveContainer(context.Context, string) error {
	panic("not implemented")
}

func (f *fakeDockerRuntime) RemoveImage(context.Context, string) error {
	panic("not implemented")
}

func (f *fakeDockerRuntime) ListManagedContainers(context.Context, string, string) ([]*agentdomain.ManagedContainer, error) {
	return f.managedItems, nil
}

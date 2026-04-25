package agent

import (
	"context"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"testing"
)

func TestSchedulerInstanceExecutorID(t *testing.T) {
	got := schedulerInstanceExecutorID(
		"service-1",
		"release-1",
		grpcapi.Slot_SLOT_GREEN,
		"1234567890abcdef",
	)
	want := "svc:service-1:rel:release-1:slot:green:ctr:1234567890ab"
	if got != want {
		t.Fatalf("schedulerInstanceExecutorID() = %q, want %q", got, want)
	}
}

func TestSchedulerInstanceMessageToExecutorMessage(t *testing.T) {
	msg := schedulerInstanceMessageToExecutorMessage("exec-a", &grpcapi.SchedulerInstanceMessage{
		Payload: &grpcapi.SchedulerInstanceMessage_Heartbeat{
			Heartbeat: &grpcapi.ExecutorHeartbeat{RunningRunIds: []string{"run-1"}},
		},
	})
	if msg == nil || msg.GetHeartbeat() == nil {
		t.Fatalf("expected heartbeat executor message")
	}
	if msg.GetHeartbeat().GetExecutorId() != "exec-a" {
		t.Fatalf("expected executor id to be filled, got %q", msg.GetHeartbeat().GetExecutorId())
	}

	update := schedulerInstanceMessageToExecutorMessage("exec-a", &grpcapi.SchedulerInstanceMessage{
		Payload: &grpcapi.SchedulerInstanceMessage_RunUpdate{
			RunUpdate: &grpcapi.SchedulerRunUpdate{RunId: "run-1", Success: true},
		},
	})
	if update == nil || update.GetRunUpdate().GetRunId() != "run-1" {
		t.Fatalf("unexpected run update conversion: %#v", update)
	}
}

func TestIsSchedulerReleaseContainer(t *testing.T) {
	service := &grpcapi.ProxyServiceConfig{
		LiveReleaseId:      "release-live",
		CandidateReleaseId: "release-candidate",
	}
	if !isSchedulerReleaseContainer("release-live", service) {
		t.Fatalf("expected live release to match")
	}
	if !isSchedulerReleaseContainer("release-candidate", service) {
		t.Fatalf("expected candidate release to match")
	}
	if isSchedulerReleaseContainer("release-old", service) {
		t.Fatalf("expected stale release not to match")
	}
}

func TestApplyWantedReconnectsWhenTargetChanges(t *testing.T) {
	connector := newSchedulerInstanceConnector(&config.AgentRuntimeConfig{AgentID: "agent-a"}, nil, nil)
	ctx := context.Background()
	first := schedulerInstanceTarget{
		executorID:  "exec-1",
		serviceID:   "svc-1",
		serviceKey:  "svc-key",
		releaseID:   "rel-1",
		containerID: "container-1",
		slot:        grpcapi.Slot_SLOT_BLUE,
		group:       "g1",
		port:        18080,
	}
	connector.applyWanted(ctx, map[string]schedulerInstanceTarget{"exec-1": first})

	changed := first
	changed.group = "g2"
	connector.applyWanted(ctx, map[string]schedulerInstanceTarget{"exec-1": changed})
	replaced := connector.sessions["exec-1"]
	if !replaced.target.equal(changed) {
		t.Fatalf("expected stored target to be updated")
	}
	connector.forgetSession("exec-1", first)
	if _, ok := connector.sessions["exec-1"]; !ok {
		t.Fatalf("expected stale forgetSession call not to remove replaced session")
	}
}

func TestApplyWantedKeepsSessionWhenTargetUnchanged(t *testing.T) {
	connector := newSchedulerInstanceConnector(&config.AgentRuntimeConfig{AgentID: "agent-a"}, nil, nil)
	ctx := context.Background()
	target := schedulerInstanceTarget{
		executorID:  "exec-1",
		serviceID:   "svc-1",
		serviceKey:  "svc-key",
		releaseID:   "rel-1",
		containerID: "container-1",
		slot:        grpcapi.Slot_SLOT_BLUE,
		group:       "g1",
		port:        18080,
	}
	connector.applyWanted(ctx, map[string]schedulerInstanceTarget{"exec-1": target})

	connector.applyWanted(ctx, map[string]schedulerInstanceTarget{"exec-1": target})
	if len(connector.sessions) != 1 {
		t.Fatalf("expected unchanged session set size, got %d", len(connector.sessions))
	}
	stored := connector.sessions["exec-1"]
	if !stored.target.equal(target) {
		t.Fatalf("expected session target to stay unchanged")
	}
}

func TestForgetSessionIgnoresReplacedSession(t *testing.T) {
	connector := newSchedulerInstanceConnector(&config.AgentRuntimeConfig{AgentID: "agent-a"}, nil, nil)
	targetA := schedulerInstanceTarget{executorID: "exec-1", group: "g1", port: 18080}
	targetB := schedulerInstanceTarget{executorID: "exec-1", group: "g2", port: 18080}
	connector.sessions["exec-1"] = schedulerInstanceSession{target: targetA, cancel: func() {}}
	connector.sessions["exec-1"] = schedulerInstanceSession{target: targetB, cancel: func() {}}

	connector.forgetSession("exec-1", targetA)
	if _, ok := connector.sessions["exec-1"]; !ok {
		t.Fatalf("expected newer session to be retained")
	}
	connector.forgetSession("exec-1", targetB)
	if _, ok := connector.sessions["exec-1"]; ok {
		t.Fatalf("expected matching session to be removed")
	}
}

type fakeDockerRuntime struct{}

func (f *fakeDockerRuntime) DeployContainer(context.Context, *grpcapi.TaskCommand) (*agentdomain.ContainerRuntime, error) {
	panic("not implemented")
}
func (f *fakeDockerRuntime) InspectContainer(context.Context, string) (*agentdomain.ContainerStatus, error) {
	panic("not implemented")
}
func (f *fakeDockerRuntime) FindContainerByName(context.Context, string) (*agentdomain.ManagedContainer, error) {
	panic("not implemented")
}
func (f *fakeDockerRuntime) ResolveListenAddress(context.Context, string, int) (string, error) {
	panic("not implemented")
}
func (f *fakeDockerRuntime) ReadContainerLogs(context.Context, string, int, int) (string, error) {
	panic("not implemented")
}
func (f *fakeDockerRuntime) RemoveContainer(context.Context, string) error {
	panic("not implemented")
}
func (f *fakeDockerRuntime) ListManagedContainers(context.Context, string, string) ([]*agentdomain.ManagedContainer, error) {
	panic("not implemented")
}

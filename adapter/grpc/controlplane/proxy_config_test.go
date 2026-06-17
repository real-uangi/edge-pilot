package controlplane

import (
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"testing"
	"time"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
)

func TestBuildProxyConfigSnapshotCarriesSortedRoutesAndLiveSlot(t *testing.T) {
	enabled := true
	services := []model.Service{
		{
			ID:              uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			ServiceKey:      "svc-root",
			AgentID:         "agent-a",
			RouteHost:       "api.example.com",
			RoutePathPrefix: "/",
			CurrentLiveSlot: model.SlotBlue,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
		{
			ID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ServiceKey:      "svc-api",
			AgentID:         "agent-a",
			RouteHost:       "api.example.com",
			RoutePathPrefix: "/v1/internal",
			CurrentLiveSlot: model.SlotGreen,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
	}

	snapshot := buildProxyConfigSnapshot("agent-a", services)
	if snapshot.GetFrontendName() == "" || snapshot.GetDefaultBackend() == "" {
		t.Fatalf("expected managed frontend metadata")
	}
	if len(snapshot.GetServices()) != 2 {
		t.Fatalf("expected two services, got %d", len(snapshot.GetServices()))
	}
	if snapshot.GetServices()[0].GetServiceKey() != "svc-api" {
		t.Fatalf("expected longest path first, got %#v", snapshot.GetServices())
	}
	if snapshot.GetServices()[0].GetCurrentLiveSlot() != toProtoSlot(model.SlotGreen) {
		t.Fatalf("expected current live slot to be preserved")
	}
}

func TestBuildProxyConfigSnapshotCarriesRouteHosts(t *testing.T) {
	enabled := true
	services := []model.Service{
		{
			ID:              uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			ServiceKey:      "svc-api",
			AgentID:         "agent-a",
			RouteHost:       "api.example.com",
			RouteHosts:      commondb.NewJSONB([]string{"api.example.com", "api-alt.example.com"}),
			RoutePathPrefix: "/api",
			CurrentLiveSlot: model.SlotBlue,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
	}

	snapshot := buildProxyConfigSnapshot("agent-a", services)
	if len(snapshot.GetServices()) != 1 {
		t.Fatalf("expected one service, got %d", len(snapshot.GetServices()))
	}
	got := snapshot.GetServices()[0].GetRouteHosts()
	want := []string{"api.example.com", "api-alt.example.com"}
	if len(got) != len(want) {
		t.Fatalf("expected route hosts %#v, got %#v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected route hosts %#v, got %#v", want, got)
		}
	}
}

func TestCandidateTrafficPercentIgnoresCompletedAndSwitchedReleases(t *testing.T) {
	for _, status := range []model.ReleaseStatus{
		model.ReleaseStatusCompleted,
		model.ReleaseStatusFailed,
	} {
		release := &model.Release{
			Status:         status,
			TrafficPercent: 100,
		}
		if got := candidateTrafficPercentForRelease(release); got != 0 {
			t.Fatalf("expected status %v to contribute no candidate traffic, got %d", status, got)
		}
	}
}

func TestCandidateTrafficPercentAllowsReadyToSwitchRelease(t *testing.T) {
	release := &model.Release{
		Status:         model.ReleaseStatusReadyToSwitch,
		TrafficPercent: 30,
	}
	if got := candidateTrafficPercentForRelease(release); got != 30 {
		t.Fatalf("expected ready-to-switch traffic percent 30, got %d", got)
	}
}

func TestCandidateTrafficPercentAllowsSwitchedPartialRelease(t *testing.T) {
	release := &model.Release{
		Status:         model.ReleaseStatusSwitched,
		TrafficPercent: 30,
	}
	if got := candidateTrafficPercentForRelease(release); got != 30 {
		t.Fatalf("expected switched partial traffic percent 30, got %d", got)
	}
}

func TestBuildProxyConfigSnapshotFiltersNonBetaCandidateRelease(t *testing.T) {
	enabled := true
	serviceID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	liveReleaseID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	completedCandidateID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	services := []model.Service{
		{
			ID:              serviceID,
			ServiceKey:      "svc-api",
			AgentID:         "agent-a",
			RouteHost:       "api.example.com",
			RoutePathPrefix: "/",
			CurrentLiveSlot: model.SlotBlue,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
	}
	publisher := &ProxyConfigPublisher{releases: &fakeProxyReleaseRepo{
		runtimeInstances: []model.RuntimeInstance{
			{ServiceID: serviceID, Slot: model.SlotBlue, ReleaseID: liveReleaseID},
			{ServiceID: serviceID, Slot: model.SlotGreen, ReleaseID: completedCandidateID},
		},
		releases: map[uuid.UUID]*model.Release{
			completedCandidateID: {ID: completedCandidateID, ServiceID: serviceID, Status: model.ReleaseStatusCompleted, TrafficPercent: 0},
		},
	}}

	snapshot, err := publisher.buildProxyConfigSnapshot("agent-a", services)
	if err != nil {
		t.Fatalf("buildProxyConfigSnapshot() error = %v", err)
	}
	if got := snapshot.GetServices()[0].GetCandidateReleaseId(); got != "" {
		t.Fatalf("expected completed opposite slot release to be hidden from beta, got %q", got)
	}
	if got := snapshot.GetServices()[0].GetCandidateTrafficPercent(); got != 0 {
		t.Fatalf("expected completed opposite slot release traffic percent 0, got %d", got)
	}
}

func TestBuildProxyConfigSnapshotKeepsReadyToSwitchCandidateRelease(t *testing.T) {
	snapshot := buildSnapshotWithCandidateStatus(t, model.ReleaseStatusReadyToSwitch, 0)
	if got := snapshot.GetServices()[0].GetCandidateReleaseId(); got != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("expected ready-to-switch release to be beta candidate, got %q", got)
	}
}

func TestBuildProxyConfigSnapshotKeepsSwitchedPartialCandidateRelease(t *testing.T) {
	snapshot := buildSnapshotWithCandidateStatus(t, model.ReleaseStatusSwitched, 30)
	if got := snapshot.GetServices()[0].GetCandidateReleaseId(); got != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("expected switched partial release to be beta candidate, got %q", got)
	}
	if got := snapshot.GetServices()[0].GetCandidateTrafficPercent(); got != 30 {
		t.Fatalf("expected switched partial traffic percent 30, got %d", got)
	}
}

func TestBuildProxyConfigSnapshotFiltersSwitchedFullCandidateRelease(t *testing.T) {
	snapshot := buildSnapshotWithCandidateStatus(t, model.ReleaseStatusSwitched, 100)
	if got := snapshot.GetServices()[0].GetCandidateReleaseId(); got != "" {
		t.Fatalf("expected switched full release to be hidden from beta, got %q", got)
	}
}

func buildSnapshotWithCandidateStatus(t *testing.T, status model.ReleaseStatus, percent int) *grpcapi.ProxyConfigSnapshot {
	t.Helper()
	enabled := true
	serviceID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	liveReleaseID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	candidateReleaseID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	services := []model.Service{
		{
			ID:              serviceID,
			ServiceKey:      "svc-api",
			AgentID:         "agent-a",
			RouteHost:       "api.example.com",
			RoutePathPrefix: "/",
			CurrentLiveSlot: model.SlotBlue,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
	}
	publisher := &ProxyConfigPublisher{releases: &fakeProxyReleaseRepo{
		runtimeInstances: []model.RuntimeInstance{
			{ServiceID: serviceID, Slot: model.SlotBlue, ReleaseID: liveReleaseID},
			{ServiceID: serviceID, Slot: model.SlotGreen, ReleaseID: candidateReleaseID},
		},
		releases: map[uuid.UUID]*model.Release{
			candidateReleaseID: {ID: candidateReleaseID, ServiceID: serviceID, Status: status, TrafficPercent: percent},
		},
	}}
	snapshot, err := publisher.buildProxyConfigSnapshot("agent-a", services)
	if err != nil {
		t.Fatalf("buildProxyConfigSnapshot() error = %v", err)
	}
	return snapshot
}

type fakeProxyReleaseRepo struct {
	runtimeInstances []model.RuntimeInstance
	releases         map[uuid.UUID]*model.Release
}

func (r *fakeProxyReleaseRepo) CreateRelease(*model.Release) error { return nil }

func (r *fakeProxyReleaseRepo) UpdateRelease(*model.Release) error { return nil }

func (r *fakeProxyReleaseRepo) GetRelease(id uuid.UUID) (*model.Release, error) {
	return r.releases[id], nil
}

func (r *fakeProxyReleaseRepo) ListReleases(int) ([]model.Release, error) { return nil, nil }

func (r *fakeProxyReleaseRepo) ListQueuedBefore(uuid.UUID, time.Time, uuid.UUID) ([]model.Release, error) {
	return nil, nil
}

func (r *fakeProxyReleaseRepo) FindReadyToSwitchRelease(uuid.UUID) (*model.Release, error) {
	return nil, nil
}

func (r *fakeProxyReleaseRepo) HasActiveRelease(uuid.UUID) (bool, error) { return false, nil }

func (r *fakeProxyReleaseRepo) HasTrafficSplitRelease(uuid.UUID) (bool, error) { return false, nil }

func (r *fakeProxyReleaseRepo) HasNewerSuccessfulRelease(uuid.UUID, time.Time) (bool, error) {
	return false, nil
}

func (r *fakeProxyReleaseRepo) FindQueuedOrActiveDuplicate(uuid.UUID, string, string) (*model.Release, error) {
	return nil, nil
}

func (r *fakeProxyReleaseRepo) CountQueuedBefore(uuid.UUID, time.Time, uuid.UUID) (int, error) {
	return 0, nil
}

func (r *fakeProxyReleaseRepo) CreateTask(*model.Task) error { return nil }

func (r *fakeProxyReleaseRepo) UpdateTask(*model.Task) error { return nil }

func (r *fakeProxyReleaseRepo) GetTask(uuid.UUID) (*model.Task, error) { return nil, nil }

func (r *fakeProxyReleaseRepo) ListTasksByRelease(uuid.UUID) ([]model.Task, error) { return nil, nil }

func (r *fakeProxyReleaseRepo) ListRecoverableTasksByAgent(string) ([]model.Task, error) {
	return nil, nil
}

func (r *fakeProxyReleaseRepo) ListActiveTasks() ([]model.Task, error) { return nil, nil }

func (r *fakeProxyReleaseRepo) CreateTaskAttempt(*model.TaskAttempt) error { return nil }

func (r *fakeProxyReleaseRepo) UpsertRuntimeInstance(*model.RuntimeInstance) error { return nil }

func (r *fakeProxyReleaseRepo) GetRuntimeInstanceByServiceAndSlot(uuid.UUID, model.Slot) (*model.RuntimeInstance, error) {
	return nil, nil
}

func (r *fakeProxyReleaseRepo) ListRuntimeInstancesByService(uuid.UUID) ([]model.RuntimeInstance, error) {
	return append([]model.RuntimeInstance(nil), r.runtimeInstances...), nil
}

func (r *fakeProxyReleaseRepo) CreateAudit(*model.AuditLog) error { return nil }

func (r *fakeProxyReleaseRepo) ListTaskAttemptsByTask(uuid.UUID) ([]model.TaskAttempt, error) {
	return nil, nil
}

func (r *fakeProxyReleaseRepo) ListAuditsByAggregate(string, string) ([]model.AuditLog, error) {
	return nil, nil
}

package controlplane

import (
	schedulerapp "edge-pilot/internal/scheduler/application"
	"edge-pilot/internal/scheduler/domain"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSchedulerSessionHubListOnlineExecutorsIncludesServiceID(t *testing.T) {
	hub := NewSchedulerSessionHub()
	hub.registerRelay(
		"svc:exec",
		"default",
		model.SlotBlue,
		"agent-a",
		"relay-1",
		"svc:exec",
		true,
		uuid.New().String(),
		nil,
	)

	online := hub.ListOnlineExecutors("default")
	if len(online) != 1 {
		t.Fatalf("expected one online executor, got %d", len(online))
	}
	if online[0].ServiceID == "" {
		t.Fatalf("expected serviceID to be propagated for service instance executor")
	}
}

func TestHandleRelayExecutorMessageRejectsServiceInstanceHelloWithoutRelaySessionID(t *testing.T) {
	srv := &SchedulerServer{hub: NewSchedulerSessionHub(), service: schedulerapp.NewService(newSchedulerAuthRepo(), nil, nil, nil)}
	err := srv.handleRelayExecutorMessage("agent-a", &grpcapi.SchedulerRelayEnvelope{}, &grpcapi.ExecutorMessage{
		Payload: &grpcapi.ExecutorMessage_Hello{Hello: &grpcapi.ExecutorHello{
			ExecutorId: "svc:x:rel:r1:slot:blue:ctr:abcdef012345",
			Group:      "default",
			LiveSlot:   grpcapi.Slot_SLOT_BLUE,
			Metadata: map[string]string{
				"edge_pilot_service_instance": "true",
				"service_id":                  uuid.New().String(),
				"release_id":                  "r1",
				"container_id":                "abcdef0123456789",
			},
		}},
	})
	if err == nil {
		t.Fatalf("expected invalid argument error for missing relay_session_id")
	}
}

func TestHandleRelayExecutorMessageRegistersServiceInstanceWithServiceID(t *testing.T) {
	repo := newSchedulerAuthRepo()
	svc := schedulerapp.NewService(repo, nil, nil, nil)
	srv := &SchedulerServer{hub: NewSchedulerSessionHub(), service: svc}
	serviceID := uuid.New().String()
	releaseID := "release-a"
	containerID := "abcdef0123456789"
	executorID := "svc:" + serviceID + ":rel:" + releaseID + ":slot:blue:ctr:abcdef012345"

	err := srv.handleRelayExecutorMessage("agent-a", &grpcapi.SchedulerRelayEnvelope{RelaySessionId: "relay-1", RoutingKey: executorID}, &grpcapi.ExecutorMessage{
		Payload: &grpcapi.ExecutorMessage_Hello{Hello: &grpcapi.ExecutorHello{
			ExecutorId: executorID,
			Group:      "default",
			LiveSlot:   grpcapi.Slot_SLOT_BLUE,
			Metadata: map[string]string{
				"edge_pilot_service_instance": "true",
				"service_id":                  serviceID,
				"release_id":                  releaseID,
				"container_id":                containerID,
			},
		}},
	})
	if err != nil {
		t.Fatalf("handleRelayExecutorMessage() error = %v", err)
	}

	online := srv.hub.ListOnlineExecutors("default")
	if len(online) != 1 {
		t.Fatalf("expected one relay session, got %d", len(online))
	}
	if online[0].ServiceID != serviceID {
		t.Fatalf("expected propagated serviceID %q, got %q", serviceID, online[0].ServiceID)
	}
}

type schedulerAuthRepo struct {
	executors map[string]*model.SchedulerExecutor
}

func newSchedulerAuthRepo() *schedulerAuthRepo {
	return &schedulerAuthRepo{executors: map[string]*model.SchedulerExecutor{}}
}

func (r *schedulerAuthRepo) CreateJob(job *model.SchedulerJob) error { panic("not implemented") }
func (r *schedulerAuthRepo) UpdateJob(job *model.SchedulerJob) error { panic("not implemented") }
func (r *schedulerAuthRepo) GetJob(id uuid.UUID) (*model.SchedulerJob, error) {
	panic("not implemented")
}
func (r *schedulerAuthRepo) ListJobs() ([]model.SchedulerJob, error) { panic("not implemented") }
func (r *schedulerAuthRepo) ListJobsDue(now time.Time, limit int) ([]model.SchedulerJob, error) {
	panic("not implemented")
}
func (r *schedulerAuthRepo) DeleteJob(id uuid.UUID) error               { panic("not implemented") }
func (r *schedulerAuthRepo) CreateRun(run *model.SchedulerJobRun) error { panic("not implemented") }
func (r *schedulerAuthRepo) UpdateRun(run *model.SchedulerJobRun) error { panic("not implemented") }
func (r *schedulerAuthRepo) GetRun(id uuid.UUID) (*model.SchedulerJobRun, error) {
	panic("not implemented")
}
func (r *schedulerAuthRepo) ListRunsByJob(jobID uuid.UUID, limit int) ([]model.SchedulerJobRun, error) {
	panic("not implemented")
}
func (r *schedulerAuthRepo) ListDispatchableRuns(now time.Time, limit int) ([]model.SchedulerJobRun, error) {
	panic("not implemented")
}
func (r *schedulerAuthRepo) ClaimRun(runID uuid.UUID, leasedBy string, leaseExpiresAt time.Time, now time.Time) (bool, error) {
	panic("not implemented")
}
func (r *schedulerAuthRepo) GetDispatchCursor(jobID uuid.UUID, executorGroup string) (*model.SchedulerDispatchCursor, error) {
	panic("not implemented")
}
func (r *schedulerAuthRepo) SaveDispatchCursor(cursor *model.SchedulerDispatchCursor) error {
	panic("not implemented")
}
func (r *schedulerAuthRepo) UpsertExecutor(executor *model.SchedulerExecutor) error {
	if executor == nil {
		return nil
	}
	copy := *executor
	r.executors[executor.ID] = &copy
	return nil
}
func (r *schedulerAuthRepo) GetExecutor(id string) (*model.SchedulerExecutor, error) {
	exec := r.executors[id]
	if exec == nil {
		return nil, nil
	}
	copy := *exec
	return &copy, nil
}
func (r *schedulerAuthRepo) ListExecutorsByGroup(group string) ([]model.SchedulerExecutor, error) {
	out := make([]model.SchedulerExecutor, 0)
	for _, executor := range r.executors {
		if executor.Group != group {
			continue
		}
		copy := *executor
		out = append(out, copy)
	}
	return out, nil
}
func (r *schedulerAuthRepo) ListExecutors() ([]model.SchedulerExecutor, error) {
	out := make([]model.SchedulerExecutor, 0, len(r.executors))
	for _, executor := range r.executors {
		copy := *executor
		out = append(out, copy)
	}
	return out, nil
}
func (r *schedulerAuthRepo) DeleteExecutor(id string) error { panic("not implemented") }
func (r *schedulerAuthRepo) MarkExecutorSeen(id string, at time.Time) error {
	exec := r.executors[id]
	if exec == nil {
		return nil
	}
	exec.LastSeenAt = &at
	return nil
}
func (r *schedulerAuthRepo) WithTx(fn func(tx domain.Repository) error) error {
	panic("not implemented")
}

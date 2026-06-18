package application

import (
	"errors"
	"testing"
	"time"

	"github.com/real-uangi/edge-pilot/internal/scheduler/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
)

func TestCalcInitialNextRun_OneTimeUsesRunAt(t *testing.T) {
	runAt := time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC)
	next, err := calcInitialNextRun(1, "", &runAt, time.Now().UTC())
	if err != nil {
		t.Fatalf("calcInitialNextRun() error = %v", err)
	}
	if next == nil || !next.Equal(runAt) {
		t.Fatalf("calcInitialNextRun() = %v, want %v", next, runAt)
	}
}

func TestParseDispatchPolicy(t *testing.T) {
	if _, err := parseDispatchPolicy("round_robin"); err != nil {
		t.Fatalf("parseDispatchPolicy(round_robin) error = %v", err)
	}
	if _, err := parseDispatchPolicy("fixed_live_slot"); err != nil {
		t.Fatalf("parseDispatchPolicy(fixed_live_slot) error = %v", err)
	}
	if _, err := parseDispatchPolicy("invalid"); err == nil {
		t.Fatalf("expected invalid policy error")
	}
}

func TestRetryBackoff(t *testing.T) {
	if got := retryBackoff(1); got != 5*time.Second {
		t.Fatalf("retryBackoff(1) = %v, want 5s", got)
	}
	if got := retryBackoff(3); got != 20*time.Second {
		t.Fatalf("retryBackoff(3) = %v, want 20s", got)
	}
}

func TestDTOShapeCompileGuard(t *testing.T) {
	_ = dto.UpsertSchedulerJobRequest{}
}

func TestEffectiveLeaseTimeoutPrefersRunField(t *testing.T) {
	run := &model.SchedulerJobRun{
		LeaseTimeoutSec: 90,
	}
	if got := effectiveLeaseTimeout(run, 60); got != 90 {
		t.Fatalf("effectiveLeaseTimeout() = %d, want 90", got)
	}
}

func TestSanitizeExecutorMetadata(t *testing.T) {
	clean := sanitizeExecutorMetadata(map[string]string{
		"relay_token": "secret",
		"ReLaY_ToKeN": "secret2",
		"instanceId":  "i-1",
	})
	if _, ok := clean["relay_token"]; ok {
		t.Fatalf("relay_token should be removed")
	}
	if _, ok := clean["ReLaY_ToKeN"]; ok {
		t.Fatalf("relay_token variant should be removed")
	}
	if got := clean["instanceId"]; got != "i-1" {
		t.Fatalf("instanceId should be kept, got %q", got)
	}
}

func TestPickExecutorIDFixedLiveSlotPrefersMatchingServiceInstance(t *testing.T) {
	serviceID := uuid.New()
	otherServiceID := uuid.New()
	svc := &Service{liveSlot: staticLiveSlotResolver{slot: model.SlotGreen}}
	run := &model.SchedulerJobRun{
		DispatchPolicy: model.SchedulerDispatchPolicyFixedLiveSlot,
		Payload:        commondb.NewJSONB(map[string]any{"serviceId": serviceID.String()}),
	}

	got, err := svc.pickExecutorID(newAuthOnlyRepo(), run, []OnlineExecutor{
		{ExecutorID: "instance-other-green", Group: "default", LiveSlot: model.SlotGreen, ServiceInstance: true, ServiceID: otherServiceID.String()},
		{ExecutorID: "instance-target-green", Group: "default", LiveSlot: model.SlotGreen, ServiceInstance: true, ServiceID: serviceID.String()},
		{ExecutorID: "manual-green", Group: "default", LiveSlot: model.SlotGreen},
	})
	if err != nil {
		t.Fatalf("pickExecutorID() error = %v", err)
	}
	if got != "instance-target-green" {
		t.Fatalf("expected matching service instance on current live slot, got %q", got)
	}
}

func TestPickExecutorIDFixedLiveSlotFallsBackWhenServiceInstanceMismatched(t *testing.T) {
	serviceID := uuid.New()
	svc := &Service{liveSlot: staticLiveSlotResolver{slot: model.SlotBlue}}
	run := &model.SchedulerJobRun{
		DispatchPolicy: model.SchedulerDispatchPolicyFixedLiveSlot,
		Payload:        commondb.NewJSONB(map[string]any{"serviceId": serviceID.String()}),
	}

	got, err := svc.pickExecutorID(newAuthOnlyRepo(), run, []OnlineExecutor{
		{ExecutorID: "instance-green", Group: "default", LiveSlot: model.SlotGreen, ServiceInstance: true, ServiceID: serviceID.String()},
		{ExecutorID: "instance-blue-other", Group: "default", LiveSlot: model.SlotBlue, ServiceInstance: true, ServiceID: uuid.New().String()},
		{ExecutorID: "manual-blue", Group: "default", LiveSlot: model.SlotBlue},
	})
	if err != nil {
		t.Fatalf("pickExecutorID() error = %v", err)
	}
	if got != "manual-blue" {
		t.Fatalf("expected fallback to non-instance executor on target slot, got %q", got)
	}
}

func TestRegisterServiceInstanceExecutor(t *testing.T) {
	repo := newAuthOnlyRepo()
	svc := NewService(repo, nil, nil, nil)

	serviceID := uuid.New().String()
	releaseID := "release-a"
	executorID := "svc:" + serviceID + ":rel:release-a:slot:blue:inst:uuid-test-123"
	executor, err := svc.RegisterServiceInstanceExecutor(
		executorID,
		"default",
		model.SlotBlue,
		map[string]string{"service_id": serviceID, "release_id": releaseID, "executor_id": executorID, "relay_token": "secret"},
		"agent-a",
		executorID,
	)
	if err != nil {
		t.Fatalf("RegisterServiceInstanceExecutor() error = %v", err)
	}
	if executor.ChannelMode != model.SchedulerExecutorChannelModeAgentRelay || executor.RelayAgentID != "agent-a" {
		t.Fatalf("unexpected relay binding: %#v", executor)
	}
	if executor.LiveSlot != model.SlotBlue || executor.Enabled == nil || !*executor.Enabled {
		t.Fatalf("unexpected executor state: %#v", executor)
	}
	if _, ok := executor.InstanceMeta.Get()["relay_token"]; ok {
		t.Fatalf("expected relay token metadata to be sanitized")
	}
}

func TestRegisterServiceInstanceExecutorRejectsMismatchedExecutorID(t *testing.T) {
	repo := newAuthOnlyRepo()
	svc := NewService(repo, nil, nil, nil)

	_, err := svc.RegisterServiceInstanceExecutor(
		"wrong-executor-id",
		"default",
		model.SlotBlue,
		map[string]string{"service_id": uuid.New().String(), "release_id": "r1", "executor_id": "expected-executor-id"},
		"agent-a",
		"wrong-executor-id",
	)
	if err == nil {
		t.Fatalf("expected executorId mismatch error")
	}
}

func TestRegisterServiceInstanceExecutorRejectsRebindToDifferentAgent(t *testing.T) {
	repo := newAuthOnlyRepo()
	existingEnabled := true
	serviceID := uuid.New().String()
	releaseID := "release-a"
	executorID := "svc:" + serviceID + ":rel:release-a:slot:green:inst:uuid-test-456"
	repo.executor = &model.SchedulerExecutor{
		ID:           executorID,
		RelayAgentID: "agent-bound",
		Enabled:      &existingEnabled,
	}
	svc := NewService(repo, nil, nil, nil)

	_, err := svc.RegisterServiceInstanceExecutor(
		executorID,
		"default",
		model.SlotGreen,
		map[string]string{"service_id": serviceID, "release_id": releaseID, "executor_id": executorID},
		"agent-other",
		executorID,
	)
	if err == nil {
		t.Fatalf("expected unauthorized rebind error")
	}
}

type staticLiveSlotResolver struct {
	slot model.Slot
}

func (r staticLiveSlotResolver) ResolveLiveSlot(uuid.UUID) (model.Slot, error) {
	return r.slot, nil
}

func TestAuthenticateExecutor_AutoDetectAllowsDirectForRelayConfiguredExecutor(t *testing.T) {
	repo := newAuthOnlyRepo()
	auth := config.LoadAgentAuthConfig()
	token := "token-direct"
	repo.executor = &model.SchedulerExecutor{
		ID:              "exec-1",
		TokenHash:       auth.HashToken(token),
		Group:           "g1",
		ChannelMode:     model.SchedulerExecutorChannelModeAgentRelay,
		RelayAgentID:    "agent-a",
		RelayRoutingKey: "rk-a",
		Enabled:         boolPtr(true),
	}
	svc := NewService(repo, auth, nil, nil)

	got, err := svc.AuthenticateExecutor("exec-1", token, "g1", 0, nil, "", "")
	if err != nil {
		t.Fatalf("AuthenticateExecutor() error = %v", err)
	}
	if got == nil || got.ID != "exec-1" {
		t.Fatalf("AuthenticateExecutor() returned unexpected executor: %+v", got)
	}
	if got.ChannelMode != model.SchedulerExecutorChannelModeDirect {
		t.Fatalf("expected channelMode direct, got %v", got.ChannelMode)
	}
	if got.RelayAgentID != "" || got.RelayRoutingKey != "" {
		t.Fatalf("expected relay fields cleared, got agent=%q routing=%q", got.RelayAgentID, got.RelayRoutingKey)
	}
}

func TestAuthenticateExecutor_AutoDetectAllowsRelayForDirectConfiguredExecutor(t *testing.T) {
	repo := newAuthOnlyRepo()
	auth := config.LoadAgentAuthConfig()
	token := "token-relay"
	repo.executor = &model.SchedulerExecutor{
		ID:          "exec-2",
		TokenHash:   auth.HashToken(token),
		Group:       "g1",
		ChannelMode: model.SchedulerExecutorChannelModeDirect,
		Enabled:     boolPtr(true),
	}
	svc := NewService(repo, auth, nil, nil)

	got, err := svc.AuthenticateExecutor("exec-2", token, "g1", 0, nil, "agent-b", "rk-b")
	if err != nil {
		t.Fatalf("AuthenticateExecutor() error = %v", err)
	}
	if got == nil || got.ID != "exec-2" {
		t.Fatalf("AuthenticateExecutor() returned unexpected executor: %+v", got)
	}
	if got.ChannelMode != model.SchedulerExecutorChannelModeAgentRelay {
		t.Fatalf("expected channelMode relay, got %v", got.ChannelMode)
	}
	if got.RelayAgentID != "agent-b" || got.RelayRoutingKey != "rk-b" {
		t.Fatalf("expected relay fields updated, got agent=%q routing=%q", got.RelayAgentID, got.RelayRoutingKey)
	}
}

func TestAuthenticateExecutor_RelayRebindAllowed(t *testing.T) {
	repo := newAuthOnlyRepo()
	auth := config.LoadAgentAuthConfig()
	token := "token-bind"
	repo.executor = &model.SchedulerExecutor{
		ID:              "exec-3",
		TokenHash:       auth.HashToken(token),
		Group:           "g1",
		ChannelMode:     model.SchedulerExecutorChannelModeAgentRelay,
		RelayAgentID:    "agent-bound",
		RelayRoutingKey: "rk-bound",
		Enabled:         boolPtr(true),
	}
	svc := NewService(repo, auth, nil, nil)

	got, err := svc.AuthenticateExecutor("exec-3", token, "g1", 0, nil, "agent-other", "rk-other")
	if err != nil {
		t.Fatalf("AuthenticateExecutor() error = %v", err)
	}
	if got == nil {
		t.Fatalf("expected executor output")
	}
	if got.RelayAgentID != "agent-other" || got.RelayRoutingKey != "rk-other" {
		t.Fatalf("expected relay binding updated, got agent=%q routing=%q", got.RelayAgentID, got.RelayRoutingKey)
	}
}

func TestCompleteRunSuccessClearsLeaseAndError(t *testing.T) {
	runID := uuid.New()
	now := time.Now().UTC()
	repo := newRunStateRepo(&model.SchedulerJobRun{
		ID:             runID,
		Status:         model.SchedulerJobRunStatusRunning,
		Attempt:        1,
		MaxRetries:     3,
		LeasedBy:       "exec-1",
		LeaseExpiresAt: timePtr(now.Add(time.Minute)),
		ErrorMessage:   "previous failure",
	})
	svc := NewService(repo, nil, nil, nil)

	if err := svc.CompleteRun(runID, "exec-1", true, false, "ignored"); err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}
	if repo.run.Status != model.SchedulerJobRunStatusSucceeded {
		t.Fatalf("status = %v, want succeeded", repo.run.Status)
	}
	if repo.run.LeasedBy != "" || repo.run.LeaseExpiresAt != nil {
		t.Fatalf("expected lease to be cleared, leasedBy=%q lease=%v", repo.run.LeasedBy, repo.run.LeaseExpiresAt)
	}
	if repo.run.ErrorMessage != "" {
		t.Fatalf("expected error message cleared, got %q", repo.run.ErrorMessage)
	}
	if repo.run.CompletedAt == nil {
		t.Fatalf("expected completedAt to be set")
	}
}

func TestCompleteRunRejectsTerminalRun(t *testing.T) {
	runID := uuid.New()
	repo := newRunStateRepo(&model.SchedulerJobRun{
		ID:       runID,
		Status:   model.SchedulerJobRunStatusSucceeded,
		LeasedBy: "exec-1",
	})
	svc := NewService(repo, nil, nil, nil)

	if err := svc.CompleteRun(runID, "exec-1", true, false, ""); err == nil {
		t.Fatalf("expected terminal run conflict")
	}
	if repo.completeCalled {
		t.Fatalf("terminal run should not call repository completion update")
	}
}

func TestDispatchDueRunsMarksExpiredRunFailedAfterRetriesExhausted(t *testing.T) {
	runID := uuid.New()
	now := time.Now().UTC()
	repo := newRunStateRepo(&model.SchedulerJobRun{
		ID:             runID,
		Status:         model.SchedulerJobRunStatusRunning,
		Attempt:        4,
		MaxRetries:     3,
		LeasedBy:       "exec-1",
		LeaseExpiresAt: timePtr(now.Add(-time.Second)),
	})
	repo.dispatchable = []model.SchedulerJobRun{*repo.run}
	svc := NewService(repo, nil, nil, nil)

	if err := svc.DispatchDueRuns(now, fakeRunDispatcher{}); err != nil {
		t.Fatalf("DispatchDueRuns() error = %v", err)
	}
	if repo.run.Status != model.SchedulerJobRunStatusFailed {
		t.Fatalf("status = %v, want failed", repo.run.Status)
	}
	if repo.run.LeasedBy != "" || repo.run.LeaseExpiresAt != nil {
		t.Fatalf("expected expired run lease to be cleared")
	}
	if repo.run.CompletedAt == nil {
		t.Fatalf("expected completedAt to be set")
	}
}

func TestShouldFailExpiredRunRequiresExhaustedRetries(t *testing.T) {
	now := time.Now().UTC()
	run := &model.SchedulerJobRun{
		Status:         model.SchedulerJobRunStatusRunning,
		Attempt:        3,
		MaxRetries:     3,
		LeaseExpiresAt: timePtr(now.Add(-time.Second)),
	}
	if shouldFailExpiredRun(run, now) {
		t.Fatalf("attempt equal to maxRetries should still be reclaimable")
	}
	run.Attempt = 4
	if !shouldFailExpiredRun(run, now) {
		t.Fatalf("expected expired run with exhausted retries to fail")
	}
}

func TestPickExecutorAndClaimSkipsDisabledExecutor(t *testing.T) {
	enabled := false
	lastSeen := time.Now().UTC()
	repo := newRunStateRepo(nil)
	repo.executor = &model.SchedulerExecutor{
		ID:         "exec-1",
		Group:      "default",
		Enabled:    &enabled,
		LastSeenAt: &lastSeen,
	}
	svc := NewService(repo, nil, nil, nil)
	run := &model.SchedulerJobRun{
		ID:              uuid.New(),
		JobID:           uuid.New(),
		Status:          model.SchedulerJobRunStatusPending,
		DispatchPolicy:  model.SchedulerDispatchPolicyRoundRobin,
		ExecutorGroup:   "default",
		LeaseTimeoutSec: 60,
	}

	executorID, _, err := svc.pickExecutorAndClaim(repo, run, time.Now().UTC(), fakeRunDispatcher{})
	if !errors.Is(err, domain.ErrExecutorOffline) {
		t.Fatalf("pickExecutorAndClaim() error = %v, want ErrExecutorOffline", err)
	}
	if executorID != "" {
		t.Fatalf("expected no executor to be selected, got %q", executorID)
	}
	if repo.claimCalled {
		t.Fatalf("disabled executor should not be claimed")
	}
}

func TestPickExecutorAndClaimSkipsStaleExecutor(t *testing.T) {
	now := time.Now().UTC()
	lastSeen := now.Add(-time.Minute)
	repo := newRunStateRepo(nil)
	repo.executor = &model.SchedulerExecutor{
		ID:         "exec-1",
		Group:      "default",
		Enabled:    boolPtr(true),
		LastSeenAt: &lastSeen,
	}
	svc := NewService(repo, nil, &config.SchedulerConfig{DefaultLeaseSec: 60, DefaultMaxRetries: 3, HeartbeatTimeout: 15 * time.Second}, nil)
	run := &model.SchedulerJobRun{
		ID:              uuid.New(),
		JobID:           uuid.New(),
		Status:          model.SchedulerJobRunStatusPending,
		DispatchPolicy:  model.SchedulerDispatchPolicyRoundRobin,
		ExecutorGroup:   "default",
		LeaseTimeoutSec: 60,
	}

	executorID, _, err := svc.pickExecutorAndClaim(repo, run, now, fakeRunDispatcher{})
	if !errors.Is(err, domain.ErrExecutorOffline) {
		t.Fatalf("pickExecutorAndClaim() error = %v, want ErrExecutorOffline", err)
	}
	if executorID != "" {
		t.Fatalf("expected no executor to be selected, got %q", executorID)
	}
	if repo.claimCalled {
		t.Fatalf("stale executor should not be claimed")
	}
}

func TestListExecutorsIncludesOnlineStatus(t *testing.T) {
	now := time.Now().UTC()
	repo := newAuthOnlyRepo()
	repo.executor = &model.SchedulerExecutor{
		ID:         "exec-1",
		Group:      "default",
		Enabled:    boolPtr(true),
		LastSeenAt: &now,
	}
	svc := NewService(repo, nil, &config.SchedulerConfig{HeartbeatTimeout: time.Minute}, nil)

	out, err := svc.ListExecutors()
	if err != nil {
		t.Fatalf("ListExecutors() error = %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one executor, got %d", len(out))
	}
	if !out[0].Online {
		t.Fatalf("expected executor to be online")
	}
}

func TestEnqueueDueJobsUsesStableIdempotencyKeyAndAdvancesDuplicate(t *testing.T) {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	jobID := uuid.New()
	enabled := true
	nextRun := now.Add(-time.Minute)
	repo := newEnqueueRepo(&model.SchedulerJob{
		ID:              jobID,
		Name:            "nightly",
		HandlerKey:      "handler",
		ServiceID:       uuid.New(),
		ScheduleKind:    model.SchedulerScheduleKindCron,
		CronExpr:        "*/5 * * * *",
		NextRunAt:       &nextRun,
		Enabled:         &enabled,
		DispatchPolicy:  model.SchedulerDispatchPolicyRoundRobin,
		ExecutorGroup:   "default",
		LeaseTimeoutSec: 60,
		MaxRetries:      3,
	})
	repo.runAlreadyExists = true
	svc := NewService(repo, nil, &config.SchedulerConfig{DispatchBatchSize: 100, DefaultLeaseSec: 60, DefaultMaxRetries: 3}, nil)

	if err := svc.EnqueueDueJobs(now); err != nil {
		t.Fatalf("EnqueueDueJobs() error = %v", err)
	}
	wantKey := scheduledRunIdempotencyKey(jobID, nextRun)
	if repo.createdRun == nil || repo.createdRun.IdempotencyKey != wantKey {
		t.Fatalf("idempotencyKey = %q, want %q", repo.createdRun.IdempotencyKey, wantKey)
	}
	if repo.job.NextRunAt == nil || !repo.job.NextRunAt.After(nextRun) {
		t.Fatalf("expected nextRunAt advanced after duplicate insert, got %v", repo.job.NextRunAt)
	}
	if repo.job.Enabled == nil || !*repo.job.Enabled {
		t.Fatalf("expected cron job to remain enabled")
	}
}

func TestRunDueCycleSkipsWhenEngineLockNotAcquired(t *testing.T) {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	enabled := true
	nextRun := now.Add(-time.Minute)
	repo := newEnqueueRepo(&model.SchedulerJob{
		ID:              uuid.New(),
		Name:            "nightly",
		HandlerKey:      "handler",
		ServiceID:       uuid.New(),
		ScheduleKind:    model.SchedulerScheduleKindCron,
		CronExpr:        "*/5 * * * *",
		NextRunAt:       &nextRun,
		Enabled:         &enabled,
		DispatchPolicy:  model.SchedulerDispatchPolicyRoundRobin,
		ExecutorGroup:   "default",
		LeaseTimeoutSec: 60,
		MaxRetries:      3,
	})
	repo.engineLockAcquired = false
	svc := NewService(repo, nil, &config.SchedulerConfig{DispatchBatchSize: 100, DefaultLeaseSec: 60, DefaultMaxRetries: 3}, nil)

	acquired, err := svc.RunDueCycle(now, fakeRunDispatcher{})
	if err != nil {
		t.Fatalf("RunDueCycle() error = %v", err)
	}
	if acquired {
		t.Fatalf("expected engine lock not acquired")
	}
	if repo.createdRun != nil {
		t.Fatalf("expected no run to be created without engine lock")
	}
	if repo.job.NextRunAt == nil || !repo.job.NextRunAt.Equal(nextRun) {
		t.Fatalf("nextRunAt changed without lock: got %v want %v", repo.job.NextRunAt, nextRun)
	}
}

type authOnlyRepo struct {
	executor *model.SchedulerExecutor
}

func newAuthOnlyRepo() *authOnlyRepo {
	return &authOnlyRepo{}
}

func (r *authOnlyRepo) CreateJob(job *model.SchedulerJob) error {
	panic("not implemented")
}

func (r *authOnlyRepo) UpdateJob(job *model.SchedulerJob) error {
	panic("not implemented")
}

func (r *authOnlyRepo) GetJob(id uuid.UUID) (*model.SchedulerJob, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ListJobs() ([]model.SchedulerJob, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ListJobsDue(now time.Time, limit int) ([]model.SchedulerJob, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) DeleteJob(id uuid.UUID) error {
	panic("not implemented")
}

func (r *authOnlyRepo) CreateRun(run *model.SchedulerJobRun) error {
	panic("not implemented")
}

func (r *authOnlyRepo) CreateRunIfNotExists(run *model.SchedulerJobRun) (bool, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) UpdateRun(run *model.SchedulerJobRun) error {
	panic("not implemented")
}

func (r *authOnlyRepo) GetRun(id uuid.UUID) (*model.SchedulerJobRun, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ListRunsByJob(jobID uuid.UUID, limit int) ([]model.SchedulerJobRun, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ListAllRuns(limit int) ([]model.SchedulerJobRun, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ListDispatchableRuns(now time.Time, limit int) ([]model.SchedulerJobRun, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ClaimRun(runID uuid.UUID, leasedBy string, leaseExpiresAt time.Time, now time.Time) (bool, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) MarkRunRunning(runID uuid.UUID, executorID string, startedAt time.Time) (bool, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) RenewRunLease(runID uuid.UUID, executorID string, leaseExpiresAt time.Time, now time.Time) (bool, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) CompleteRun(runID uuid.UUID, executorID string, status model.SchedulerJobRunStatus, attempt int, nextRetryAt *time.Time, completedAt *time.Time, errorMessage string) (bool, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) MarkRunDispatchFailed(runID uuid.UUID, executorID string, errorMessage string) (bool, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) MarkExpiredRunFailed(runID uuid.UUID, now time.Time, errorMessage string) (bool, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) GetDispatchCursor(jobID uuid.UUID, executorGroup string) (*model.SchedulerDispatchCursor, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) SaveDispatchCursor(cursor *model.SchedulerDispatchCursor) error {
	panic("not implemented")
}

func (r *authOnlyRepo) UpsertExecutor(executor *model.SchedulerExecutor) error {
	if executor == nil {
		return nil
	}
	copy := *executor
	r.executor = &copy
	return nil
}

func (r *authOnlyRepo) GetExecutor(id string) (*model.SchedulerExecutor, error) {
	if r.executor == nil || r.executor.ID != id {
		return nil, nil
	}
	copy := *r.executor
	return &copy, nil
}

func (r *authOnlyRepo) ListExecutorsByGroup(group string) ([]model.SchedulerExecutor, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ListExecutors() ([]model.SchedulerExecutor, error) {
	if r.executor == nil {
		return nil, nil
	}
	copy := *r.executor
	return []model.SchedulerExecutor{copy}, nil
}

func (r *authOnlyRepo) DeleteExecutor(id string) error {
	panic("not implemented")
}

func (r *authOnlyRepo) MarkExecutorSeen(id string, at time.Time) error {
	panic("not implemented")
}

func (r *authOnlyRepo) WithTx(fn func(tx domain.Repository) error) error {
	panic("not implemented")
}

func (r *authOnlyRepo) WithEngineLock(lockKey int64, fn func(tx domain.Repository) error) (bool, error) {
	panic("not implemented")
}

type runStateRepo struct {
	*authOnlyRepo
	run            *model.SchedulerJobRun
	dispatchable   []model.SchedulerJobRun
	completeCalled bool
	claimCalled    bool
}

func newRunStateRepo(run *model.SchedulerJobRun) *runStateRepo {
	return &runStateRepo{authOnlyRepo: newAuthOnlyRepo(), run: run}
}

func (r *runStateRepo) GetRun(id uuid.UUID) (*model.SchedulerJobRun, error) {
	if r.run == nil || r.run.ID != id {
		return nil, nil
	}
	copy := *r.run
	return &copy, nil
}

func (r *runStateRepo) ListDispatchableRuns(now time.Time, limit int) ([]model.SchedulerJobRun, error) {
	return append([]model.SchedulerJobRun(nil), r.dispatchable...), nil
}

func (r *runStateRepo) ClaimRun(runID uuid.UUID, leasedBy string, leaseExpiresAt time.Time, now time.Time) (bool, error) {
	r.claimCalled = true
	if r.run == nil || r.run.ID != runID {
		return false, nil
	}
	r.run.Status = model.SchedulerJobRunStatusDispatched
	r.run.LeasedBy = leasedBy
	r.run.LeaseExpiresAt = &leaseExpiresAt
	return true, nil
}

func (r *runStateRepo) MarkRunRunning(runID uuid.UUID, executorID string, startedAt time.Time) (bool, error) {
	if r.run == nil || r.run.ID != runID || r.run.LeasedBy != executorID || r.run.Status.IsTerminal() {
		return false, nil
	}
	r.run.Status = model.SchedulerJobRunStatusRunning
	if r.run.StartedAt == nil {
		r.run.StartedAt = &startedAt
	}
	return true, nil
}

func (r *runStateRepo) RenewRunLease(runID uuid.UUID, executorID string, leaseExpiresAt time.Time, now time.Time) (bool, error) {
	if r.run == nil || r.run.ID != runID || r.run.LeasedBy != executorID || r.run.LeaseExpiresAt == nil || !r.run.LeaseExpiresAt.After(now) {
		return false, nil
	}
	r.run.LeaseExpiresAt = &leaseExpiresAt
	return true, nil
}

func (r *runStateRepo) CompleteRun(runID uuid.UUID, executorID string, status model.SchedulerJobRunStatus, attempt int, nextRetryAt *time.Time, completedAt *time.Time, errorMessage string) (bool, error) {
	r.completeCalled = true
	if r.run == nil || r.run.ID != runID || r.run.LeasedBy != executorID || r.run.Status.IsTerminal() {
		return false, nil
	}
	r.run.Status = status
	r.run.Attempt = attempt
	r.run.NextRetryAt = nextRetryAt
	r.run.CompletedAt = completedAt
	r.run.LeaseExpiresAt = nil
	r.run.LeasedBy = ""
	r.run.ErrorMessage = errorMessage
	return true, nil
}

func (r *runStateRepo) MarkExpiredRunFailed(runID uuid.UUID, now time.Time, errorMessage string) (bool, error) {
	if !shouldFailExpiredRun(r.run, now) || r.run.ID != runID {
		return false, nil
	}
	r.run.Status = model.SchedulerJobRunStatusFailed
	r.run.CompletedAt = &now
	r.run.LeasedBy = ""
	r.run.LeaseExpiresAt = nil
	r.run.ErrorMessage = errorMessage
	return true, nil
}

type fakeRunDispatcher struct{}

func (fakeRunDispatcher) ListOnlineExecutors(group string) []OnlineExecutor {
	return []OnlineExecutor{{ExecutorID: "exec-1", Group: group}}
}

func (fakeRunDispatcher) DispatchRun(executorID string, run *model.SchedulerJobRun) error {
	return nil
}

type enqueueRepo struct {
	*authOnlyRepo
	job                *model.SchedulerJob
	createdRun         *model.SchedulerJobRun
	runAlreadyExists   bool
	engineLockAcquired bool
}

func newEnqueueRepo(job *model.SchedulerJob) *enqueueRepo {
	return &enqueueRepo{authOnlyRepo: newAuthOnlyRepo(), job: job, engineLockAcquired: true}
}

func (r *enqueueRepo) ListJobsDue(now time.Time, limit int) ([]model.SchedulerJob, error) {
	if r.job == nil || r.job.Enabled == nil || !*r.job.Enabled || r.job.NextRunAt == nil || r.job.NextRunAt.After(now) {
		return nil, nil
	}
	copy := *r.job
	return []model.SchedulerJob{copy}, nil
}

func (r *enqueueRepo) GetJob(id uuid.UUID) (*model.SchedulerJob, error) {
	if r.job == nil || r.job.ID != id {
		return nil, nil
	}
	copy := *r.job
	return &copy, nil
}

func (r *enqueueRepo) CreateRunIfNotExists(run *model.SchedulerJobRun) (bool, error) {
	copy := *run
	r.createdRun = &copy
	return !r.runAlreadyExists, nil
}

func (r *enqueueRepo) UpdateJob(job *model.SchedulerJob) error {
	copy := *job
	r.job = &copy
	return nil
}

func (r *enqueueRepo) WithTx(fn func(tx domain.Repository) error) error {
	return fn(r)
}

func (r *enqueueRepo) WithEngineLock(lockKey int64, fn func(tx domain.Repository) error) (bool, error) {
	if !r.engineLockAcquired {
		return false, nil
	}
	return true, fn(r)
}

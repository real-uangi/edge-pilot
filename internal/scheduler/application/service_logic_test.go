package application

import (
	"edge-pilot/internal/scheduler/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestIsReleaseLinkedTaskType(t *testing.T) {
	if !isReleaseLinkedTaskType("release.deploy") {
		t.Fatalf("expected release.deploy to be linked")
	}
	if !isReleaseLinkedTaskType("release_switch") {
		t.Fatalf("expected release_switch to be linked")
	}
	if isReleaseLinkedTaskType("custom.task") {
		t.Fatalf("expected custom.task not linked")
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

func (r *authOnlyRepo) UpdateRun(run *model.SchedulerJobRun) error {
	panic("not implemented")
}

func (r *authOnlyRepo) GetRun(id uuid.UUID) (*model.SchedulerJobRun, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ListRunsByJob(jobID uuid.UUID, limit int) ([]model.SchedulerJobRun, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ListDispatchableRuns(now time.Time, limit int) ([]model.SchedulerJobRun, error) {
	panic("not implemented")
}

func (r *authOnlyRepo) ClaimRun(runID uuid.UUID, leasedBy string, leaseExpiresAt time.Time, now time.Time) (bool, error) {
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
	panic("not implemented")
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

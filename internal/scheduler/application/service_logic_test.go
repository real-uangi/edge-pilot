package application

import (
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"testing"
	"time"
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

func TestParseExecutorChannelMode(t *testing.T) {
	if mode, err := parseExecutorChannelMode("direct"); err != nil || mode != model.SchedulerExecutorChannelModeDirect {
		t.Fatalf("parseExecutorChannelMode(direct) = (%v, %v)", mode, err)
	}
	if mode, err := parseExecutorChannelMode("agent_relay"); err != nil || mode != model.SchedulerExecutorChannelModeAgentRelay {
		t.Fatalf("parseExecutorChannelMode(agent_relay) = (%v, %v)", mode, err)
	}
	if _, err := parseExecutorChannelMode("bad_mode"); err == nil {
		t.Fatalf("expected parseExecutorChannelMode(bad_mode) to fail")
	}
}

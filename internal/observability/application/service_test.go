package application

import (
	"context"
	"testing"
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
	"github.com/real-uangi/edge-pilot/internal/shared/model"
	"github.com/real-uangi/edge-pilot/internal/shared/perf"

	"github.com/google/uuid"
)

func TestRecordStatsStoresAgentPerformanceInRing(t *testing.T) {
	repo := &fakeObservabilityRepo{}
	service := NewService(repo, &fakeAgentOverviewReader{}, nil, nil, &fakePerfCollector{})

	agentID := "11111111-1111-1111-1111-111111111111"
	err := service.RecordStats(&grpcapi.StatsReport{
		AgentId: agentID,
		SelfPerformance: &grpcapi.PerformanceSnapshot{
			CpuPercent:        45.8,
			MemoryUsedBytes:   1024,
			MemoryLimitBytes:  2048,
			Source:            perf.SourceCgroup,
			CollectedAtUnixMs: time.Now().UnixMilli(),
		},
	})
	if err != nil {
		t.Fatalf("RecordStats() error = %v", err)
	}

	history, err := service.GetAgentPerformanceHistory(agentID)
	if err != nil {
		t.Fatalf("GetAgentPerformanceHistory() error = %v", err)
	}
	if len(history.History) != 1 {
		t.Fatalf("expected 1 history point, got %d", len(history.History))
	}
	if history.History[0].CPUPercent != 45.8 {
		t.Fatalf("expected cpu=45.8, got %v", history.History[0].CPUPercent)
	}
}

func TestGetSystemPerformanceOverviewIncludesControlPlaneAndAgentLatest(t *testing.T) {
	collector := &fakePerfCollector{
		snapshot: &perf.Snapshot{
			CPUPercent:       21.4,
			MemoryUsedBytes:  4096,
			MemoryLimitBytes: 8192,
			Source:           perf.SourceGopsutil,
			CollectedAt:      time.Now(),
		},
	}
	repo := &fakeObservabilityRepo{}
	agents := &fakeAgentOverviewReader{
		agents: []dto.AgentOverview{
			{ID: "11111111-1111-1111-1111-111111111111", Hostname: "agent-a"},
			{ID: "22222222-2222-2222-2222-222222222222", Hostname: "agent-b"},
		},
	}
	service := NewService(repo, agents, nil, nil, collector)
	_ = service.RecordStats(&grpcapi.StatsReport{
		AgentId: "11111111-1111-1111-1111-111111111111",
		SelfPerformance: &grpcapi.PerformanceSnapshot{
			CpuPercent:        10.2,
			MemoryUsedBytes:   100,
			MemoryLimitBytes:  200,
			Source:            perf.SourceCgroup,
			CollectedAtUnixMs: time.Now().UnixMilli(),
		},
	})

	overview, err := service.GetSystemPerformanceOverview()
	if err != nil {
		t.Fatalf("GetSystemPerformanceOverview() error = %v", err)
	}
	if overview.ControlPlaneLatest == nil {
		t.Fatal("expected control-plane latest snapshot")
	}
	if overview.ControlPlaneLatest.CPUPercent != 21.4 {
		t.Fatalf("expected control-plane cpu=21.4, got %v", overview.ControlPlaneLatest.CPUPercent)
	}
	if len(overview.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(overview.Agents))
	}
	if overview.Agents[0].Latest == nil {
		t.Fatal("expected first agent latest snapshot")
	}
	if overview.Agents[1].Latest != nil {
		t.Fatal("expected second agent without snapshot")
	}
}

type fakeObservabilityRepo struct{}

func (f *fakeObservabilityRepo) SaveBackendStats([]model.BackendStatSnapshot) error {
	return nil
}

func (f *fakeObservabilityRepo) ListBackendStats(uuid.UUID) ([]model.BackendStatSnapshot, error) {
	return nil, nil
}

func (f *fakeObservabilityRepo) CountActiveInstances() (int64, error) {
	return 0, nil
}

type fakeAgentOverviewReader struct {
	agents []dto.AgentOverview
}

func (f *fakeAgentOverviewReader) List() ([]dto.AgentOverview, error) {
	return f.agents, nil
}

type fakePerfCollector struct {
	snapshot *perf.Snapshot
	err      error
}

func (f *fakePerfCollector) Collect(context.Context) (*perf.Snapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}

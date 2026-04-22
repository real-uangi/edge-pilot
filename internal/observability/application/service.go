package application

import (
	"context"
	releaseapp "edge-pilot/internal/release/application"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"edge-pilot/internal/shared/perf"
	"strings"
	"sync"
	"time"

	"edge-pilot/internal/observability/domain"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/log"
)

const (
	performanceRingCapacity = 240
)

type AgentOverviewReader interface {
	List() ([]dto.AgentOverview, error)
}

type Service struct {
	repo             domain.Repository
	agents           AgentOverviewReader
	services         *servicecatalogapp.Service
	releases         *releaseapp.Service
	collector        perf.Collector
	controlPlaneRing *snapshotRing
	agentRings       map[string]*snapshotRing
	agentRingsMu     sync.RWMutex
	logger           *log.StdLogger
}

func NewService(
	repo domain.Repository,
	agents AgentOverviewReader,
	services *servicecatalogapp.Service,
	releases *releaseapp.Service,
	collector perf.Collector,
) *Service {
	return &Service{
		repo:             repo,
		agents:           agents,
		services:         services,
		releases:         releases,
		collector:        collector,
		controlPlaneRing: newSnapshotRing(performanceRingCapacity),
		agentRings:       make(map[string]*snapshotRing),
		logger:           log.NewStdLogger("observability.service"),
	}
}

func (s *Service) RecordStats(report *grpcapi.StatsReport) error {
	stats := make([]model.BackendStatSnapshot, 0, len(report.GetServices()))
	for _, item := range report.GetServices() {
		serviceID, err := uuid.Parse(item.GetServiceId())
		if err != nil {
			continue
		}
		stats = append(stats, model.BackendStatSnapshot{
			ID:            uuid.New(),
			ServiceID:     serviceID,
			BackendName:   item.GetBackendName(),
			ServerName:    item.GetServerName(),
			Scur:          item.GetScur(),
			Rate:          item.GetRate(),
			ErrorRequests: item.GetErrorRequests(),
		})
	}
	if report.GetSelfPerformance() != nil {
		s.recordAgentPerformance(report.GetAgentId(), fromProtoPerformance(report.GetSelfPerformance()))
	}
	return s.repo.SaveBackendStats(stats)
}

func (s *Service) GetOverview() (*dto.OverviewOutput, error) {
	agents, err := s.agents.List()
	if err != nil {
		return nil, err
	}
	services, err := s.services.List()
	if err != nil {
		return nil, err
	}
	releases, err := s.releases.List()
	if err != nil {
		return nil, err
	}
	activeInstances, err := s.repo.CountActiveInstances()
	if err != nil {
		return nil, err
	}
	return &dto.OverviewOutput{
		Agents:          agents,
		Services:        services,
		RecentReleases:  takeFirst(releases, 10),
		ActiveInstances: int(activeInstances),
	}, nil
}

func (s *Service) GetServiceObservability(serviceID uuid.UUID) (*dto.ObservabilityOutput, error) {
	instances, err := s.releases.GetRuntimeInstances(serviceID)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.ListBackendStats(serviceID)
	if err != nil {
		return nil, err
	}
	output := &dto.ObservabilityOutput{
		ServiceID:        serviceID,
		RuntimeInstances: make([]dto.RuntimeInstanceOutput, 0, len(instances)),
		BackendStats:     make([]dto.BackendStatOutput, 0, len(stats)),
	}
	for _, instance := range instances {
		output.RuntimeInstances = append(output.RuntimeInstances, dto.RuntimeInstanceOutput{
			ID:               instance.ID,
			ServiceID:        instance.ServiceID,
			ReleaseID:        instance.ReleaseID,
			Slot:             instance.Slot,
			ContainerID:      instance.ContainerID,
			ImageTag:         instance.ImageTag,
			ListenAddress:    instance.ListenAddress,
			HostPort:         instance.HostPort,
			ServerName:       instance.ServerName,
			Healthy:          instance.Healthy,
			AcceptingTraffic: instance.AcceptingTraffic,
			Active:           instance.Active,
			UpdatedAt:        instance.UpdatedAt,
		})
	}
	for _, item := range stats {
		output.BackendStats = append(output.BackendStats, dto.BackendStatOutput{
			ServiceID:     item.ServiceID,
			BackendName:   item.BackendName,
			ServerName:    item.ServerName,
			Scur:          item.Scur,
			Rate:          item.Rate,
			ErrorRequests: item.ErrorRequests,
			CreatedAt:     item.CreatedAt,
		})
	}
	return output, nil
}

func (s *Service) SampleControlPlanePerformance(ctx context.Context) error {
	snapshot, err := s.collector.Collect(ctx)
	if err != nil {
		return err
	}
	s.controlPlaneRing.append(*snapshot)
	return nil
}

func (s *Service) GetSystemPerformanceOverview() (*dto.SystemPerformanceOverviewOutput, error) {
	if _, ok := s.controlPlaneRing.latest(); !ok {
		if err := s.SampleControlPlanePerformance(context.Background()); err != nil {
			s.logger.Errorf(err, "control-plane performance sampling failed")
		}
	}
	controlPlaneLatest, hasControlPlaneLatest := s.controlPlaneRing.latest()
	controlPlaneHistory := s.controlPlaneRing.list()
	agentList, err := s.agents.List()
	if err != nil {
		return nil, err
	}
	output := &dto.SystemPerformanceOverviewOutput{
		ControlPlaneHistory: make([]dto.PerformancePointOutput, 0, len(controlPlaneHistory)),
		Agents:              make([]dto.AgentPerformanceLatestOutput, 0, len(agentList)),
	}
	if hasControlPlaneLatest {
		latest := toOutputPerformancePoint(controlPlaneLatest)
		output.ControlPlaneLatest = &latest
	}
	for _, point := range controlPlaneHistory {
		output.ControlPlaneHistory = append(output.ControlPlaneHistory, toOutputPerformancePoint(point))
	}
	for _, agent := range agentList {
		var latest *dto.PerformancePointOutput
		if ring := s.getAgentRing(agent.ID); ring != nil {
			if snapshot, ok := ring.latest(); ok {
				value := toOutputPerformancePoint(snapshot)
				latest = &value
			}
		}
		output.Agents = append(output.Agents, dto.AgentPerformanceLatestOutput{
			ID:       agent.ID,
			Hostname: agent.Hostname,
			IP:       agent.IP,
			Enabled:  agent.Enabled,
			Online:   agent.Online,
			Latest:   latest,
		})
	}
	return output, nil
}

func (s *Service) GetAgentPerformanceHistory(agentID string) (*dto.AgentPerformanceHistoryOutput, error) {
	history := make([]dto.PerformancePointOutput, 0, performanceRingCapacity)
	if ring := s.getAgentRing(agentID); ring != nil {
		snapshots := ring.list()
		history = make([]dto.PerformancePointOutput, 0, len(snapshots))
		for _, snapshot := range snapshots {
			history = append(history, toOutputPerformancePoint(snapshot))
		}
	}
	return &dto.AgentPerformanceHistoryOutput{
		AgentID: agentID,
		History: history,
	}, nil
}

func (s *Service) recordAgentPerformance(agentID string, snapshot perf.Snapshot) {
	trimmedAgentID := strings.TrimSpace(agentID)
	if trimmedAgentID == "" {
		return
	}
	ring := s.getOrCreateAgentRing(trimmedAgentID)
	ring.append(snapshot)
}

func (s *Service) getOrCreateAgentRing(agentID string) *snapshotRing {
	s.agentRingsMu.RLock()
	ring := s.agentRings[agentID]
	s.agentRingsMu.RUnlock()
	if ring != nil {
		return ring
	}
	s.agentRingsMu.Lock()
	defer s.agentRingsMu.Unlock()
	if existing := s.agentRings[agentID]; existing != nil {
		return existing
	}
	created := newSnapshotRing(performanceRingCapacity)
	s.agentRings[agentID] = created
	return created
}

func (s *Service) getAgentRing(agentID string) *snapshotRing {
	s.agentRingsMu.RLock()
	defer s.agentRingsMu.RUnlock()
	return s.agentRings[agentID]
}

func toOutputPerformancePoint(snapshot perf.Snapshot) dto.PerformancePointOutput {
	return dto.PerformancePointOutput{
		CPUPercent:       snapshot.CPUPercent,
		MemoryUsedBytes:  snapshot.MemoryUsedBytes,
		MemoryLimitBytes: snapshot.MemoryLimitBytes,
		Source:           snapshot.Source,
		CollectedAt:      snapshot.CollectedAt,
	}
}

func fromProtoPerformance(snapshot *grpcapi.PerformanceSnapshot) perf.Snapshot {
	collectedAt := time.UnixMilli(snapshot.GetCollectedAtUnixMs())
	if snapshot.GetCollectedAtUnixMs() <= 0 {
		collectedAt = time.Now()
	}
	return perf.Snapshot{
		CPUPercent:       snapshot.GetCpuPercent(),
		MemoryUsedBytes:  snapshot.GetMemoryUsedBytes(),
		MemoryLimitBytes: snapshot.GetMemoryLimitBytes(),
		Source:           snapshot.GetSource(),
		CollectedAt:      collectedAt,
	}
}

func takeFirst[T any](items []T, limit int) []T {
	if len(items) <= limit {
		return items
	}
	out := make([]T, limit)
	copy(out, items[:limit])
	return out
}

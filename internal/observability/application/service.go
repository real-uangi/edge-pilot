package application

import (
	agentapp "edge-pilot/internal/agent/application"
	releaseapp "edge-pilot/internal/release/application"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"time"

	"edge-pilot/internal/observability/domain"

	"github.com/google/uuid"
)

type Service struct {
	repo     domain.Repository
	agents   *agentapp.RegistryService
	services *servicecatalogapp.Service
	releases *releaseapp.Service
}

func NewService(
	repo domain.Repository,
	agents *agentapp.RegistryService,
	services *servicecatalogapp.Service,
	releases *releaseapp.Service,
) *Service {
	return &Service{
		repo:     repo,
		agents:   agents,
		services: services,
		releases: releases,
	}
}

func (s *Service) RecordStats(report *grpcapi.StatsReport) error {
	stats := make([]model.BackendStatSnapshot, 0, len(report.Services))
	for _, item := range report.Services {
		serviceID, err := uuid.Parse(item.ServiceID)
		if err != nil {
			continue
		}
		stats = append(stats, model.BackendStatSnapshot{
			ID:            uuid.New(),
			ServiceID:     serviceID,
			BackendName:   item.BackendName,
			ServerName:    item.ServerName,
			Scur:          item.Scur,
			Rate:          item.Rate,
			ErrorRequests: item.ErrorRequests,
		})
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

func takeFirst[T any](items []T, limit int) []T {
	if len(items) <= limit {
		return items
	}
	out := make([]T, limit)
	copy(out, items[:limit])
	return out
}

var _ = time.Now

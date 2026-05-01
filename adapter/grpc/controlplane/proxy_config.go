package controlplane

import (
	releasedomain "edge-pilot/internal/release/domain"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	servicecatalogdomain "edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type ProxyConfigPublisher struct {
	repo     servicecatalogdomain.Repository
	releases releasedomain.Repository
	hub      *sessionHub
}

func NewProxyConfigPublisher(repo servicecatalogdomain.Repository, releases releasedomain.Repository, hub *sessionHub) servicecatalogdomain.ProxyConfigPublisher {
	return &ProxyConfigPublisher{
		repo:     repo,
		releases: releases,
		hub:      hub,
	}
}

func (p *ProxyConfigPublisher) PublishAgent(agentID string) error {
	services, err := p.repo.ListByAgent(agentID)
	if err != nil {
		return err
	}
	snapshot, err := p.buildProxyConfigSnapshot(agentID, services)
	if err != nil {
		return err
	}
	if err := p.hub.DispatchProxyConfig(agentID, snapshot); err != nil {
		if errors.Is(err, releasedomain.ErrAgentOffline) {
			return nil
		}
		return err
	}
	return nil
}

func (p *ProxyConfigPublisher) buildProxyConfigSnapshot(agentID string, services []model.Service) (*grpcapi.ProxyConfigSnapshot, error) {
	configs := servicecatalogapp.BuildProxyServiceConfigs(services)
	out := make([]*grpcapi.ProxyServiceConfig, 0, len(configs))
	for _, item := range configs {
		liveReleaseID, candidateReleaseID, candidateTrafficPercent, err := p.resolveReleases(item)
		if err != nil {
			return nil, err
		}
		out = append(out, &grpcapi.ProxyServiceConfig{
			ServiceId:               item.ServiceID.String(),
			ServiceKey:              item.ServiceKey,
			RouteHost:               item.RouteHost,
			RouteHosts:              item.RouteHosts,
			RoutePathPrefix:         item.RoutePathPrefix,
			BackendName:             item.BackendName,
			ContainerPort:           int32(item.ContainerPort),
			CurrentLiveSlot:         toProtoSlot(item.CurrentLiveSlot),
			LiveReleaseId:           liveReleaseID,
			LiveBackendName:         servicecatalogapp.BackendNameForRelease(item.ServiceID, liveReleaseID),
			CandidateReleaseId:      candidateReleaseID,
			CandidateBackendName:    servicecatalogapp.BackendNameForRelease(item.ServiceID, candidateReleaseID),
			CandidateTrafficPercent: int32(candidateTrafficPercent),
			SchedulerSdkPort:        int32(item.SchedulerSDKPort),
			SchedulerExecutorGroup:  item.SchedulerExecutorGroup,
		})
	}
	return &grpcapi.ProxyConfigSnapshot{
		AgentId:        agentID,
		FrontendName:   servicecatalogapp.SharedFrontendName,
		DefaultBackend: servicecatalogapp.SharedDefaultBackend,
		BindPort:       int32(servicecatalogapp.SharedFrontendBindPort),
		Services:       out,
	}, nil
}

func buildProxyConfigSnapshot(agentID string, services []model.Service) *grpcapi.ProxyConfigSnapshot {
	publisher := &ProxyConfigPublisher{}
	snapshot, err := publisher.buildProxyConfigSnapshot(agentID, services)
	if err != nil {
		return &grpcapi.ProxyConfigSnapshot{
			AgentId:        agentID,
			FrontendName:   servicecatalogapp.SharedFrontendName,
			DefaultBackend: servicecatalogapp.SharedDefaultBackend,
			BindPort:       int32(servicecatalogapp.SharedFrontendBindPort),
		}
	}
	return snapshot
}

func (p *ProxyConfigPublisher) resolveReleases(item servicecatalogapp.ProxyServiceConfig) (string, string, int, error) {
	if p.releases == nil {
		return "", "", 0, nil
	}
	instances, err := p.releases.ListRuntimeInstancesByService(item.ServiceID)
	if err != nil {
		return "", "", 0, err
	}
	liveSlot := normalizeLiveSlot(item.CurrentLiveSlot)
	candidateSlot := nextSlot(liveSlot)
	liveReleaseID := ""
	candidateReleaseID := ""
	for i := range instances {
		instance := instances[i]
		if instance.ServiceID != item.ServiceID {
			continue
		}
		if instance.Slot == liveSlot {
			liveReleaseID = instance.ReleaseID.String()
		}
		if instance.Slot == candidateSlot {
			candidateReleaseID = instance.ReleaseID.String()
		}
	}
	candidateTrafficPercent := 0
	candidateID, parseErr := uuid.Parse(strings.TrimSpace(candidateReleaseID))
	if parseErr == nil {
		candidateRelease, getErr := p.releases.GetRelease(candidateID)
		if getErr != nil {
			return "", "", 0, getErr
		}
		candidateTrafficPercent = candidateTrafficPercentForRelease(candidateRelease)
	}
	return liveReleaseID, candidateReleaseID, candidateTrafficPercent, nil
}

func candidateTrafficPercentForRelease(release *model.Release) int {
	if !isSplitActiveRelease(release) {
		return 0
	}
	return clampTrafficPercent(release.TrafficPercent)
}

func isSplitActiveRelease(release *model.Release) bool {
	if release == nil {
		return false
	}
	switch release.Status {
	case model.ReleaseStatusReadyToSwitch, model.ReleaseStatusSwitched:
		percent := clampTrafficPercent(release.TrafficPercent)
		return percent >= 1 && percent <= 99
	default:
		return false
	}
}

func normalizeLiveSlot(slot model.Slot) model.Slot {
	switch slot {
	case model.SlotBlue, model.SlotGreen:
		return slot
	default:
		return model.SlotBlue
	}
}

func nextSlot(slot model.Slot) model.Slot {
	switch slot {
	case model.SlotBlue:
		return model.SlotGreen
	case model.SlotGreen:
		return model.SlotBlue
	default:
		return model.SlotGreen
	}
}

func clampTrafficPercent(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

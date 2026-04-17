package controlplane

import (
	releasedomain "edge-pilot/internal/release/domain"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	servicecatalogdomain "edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"errors"
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
		blueReleaseID, greenReleaseID, err := p.allowedReleaseIDs(item)
		if err != nil {
			return nil, err
		}
		out = append(out, &grpcapi.ProxyServiceConfig{
			ServiceId:       item.ServiceID.String(),
			ServiceKey:      item.ServiceKey,
			RouteHost:       item.RouteHost,
			RoutePathPrefix: item.RoutePathPrefix,
			BackendName:     item.BackendName,
			BlueServerName:  blueReleaseID,
			GreenServerName: greenReleaseID,
			ContainerPort:   int32(item.ContainerPort),
			CurrentLiveSlot: toProtoSlot(item.CurrentLiveSlot),
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

func (p *ProxyConfigPublisher) allowedReleaseIDs(item servicecatalogapp.ProxyServiceConfig) (string, string, error) {
	if p.releases == nil {
		return "", "", nil
	}
	instances, err := p.releases.ListRuntimeInstancesByService(item.ServiceID)
	if err != nil {
		return "", "", err
	}
	liveReleaseID := ""
	targetReleaseID := ""
	for i := range instances {
		instance := instances[i]
		if instance.ServiceID != item.ServiceID {
			continue
		}
		if instance.Slot == item.CurrentLiveSlot {
			liveReleaseID = instance.ReleaseID.String()
		}
	}
	readyRelease, err := p.releases.FindReadyToSwitchRelease(item.ServiceID)
	if err != nil {
		return "", "", err
	}
	if readyRelease != nil {
		for i := range instances {
			instance := instances[i]
			if instance.Slot == readyRelease.TargetSlot {
				targetReleaseID = instance.ReleaseID.String()
				break
			}
		}
	}
	blue := ""
	green := ""
	switch item.CurrentLiveSlot {
	case model.SlotBlue:
		blue = liveReleaseID
		if readyRelease != nil && readyRelease.TargetSlot == model.SlotGreen {
			green = targetReleaseID
		}
	case model.SlotGreen:
		green = liveReleaseID
		if readyRelease != nil && readyRelease.TargetSlot == model.SlotBlue {
			blue = targetReleaseID
		}
	}
	return blue, green, nil
}

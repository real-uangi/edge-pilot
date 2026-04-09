package controlplane

import (
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	servicecatalogdomain "edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"errors"
)

type ProxyConfigPublisher struct {
	repo servicecatalogdomain.Repository
	hub  *sessionHub
}

func NewProxyConfigPublisher(repo servicecatalogdomain.Repository, hub *sessionHub) servicecatalogdomain.ProxyConfigPublisher {
	return &ProxyConfigPublisher{
		repo: repo,
		hub:  hub,
	}
}

func (p *ProxyConfigPublisher) PublishAgent(agentID string) error {
	services, err := p.repo.ListByAgent(agentID)
	if err != nil {
		return err
	}
	snapshot := buildProxyConfigSnapshot(agentID, services)
	if err := p.hub.DispatchProxyConfig(agentID, snapshot); err != nil {
		if errors.Is(err, ErrAgentOffline) {
			return nil
		}
		return err
	}
	return nil
}

func buildProxyConfigSnapshot(agentID string, services []model.Service) *grpcapi.ProxyConfigSnapshot {
	configs := servicecatalogapp.BuildProxyServiceConfigs(services)
	out := make([]*grpcapi.ProxyServiceConfig, 0, len(configs))
	for _, item := range configs {
		out = append(out, &grpcapi.ProxyServiceConfig{
			ServiceId:       item.ServiceID.String(),
			ServiceKey:      item.ServiceKey,
			RouteHost:       item.RouteHost,
			RoutePathPrefix: item.RoutePathPrefix,
			BackendName:     item.BackendName,
			BlueServerName:  servicecatalogapp.ServerName(model.SlotBlue),
			GreenServerName: servicecatalogapp.ServerName(model.SlotGreen),
			BlueHostPort:    int32(item.BlueHostPort),
			GreenHostPort:   int32(item.GreenHostPort),
			CurrentLiveSlot: toProtoSlot(item.CurrentLiveSlot),
		})
	}
	return &grpcapi.ProxyConfigSnapshot{
		AgentId:        agentID,
		FrontendName:   servicecatalogapp.SharedFrontendName,
		DefaultBackend: servicecatalogapp.SharedDefaultBackend,
		BindPort:       int32(servicecatalogapp.SharedFrontendBindPort),
		Services:       out,
	}
}

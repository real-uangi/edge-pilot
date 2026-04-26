package servicecatalog

import (
	"edge-pilot/internal/agent/application/registry"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/servicecatalog/infra"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"servicecatalog",
	fx.Provide(
		infra.NewRepository,
		func(reg *registry.RegistryService) domain.AgentLookup { return reg },
		func(repo domain.Repository) agentdomain.ServiceBindingChecker {
			return infra.NewAgentServiceBindingChecker(repo)
		},
		application.NewServiceWithPublisherAndCodecAndReleases,
	),
)

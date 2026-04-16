package servicecatalog

import (
	agentapp "edge-pilot/internal/agent/application"
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
		func(registry *agentapp.RegistryService) domain.AgentLookup { return registry },
		func(repo domain.Repository) agentdomain.ServiceBindingChecker {
			return infra.NewAgentServiceBindingChecker(repo)
		},
		application.NewServiceWithPublisherAndCodec,
	),
)

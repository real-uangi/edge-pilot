package servicecatalog

import (
	agentapp "edge-pilot/internal/agent/application"
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
		application.NewServiceWithPublisherAndCodec,
	),
)

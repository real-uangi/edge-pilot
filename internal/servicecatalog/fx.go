package servicecatalog

import (
	"github.com/real-uangi/edge-pilot/internal/agent/application/registry"
	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	"github.com/real-uangi/edge-pilot/internal/servicecatalog/application"
	"github.com/real-uangi/edge-pilot/internal/servicecatalog/domain"
	"github.com/real-uangi/edge-pilot/internal/servicecatalog/infra"

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

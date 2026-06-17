package release

import (
	"github.com/real-uangi/edge-pilot/internal/agent/application/registry"
	"github.com/real-uangi/edge-pilot/internal/release/application"
	"github.com/real-uangi/edge-pilot/internal/release/infra"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"release",
	fx.Provide(
		infra.NewRepository,
		application.NewServiceCatalogReleaseChecker,
		func(reg *registry.RegistryService) application.AgentOnlineChecker { return reg },
		application.NewServiceWithRegistryCredentialsAndCodec,
	),
)

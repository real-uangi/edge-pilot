package release

import (
	"edge-pilot/internal/agent/application/registry"
	"edge-pilot/internal/release/application"
	"edge-pilot/internal/release/infra"

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

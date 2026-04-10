package registrycredential

import (
	"edge-pilot/internal/registrycredential/application"
	"edge-pilot/internal/registrycredential/infra"
	releasedomain "edge-pilot/internal/release/domain"
	"edge-pilot/internal/shared/config"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"registry-credential",
	fx.Provide(
		config.LoadRegistryCredentialConfig,
		infra.NewRepository,
		application.NewCrypto,
		application.NewService,
		func(service *application.Service) releasedomain.RegistryCredentialResolver { return service },
	),
)

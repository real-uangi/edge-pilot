package registrycredential

import (
	"github.com/real-uangi/edge-pilot/internal/registrycredential/application"
	"github.com/real-uangi/edge-pilot/internal/registrycredential/infra"
	releasedomain "github.com/real-uangi/edge-pilot/internal/release/domain"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"registry-credential",
	fx.Provide(
		infra.NewRepository,
		application.NewCrypto,
		application.NewService,
		func(service *application.Service) releasedomain.RegistryCredentialResolver { return service },
	),
)

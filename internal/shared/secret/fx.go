package secret

import (
	"edge-pilot/internal/shared/config"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"shared-secret",
	fx.Provide(
		config.LoadServiceSecretConfig,
		NewCodec,
	),
)

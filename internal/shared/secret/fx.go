package secret

import (
	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"shared-secret",
	fx.Provide(
		NewCodec,
	),
)

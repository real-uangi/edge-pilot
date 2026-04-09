package release

import (
	"edge-pilot/internal/release/application"
	"edge-pilot/internal/release/infra"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"release",
	fx.Provide(
		infra.NewRepository,
		application.NewService,
	),
)

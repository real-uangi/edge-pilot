package observability

import (
	"edge-pilot/internal/observability/application"
	"edge-pilot/internal/observability/infra"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"observability",
	fx.Provide(
		infra.NewRepository,
		application.NewService,
	),
)

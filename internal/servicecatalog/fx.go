package servicecatalog

import (
	"edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/servicecatalog/infra"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"servicecatalog",
	fx.Provide(
		infra.NewRepository,
		application.NewService,
	),
)

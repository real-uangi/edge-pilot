package adminauth

import (
	"github.com/real-uangi/edge-pilot/internal/adminauth/application"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"admin-auth",
	fx.Provide(
		application.NewService,
	),
)

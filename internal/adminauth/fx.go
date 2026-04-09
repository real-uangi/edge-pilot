package adminauth

import (
	"edge-pilot/internal/adminauth/application"
	"edge-pilot/internal/shared/config"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"admin-auth",
	fx.Provide(
		config.LoadAdminAuthConfig,
		application.NewService,
	),
)

package controlplane

import (
	basehttp "github.com/real-uangi/edge-pilot/adapter/http"
	"github.com/real-uangi/edge-pilot/adapter/http/controlplane/routes"
	baseroutes "github.com/real-uangi/edge-pilot/adapter/http/routes"
	"github.com/real-uangi/edge-pilot/adapter/http/static"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"http-control-plane",
	fx.Invoke(
		basehttp.SetGlobalMiddleware,
		basehttp.ApplyProxyTrust,
		baseroutes.SetMetricsRoutes,
		routes.SetAuthRoutes,
		routes.SetAdminAgentRoutes,
		routes.SetAdminInstanceRoutes,
		routes.SetAdminRegistryCredentialRoutes,
		routes.SetAdminServiceRoutes,
		routes.SetAdminReleaseRoutes,
		routes.SetSchedulerRoutes,
		routes.SetObservabilityRoutes,
		routes.SetIntegrationRoutes,
		static.SetStaticWebHandler,
	),
)

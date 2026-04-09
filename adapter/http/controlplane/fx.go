package controlplane

import (
	basehttp "edge-pilot/adapter/http"
	"edge-pilot/adapter/http/controlplane/routes"
	baseroutes "edge-pilot/adapter/http/routes"
	"edge-pilot/adapter/http/static"

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
		routes.SetAdminServiceRoutes,
		routes.SetAdminReleaseRoutes,
		routes.SetObservabilityRoutes,
		routes.SetIntegrationRoutes,
		static.SetStaticWebHandler,
	),
)

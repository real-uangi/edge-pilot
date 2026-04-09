package agent

import (
	basehttp "edge-pilot/adapter/http"
	"edge-pilot/adapter/http/agent/routes"
	baseroutes "edge-pilot/adapter/http/routes"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"http-agent",
	fx.Invoke(
		basehttp.SetGlobalMiddleware,
		baseroutes.SetMetricsRoutes,
		routes.SetLocalRoutes,
	),
)

package agent

import (
	"edge-pilot/internal/agent/application"
	"edge-pilot/internal/agent/infra"
	"edge-pilot/internal/shared/config"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"agent-control-plane",
	fx.Provide(
		config.LoadAgentAuthConfig,
		infra.NewRepository,
		application.NewRegistryService,
	),
)

var RuntimeModule = fx.Module(
	"agent-runtime",
	fx.Provide(
		config.LoadAgentRuntimeConfig,
		infra.NewDockerClient,
		infra.NewHAProxyRuntimeClient,
		application.NewExecutor,
		application.NewRuntimeState,
	),
)

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
		infra.NewRawDockerClient,
		func(client *infra.DockerClient) application.DockerRuntime { return client },
		infra.NewManagedProxyRuntime,
		func(runtime *infra.ManagedProxyRuntime) application.ProxyRuntime { return runtime },
		application.NewExecutor,
		application.NewRuntimeState,
	),
	fx.Invoke(infra.StartManagedProxyRuntime),
)

package agent

import (
	"edge-pilot/internal/agent/application"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/agent/infra"
	"edge-pilot/internal/shared/config"

	"go.uber.org/fx"
)

type registryServiceDeps struct {
	fx.In

	Auth     *config.AgentAuthConfig
	Repo     agentdomain.Repository
	Bindings agentdomain.ServiceBindingChecker `optional:"true"`
}

func provideRegistryService(deps registryServiceDeps) *application.RegistryService {
	return application.NewRegistryServiceWithBindingChecker(deps.Auth, deps.Repo, deps.Bindings)
}

var ControlPlaneModule = fx.Module(
	"agent-control-plane",
	fx.Provide(
		config.LoadAgentAuthConfig,
		infra.NewRepository,
		provideRegistryService,
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

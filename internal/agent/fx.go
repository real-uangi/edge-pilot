package agent

import (
	"context"
	"edge-pilot/internal/agent/application"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/agent/infra"
	"edge-pilot/internal/shared/config"

	"github.com/real-uangi/allingo/common/log"
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

func startManagedContainerStartupReconcile(lc fx.Lifecycle, cfg *config.AgentRuntimeConfig, executor *application.Executor) {
	logger := log.NewStdLogger("agent.executor")
	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			stats, err := executor.ReconcileManagedContainersOnStartup(startCtx, cfg.AgentID)
			if err != nil {
				logger.Errorf(err, "startup managed container scan failed: agentId=%s scanned=%d removed=%d preserved=%d failed=%d", cfg.AgentID, stats.Scanned, stats.Removed, stats.Preserved, stats.Failed)
			}
			return nil
		},
	})
}

var ControlPlaneModule = fx.Module(
	"agent-control-plane",
	fx.Provide(
		config.LoadAgentAuthConfig,
		infra.NewRepository,
		provideRegistryService,
		application.NewHAProxyConfigService,
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
	fx.Invoke(
		infra.StartManagedProxyRuntime,
		startManagedContainerStartupReconcile,
	),
)

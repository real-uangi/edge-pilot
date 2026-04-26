package agent

import (
	"context"
	"edge-pilot/internal/agent/application/containerindex"
	"edge-pilot/internal/agent/application/proxyconfig"
	"edge-pilot/internal/agent/application/registry"
	"edge-pilot/internal/agent/application/taskexec"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/agent/infra/persistence"
	"edge-pilot/internal/agent/infra/runtime"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/perf"
	"time"

	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
)

type registryServiceDeps struct {
	fx.In

	Auth     *config.AgentAuthConfig
	Repo     agentdomain.Repository
	Bindings agentdomain.ServiceBindingChecker `optional:"true"`
}

func provideRegistryService(deps registryServiceDeps) *registry.RegistryService {
	return registry.NewRegistryServiceWithBindingChecker(deps.Auth, deps.Repo, deps.Bindings)
}

func provideProxyConfigAgentOnlineChecker(reg *registry.RegistryService) proxyconfig.AgentOnlineChecker {
	return reg
}

func startManagedContainerStartupReconcile(lc fx.Lifecycle, cfg *config.AgentRuntimeConfig, executor *taskexec.Executor) {
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

func startManagedContainerIndex(lc fx.Lifecycle, cfg *config.AgentRuntimeConfig, index *containerindex.ManagedContainerIndex) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			if err := index.RefreshNow(startCtx); err != nil {
				return err
			}
			go index.Run(ctx, time.Duration(cfg.ManagedContainerScanIntervalS)*time.Second)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

var ControlPlaneModule = fx.Module(
	"agent-control-plane",
	fx.Provide(
		persistence.NewRepository,
		provideRegistryService,
		provideProxyConfigAgentOnlineChecker,
		proxyconfig.NewHAProxyConfigService,
	),
)

var RuntimeModule = fx.Module(
	"agent-runtime",
	fx.Provide(
		config.LoadAgentRuntimeConfig,
		perf.NewCollector,
		runtime.NewRawDockerClient,
		func(client *runtime.DockerClient) agentdomain.DockerRuntime { return client },
		containerindex.NewManagedContainerIndex,
		runtime.NewManagedProxyRuntime,
		func(runtime *runtime.ManagedProxyRuntime) agentdomain.ProxyRuntime { return runtime },
		taskexec.NewExecutor,
		taskexec.NewRuntimeState,
	),
	fx.Invoke(
		runtime.StartManagedProxyRuntime,
		startManagedContainerIndex,
		startManagedContainerStartupReconcile,
	),
)

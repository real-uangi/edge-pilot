package observability

import (
	"edge-pilot/internal/agent/application/registry"
	"edge-pilot/internal/observability/application"
	"edge-pilot/internal/observability/infra"
	"edge-pilot/internal/shared/perf"

	"go.uber.org/fx"
)

var ControlPlaneModule = fx.Module(
	"observability",
	fx.Provide(
		infra.NewRepository,
		perf.NewCollector,
		func(reg *registry.RegistryService) application.AgentOverviewReader { return reg },
		application.NewService,
	),
	fx.Invoke(
		application.StartControlPlanePerformanceSampler,
	),
)

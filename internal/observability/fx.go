package observability

import (
	"github.com/real-uangi/edge-pilot/internal/agent/application/registry"
	"github.com/real-uangi/edge-pilot/internal/observability/application"
	"github.com/real-uangi/edge-pilot/internal/observability/infra"
	"github.com/real-uangi/edge-pilot/internal/shared/perf"

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

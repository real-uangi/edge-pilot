package controlplane

import (
	"edge-pilot/internal/agent/application/proxyconfig"
	releasedomain "edge-pilot/internal/release/domain"
	schedulerapp "edge-pilot/internal/scheduler/application"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"grpc-control-plane",
	fx.Provide(
		NewSessionHub,
		NewSchedulerSessionHub,
		func(hub *sessionHub) releasedomain.TaskDispatcher { return hub },
		func(hub *sessionHub) proxyconfig.HAProxyConfigRequester { return hub },
		func(hub *schedulerSessionHub) schedulerapp.RunDispatcher { return hub },
		NewProxyConfigPublisher,
		NewServer,
		NewSchedulerServer,
	),
	fx.Invoke(startGRPCServer),
)

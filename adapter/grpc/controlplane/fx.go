package controlplane

import (
	"github.com/real-uangi/edge-pilot/internal/agent/application/managedcontainer"
	"github.com/real-uangi/edge-pilot/internal/agent/application/proxyconfig"
	releasedomain "github.com/real-uangi/edge-pilot/internal/release/domain"
	schedulerapp "github.com/real-uangi/edge-pilot/internal/scheduler/application"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"grpc-control-plane",
	fx.Provide(
		NewSessionHub,
		NewSchedulerSessionHub,
		func(hub *sessionHub) releasedomain.TaskDispatcher { return hub },
		func(hub *sessionHub) proxyconfig.HAProxyConfigRequester { return hub },
		func(hub *sessionHub) managedcontainer.ContainerRuntimeRequester { return hub },
		func(hub *schedulerSessionHub) schedulerapp.RunDispatcher { return hub },
		NewProxyConfigPublisher,
		NewServer,
		NewSchedulerServer,
	),
	fx.Invoke(
		wireSchedulerRelay,
		startGRPCServer,
	),
)

func wireSchedulerRelay(server *Server, relay *SchedulerServer) {
	server.schedulerRelay = relay
}

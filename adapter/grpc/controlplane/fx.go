package controlplane

import (
	agentapp "edge-pilot/internal/agent/application"
	releasedomain "edge-pilot/internal/release/domain"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"grpc-control-plane",
	fx.Provide(
		NewSessionHub,
		func(hub *sessionHub) releasedomain.TaskDispatcher { return hub },
		func(hub *sessionHub) agentapp.HAProxyConfigRequester { return hub },
		NewProxyConfigPublisher,
		NewServer,
	),
	fx.Invoke(startGRPCServer),
)

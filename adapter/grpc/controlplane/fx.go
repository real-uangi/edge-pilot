package controlplane

import (
	"edge-pilot/internal/agent/application/proxyconfig"
	releasedomain "edge-pilot/internal/release/domain"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"grpc-control-plane",
	fx.Provide(
		NewSessionHub,
		func(hub *sessionHub) releasedomain.TaskDispatcher { return hub },
		func(hub *sessionHub) proxyconfig.HAProxyConfigRequester { return hub },
		NewProxyConfigPublisher,
		NewServer,
	),
	fx.Invoke(startGRPCServer),
)

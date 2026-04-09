package controlplane

import (
	releasedomain "edge-pilot/internal/release/domain"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"grpc-control-plane",
	fx.Provide(
		NewSessionHub,
		func(hub *sessionHub) releasedomain.TaskDispatcher { return hub },
		NewServer,
	),
	fx.Invoke(startGRPCServer),
)

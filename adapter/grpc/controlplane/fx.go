package controlplane

import (
	releasedomain "edge-pilot/internal/release/domain"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"grpc-control-plane",
	fx.Provide(
		fx.Annotate(
			NewSessionHub,
			fx.As(new(releasedomain.TaskDispatcher)),
		),
		NewServer,
	),
	fx.Invoke(startGRPCServer),
)

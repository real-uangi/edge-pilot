package agent

import "go.uber.org/fx"

var Module = fx.Module(
	"grpc-agent",
	fx.Provide(NewClient),
	fx.Invoke(startClient),
)

package bootstrap

import (
	agentgrpc "edge-pilot/adapter/grpc/agent"
	"edge-pilot/internal/agent"

	"github.com/real-uangi/allingo/common/app"
	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
)

func RunAgent() {
	logBuildInfo("agent")
	app.Current().Option(fx.WithLogger(log.NewFxLogger))
	app.Current().Option(agent.RuntimeModule)
	app.Current().Option(agentgrpc.Module)
	app.Current().Run()
}

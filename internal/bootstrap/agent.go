package bootstrap

import (
	agentgrpc "edge-pilot/adapter/grpc/agent"
	agenthttp "edge-pilot/adapter/http/agent"
	"edge-pilot/internal/agent"

	"github.com/real-uangi/allingo/common"
	"github.com/real-uangi/allingo/common/app"
	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
)

func RunAgent() {
	app.Current().Option(fx.WithLogger(log.NewFxLogger))
	app.Current().Option(common.Module)
	app.Current().Option(agent.RuntimeModule)
	app.Current().Option(agentgrpc.Module)
	app.Current().Option(agenthttp.Module)
	app.Current().Run()
}

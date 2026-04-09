package bootstrap

import (
	controlgrpc "edge-pilot/adapter/grpc/controlplane"
	controlhttp "edge-pilot/adapter/http/controlplane"
	"edge-pilot/adapter/schedule"
	"edge-pilot/internal/adminauth"
	"edge-pilot/internal/agent"
	"edge-pilot/internal/observability"
	"edge-pilot/internal/release"
	"edge-pilot/internal/servicecatalog"
	"edge-pilot/internal/shared/model"
	"edge-pilot/web"

	"github.com/real-uangi/allingo/common"
	"github.com/real-uangi/allingo/common/app"
	"github.com/real-uangi/allingo/common/db"
	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
)

func RunControlPlane() {
	logBuildInfo("control-plane")
	app.Current().Option(fx.WithLogger(log.NewFxLogger))
	app.Current().Option(common.Module)
	app.Current().Option(db.Module)
	app.Current().Option(model.ControlPlaneModule)
	app.Current().Option(servicecatalog.ControlPlaneModule)
	app.Current().Option(agent.ControlPlaneModule)
	app.Current().Option(adminauth.ControlPlaneModule)
	app.Current().Option(release.ControlPlaneModule)
	app.Current().Option(observability.ControlPlaneModule)
	app.Current().Option(controlgrpc.Module)
	app.Current().Option(controlhttp.Module)
	app.Current().Option(schedule.Module)
	app.Current().Option(web.Module)
	app.Current().Run()
}

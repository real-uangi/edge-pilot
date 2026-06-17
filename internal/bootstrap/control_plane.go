package bootstrap

import (
	controlgrpc "github.com/real-uangi/edge-pilot/adapter/grpc/controlplane"
	controlhttp "github.com/real-uangi/edge-pilot/adapter/http/controlplane"
	"github.com/real-uangi/edge-pilot/adapter/schedule"
	"github.com/real-uangi/edge-pilot/internal/adminauth"
	"github.com/real-uangi/edge-pilot/internal/agent"
	"github.com/real-uangi/edge-pilot/internal/observability"
	"github.com/real-uangi/edge-pilot/internal/registrycredential"
	"github.com/real-uangi/edge-pilot/internal/release"
	"github.com/real-uangi/edge-pilot/internal/scheduler"
	"github.com/real-uangi/edge-pilot/internal/servicecatalog"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/model"
	"github.com/real-uangi/edge-pilot/internal/shared/secret"
	"github.com/real-uangi/edge-pilot/web"

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
	app.Current().Option(config.ControlPlaneModule)
	app.Current().Option(model.ControlPlaneModule)
	app.Current().Option(secret.ControlPlaneModule)
	app.Current().Option(servicecatalog.ControlPlaneModule)
	app.Current().Option(agent.ControlPlaneModule)
	app.Current().Option(adminauth.ControlPlaneModule)
	app.Current().Option(registrycredential.ControlPlaneModule)
	app.Current().Option(release.ControlPlaneModule)
	app.Current().Option(scheduler.ControlPlaneModule)
	app.Current().Option(observability.ControlPlaneModule)
	app.Current().Option(controlgrpc.Module)
	app.Current().Option(controlhttp.Module)
	app.Current().Option(schedule.Module)
	app.Current().Option(web.Module)
	app.Current().Run()
}

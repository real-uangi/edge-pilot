package main

import (
	"edge-pilot/adapter/http"
	"edge-pilot/web"

	"github.com/real-uangi/allingo/common"
	"github.com/real-uangi/allingo/common/app"
	"github.com/real-uangi/allingo/common/db"
	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
)

func main() {
	app.Current().Option(fx.WithLogger(log.NewFxLogger))

	app.Current().Option(common.Module)
	app.Current().Option(db.Module)

	app.Current().Option(http.Module)
	app.Current().Option(web.Module)

	app.Current().Run()
}

/*
 * Copyright 2026 Uangi. All rights reserved.
 * @author uangi
 * @date 2026/1/22 11:28
 */

// Package static

package static

import (
	"edge-pilot/web"

	"github.com/gin-gonic/gin"
)

func SetStaticWebHandler(engine *gin.Engine, staticWebService *web.StaticWebService) {

	engine.Use(staticWebService.Serve)

	engine.NoRoute(staticWebService.NoRoute)

}

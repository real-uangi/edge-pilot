/*
 * Copyright 2026 Uangi. All rights reserved.
 * @author uangi
 * @date 2026/3/2 08:23
 */

// Package routes

package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/common/auth"
	"github.com/real-uangi/allingo/performance"
)

func SetMetricsRoutes(engine *gin.Engine) {

	engine.GET("/metrics", auth.InternalOnlyMiddleware, performance.Handler())

}

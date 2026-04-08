/*
 * Copyright 2026 Uangi. All rights reserved.
 * @author uangi
 * @date 2026/1/22 11:18
 */

// Package http

package http

import (
	"edge-pilot/adapter/http/middleware"
	"edge-pilot/adapter/http/routes"
	"edge-pilot/adapter/http/static"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/performance"
	"go.uber.org/fx"
)

var Module = fx.Module(
	"http-adapter",
	Infra,

	fx.Invoke(
	// app routes

	),

	// This MUST be the last option: fx.Invoke(static.SetStaticWebHandler)
	fx.Invoke(static.SetStaticWebHandler),
)

var Infra = fx.Module(
	"http-adapter-infra",
	fx.Invoke(
		//性能指标
		routes.SetMetricsRoutes,
		//全局http中间件
		setGlobalMiddleware,
	),
)

func setGlobalMiddleware(engine *gin.Engine) {
	//http性能监测
	engine.Use(performance.GinHttpMiddleware)
	//压缩
	engine.Use(gzip.Gzip(gzip.DefaultCompression))
	//缓存策略
	engine.Use(middleware.AssignCache)
}

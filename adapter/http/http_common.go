package http

import (
	"edge-pilot/adapter/http/middleware"
	"edge-pilot/internal/shared/config"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/performance"
)

func SetGlobalMiddleware(engine *gin.Engine) {
	engine.Use(performance.GinHttpMiddleware)
	engine.Use(gzip.Gzip(gzip.DefaultCompression))
	engine.Use(middleware.AssignCache)
}

func ApplyProxyTrust(engine *gin.Engine, cfg *config.AdminAuthConfig) error {
	return middleware.ApplyProxyTrust(engine, cfg)
}

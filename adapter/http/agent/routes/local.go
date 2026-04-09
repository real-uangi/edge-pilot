package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/common/auth"
	"github.com/real-uangi/allingo/common/result"
)

func SetLocalRoutes(engine *gin.Engine) {
	engine.GET("/healthz", auth.InternalOnlyMiddleware, func(c *gin.Context) {
		c.Render(http.StatusOK, result.Ok(gin.H{"status": "ok"}))
	})
}

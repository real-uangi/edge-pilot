package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

func SetAuthRoutes(engine *gin.Engine, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	engine.POST("/api/auth/login", func(c *gin.Context) {
		var input dto.AdminLoginRequest
		if err := c.BindJSON(&input); err != nil {
			c.Render(http.StatusBadRequest, result.BadRequest(err))
			return
		}
		token, output, err := auth.Login(input)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		adaptermiddleware.SetSessionCookie(c, cfg, token, output.ExpiresAt)
		c.Render(http.StatusOK, result.Ok(output))
	})

	engine.POST("/api/auth/logout", func(c *gin.Context) {
		adaptermiddleware.ClearSessionCookie(c, cfg)
		c.Render(http.StatusOK, result.Ok(gin.H{"ok": true}))
	})

	me := engine.Group("/api/auth")
	me.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	me.GET("/me", func(c *gin.Context) {
		claims, ok := adaptermiddleware.CurrentAdminSession(c)
		if !ok {
			c.Render(http.StatusUnauthorized, result.Custom[any](http.StatusUnauthorized, "Unauthorized", nil))
			return
		}
		c.Render(http.StatusOK, result.Ok(dto.AdminSessionOutput{
			Username:  claims.Username,
			ExpiresAt: claims.ExpiresAt,
		}))
	})
}

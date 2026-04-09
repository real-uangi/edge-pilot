package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	observabilityapp "edge-pilot/internal/observability/application"
	"edge-pilot/internal/shared/config"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

func SetObservabilityRoutes(engine *gin.Engine, service *observabilityapp.Service, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	admin.GET("/overview", func(c *gin.Context) {
		output, err := service.GetOverview()
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.GET("/services/:id/observability", api.SingleParamUUIDFunc(func(id uuid.UUID) (interface{}, error) {
		return service.GetServiceObservability(id)
	}, "id"))
}

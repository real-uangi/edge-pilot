package routes

import (
	observabilityapp "edge-pilot/internal/observability/application"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/auth"
	"github.com/real-uangi/allingo/common/result"
)

func SetObservabilityRoutes(engine *gin.Engine, service *observabilityapp.Service) {
	admin := engine.Group("/api/admin")
	admin.Use(auth.InternalOnlyMiddleware)
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

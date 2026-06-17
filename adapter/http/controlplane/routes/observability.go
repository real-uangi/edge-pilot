package routes

import (
	"net/http"

	adaptermiddleware "github.com/real-uangi/edge-pilot/adapter/http/middleware"
	adminauthapp "github.com/real-uangi/edge-pilot/internal/adminauth/application"
	observabilityapp "github.com/real-uangi/edge-pilot/internal/observability/application"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

type observabilityAdminActions interface {
	GetOverview() (*dto.OverviewOutput, error)
	GetServiceObservability(uuid.UUID) (*dto.ObservabilityOutput, error)
	GetSystemPerformanceOverview() (*dto.SystemPerformanceOverviewOutput, error)
	GetAgentPerformanceHistory(string) (*dto.AgentPerformanceHistoryOutput, error)
}

func SetObservabilityRoutes(engine *gin.Engine, service *observabilityapp.Service, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	registerObservabilityRoutes(admin, service)
}

func registerObservabilityRoutes(admin *gin.RouterGroup, service observabilityAdminActions) {
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
	admin.GET("/system/performance", func(c *gin.Context) {
		output, err := service.GetSystemPerformanceOverview()
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.GET("/system/performance/agents/:id/history", api.SingleParamUUIDFunc(func(id uuid.UUID) (interface{}, error) {
		return service.GetAgentPerformanceHistory(id.String())
	}, "id"))
}

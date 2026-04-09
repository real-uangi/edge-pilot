package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

func SetAdminServiceRoutes(engine *gin.Engine, services *servicecatalogapp.Service, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	admin.POST("/services", api.JsonFunc(func(input dto.UpsertServiceRequest) (*dto.ServiceOutput, error) {
		return services.Create(input)
	}))
	admin.PUT("/services/:id", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		var input dto.UpsertServiceRequest
		if err := c.BindJSON(&input); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := services.Update(id, input)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.GET("/services", api.NoArgsFunc(func() ([]dto.ServiceOutput, error) {
		return services.List()
	}))
	admin.GET("/services/:id", api.SingleParamUUIDFunc(func(id uuid.UUID) (*dto.ServiceOutput, error) {
		return services.Get(id)
	}, "id"))
}

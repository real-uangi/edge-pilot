package routes

import (
	"net/http"

	adaptermiddleware "github.com/real-uangi/edge-pilot/adapter/http/middleware"
	adminauthapp "github.com/real-uangi/edge-pilot/internal/adminauth/application"
	servicecatalogapp "github.com/real-uangi/edge-pilot/internal/servicecatalog/application"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

type serviceAdminActions interface {
	Create(dto.UpsertServiceRequest) (*dto.ServiceOutput, error)
	Update(uuid.UUID, dto.UpsertServiceRequest) (*dto.ServiceOutput, error)
	Delete(uuid.UUID) error
	List() ([]dto.ServiceOutput, error)
	Get(uuid.UUID) (*dto.ServiceOutput, error)
}

func SetAdminServiceRoutes(engine *gin.Engine, services *servicecatalogapp.Service, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	registerAdminServiceRoutes(admin, services)
}

func registerAdminServiceRoutes(admin *gin.RouterGroup, services serviceAdminActions) {
	admin.POST("/services", api.JsonFunc(func(input dto.UpsertServiceRequest) (*dto.ServiceOutput, error) {
		return services.Create(input)
	}))
	admin.DELETE("/services/:id", api.SingleParamUUIDFunc(func(id uuid.UUID) (*gin.H, error) {
		if err := services.Delete(id); err != nil {
			return nil, err
		}
		output := gin.H{"deleted": true}
		return &output, nil
	}, "id"))
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

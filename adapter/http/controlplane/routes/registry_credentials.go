package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	registrycredentialapp "edge-pilot/internal/registrycredential/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

type registryCredentialAdminActions interface {
	Create(dto.UpsertRegistryCredentialRequest) (*dto.RegistryCredentialOutput, error)
	Update(uuid.UUID, dto.UpsertRegistryCredentialRequest) (*dto.RegistryCredentialOutput, error)
	Delete(uuid.UUID) error
	Get(uuid.UUID) (*dto.RegistryCredentialOutput, error)
	List() ([]dto.RegistryCredentialOutput, error)
}

func SetAdminRegistryCredentialRoutes(engine *gin.Engine, credentials *registrycredentialapp.Service, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	registerAdminRegistryCredentialRoutes(admin, credentials)
}

func registerAdminRegistryCredentialRoutes(admin *gin.RouterGroup, credentials registryCredentialAdminActions) {
	admin.POST("/registry-credentials", api.JsonFunc(func(input dto.UpsertRegistryCredentialRequest) (*dto.RegistryCredentialOutput, error) {
		return credentials.Create(input)
	}))
	admin.PUT("/registry-credentials/:id", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		var input dto.UpsertRegistryCredentialRequest
		if err := c.BindJSON(&input); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := credentials.Update(id, input)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.DELETE("/registry-credentials/:id", api.SingleParamUUIDFunc(func(id uuid.UUID) (*gin.H, error) {
		if err := credentials.Delete(id); err != nil {
			return nil, err
		}
		output := gin.H{"deleted": true}
		return &output, nil
	}, "id"))
	admin.GET("/registry-credentials", api.NoArgsFunc(func() ([]dto.RegistryCredentialOutput, error) {
		return credentials.List()
	}))
	admin.GET("/registry-credentials/:id", api.SingleParamUUIDFunc(func(id uuid.UUID) (*dto.RegistryCredentialOutput, error) {
		return credentials.Get(id)
	}, "id"))
}

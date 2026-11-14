package routes

import (
	"context"

	adaptermiddleware "github.com/real-uangi/edge-pilot/adapter/http/middleware"
	adminauthapp "github.com/real-uangi/edge-pilot/internal/adminauth/application"
	servicecatalogapp "github.com/real-uangi/edge-pilot/internal/servicecatalog/application"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"

	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/common/api"
)

type serviceImageTagActions interface {
	List(context.Context, dto.ListServiceImageTagsRequest) (*dto.ServiceImageTagsOutput, error)
}

func SetAdminServiceImageTagRoutes(engine *gin.Engine, tags *servicecatalogapp.ImageTagService, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	registerAdminServiceImageTagRoutes(admin, tags)
}

func registerAdminServiceImageTagRoutes(admin *gin.RouterGroup, tags serviceImageTagActions) {
	admin.POST("/services/image-tags", func(c *gin.Context) {
		api.JsonFunc(func(input dto.ListServiceImageTagsRequest) (*dto.ServiceImageTagsOutput, error) {
			return tags.List(c.Request.Context(), input)
		})(c)
	})
}

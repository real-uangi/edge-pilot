package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	releaseapp "edge-pilot/internal/release/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

type releaseAdminActions interface {
	List() ([]dto.ReleaseOutput, error)
	Get(id uuid.UUID) (*dto.ReleaseOutput, error)
	ListTaskSnapshots(releaseID uuid.UUID) ([]dto.TaskSnapshot, error)
	Start(id uuid.UUID, operator string) (*dto.ReleaseOutput, error)
	Skip(id uuid.UUID, operator string) (*dto.ReleaseOutput, error)
	ConfirmSwitch(id uuid.UUID, operator string) (*dto.ReleaseOutput, error)
	Rollback(id uuid.UUID, operator string) (*dto.ReleaseOutput, error)
}

func SetAdminReleaseRoutes(engine *gin.Engine, releases *releaseapp.Service, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	registerAdminReleaseRoutes(admin, releases)
}

func registerAdminReleaseRoutes(admin *gin.RouterGroup, releases releaseAdminActions) {
	admin.GET("/releases", api.NoArgsFunc(func() ([]dto.ReleaseOutput, error) {
		return releases.List()
	}))
	admin.GET("/releases/:id", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		releaseOutput, err := releases.Get(id)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		tasks, err := releases.ListTaskSnapshots(id)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(gin.H{
			"release": releaseOutput,
			"tasks":   tasks,
		}))
	})
	admin.POST("/releases/:id/start", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		var input dto.StartReleaseRequest
		if err := c.BindJSON(&input); err != nil && err.Error() != "EOF" {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := releases.Start(id, adaptermiddleware.CurrentAdminUsername(c))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.POST("/releases/:id/skip", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		var input dto.SkipReleaseRequest
		if err := c.BindJSON(&input); err != nil && err.Error() != "EOF" {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := releases.Skip(id, adaptermiddleware.CurrentAdminUsername(c))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.POST("/releases/:id/confirm-switch", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		var input dto.ConfirmSwitchRequest
		if err := c.BindJSON(&input); err != nil && err.Error() != "EOF" {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := releases.ConfirmSwitch(id, adaptermiddleware.CurrentAdminUsername(c))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.POST("/releases/:id/rollback", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		var input dto.RollbackRequest
		if err := c.BindJSON(&input); err != nil && err.Error() != "EOF" {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := releases.Rollback(id, adaptermiddleware.CurrentAdminUsername(c))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
}

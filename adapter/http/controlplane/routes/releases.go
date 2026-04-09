package routes

import (
	releaseapp "edge-pilot/internal/release/application"
	"edge-pilot/internal/shared/dto"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/auth"
	"github.com/real-uangi/allingo/common/result"
)

func SetAdminReleaseRoutes(engine *gin.Engine, releases *releaseapp.Service) {
	admin := engine.Group("/api/admin")
	admin.Use(auth.InternalOnlyMiddleware)
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
		output, err := releases.ConfirmSwitch(id, input.Operator)
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
		output, err := releases.Rollback(id, input.Operator)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
}

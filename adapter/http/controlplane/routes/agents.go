package routes

import (
	agentapp "edge-pilot/internal/agent/application"
	"edge-pilot/internal/shared/dto"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/auth"
	"github.com/real-uangi/allingo/common/result"
)

func SetAdminAgentRoutes(engine *gin.Engine, agents *agentapp.RegistryService) {
	admin := engine.Group("/api/admin")
	admin.Use(auth.InternalOnlyMiddleware)
	admin.POST("/agents", func(c *gin.Context) {
		output, err := agents.CreateAgent()
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.GET("/agents", api.NoArgsFunc(func() ([]dto.AgentOutput, error) {
		return agents.ListAgents()
	}))
	admin.GET("/agents/:id", func(c *gin.Context) {
		id := c.Param("id")
		if _, err := uuid.Parse(id); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := agents.GetAgent(id)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.POST("/agents/:id/reset-token", func(c *gin.Context) {
		id := c.Param("id")
		if _, err := uuid.Parse(id); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := agents.ResetToken(id)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.POST("/agents/:id/enable", func(c *gin.Context) {
		id := c.Param("id")
		if _, err := uuid.Parse(id); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := agents.Enable(id)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
	admin.POST("/agents/:id/disable", func(c *gin.Context) {
		id := c.Param("id")
		if _, err := uuid.Parse(id); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := agents.Disable(id)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
}

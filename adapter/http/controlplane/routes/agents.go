package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	"edge-pilot/internal/agent/application/proxyconfig"
	"edge-pilot/internal/agent/application/registry"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

type agentAdminActions interface {
	CreateAgent() (*dto.AgentCredentialOutput, error)
	ListAgents() ([]dto.AgentOutput, error)
	GetAgent(string) (*dto.AgentOutput, error)
	ResetToken(string) (*dto.AgentCredentialOutput, error)
	Enable(string) (*dto.AgentOutput, error)
	Disable(string) (*dto.AgentOutput, error)
	Delete(string) error
}

type agentHAProxyConfigActions interface {
	GetHAProxyConfig(string) (*dto.AgentHAProxyConfigOutput, error)
}

func SetAdminAgentRoutes(engine *gin.Engine, agents *registry.RegistryService, haproxyConfigs *proxyconfig.HAProxyConfigService, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	registerAdminAgentRoutes(admin, agents, haproxyConfigs)
}

func registerAdminAgentRoutes(admin *gin.RouterGroup, agents agentAdminActions, haproxyConfigs agentHAProxyConfigActions) {
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
	admin.GET("/agents/:id", api.SingleParamUUIDFunc(func(id uuid.UUID) (*dto.AgentOutput, error) {
		return agents.GetAgent(id.String())
	}, "id"))
	admin.GET("/agents/:id/haproxy-config", api.SingleParamUUIDFunc(func(id uuid.UUID) (*dto.AgentHAProxyConfigOutput, error) {
		return haproxyConfigs.GetHAProxyConfig(id.String())
	}, "id"))
	admin.POST("/agents/:id/reset-token", api.SingleParamUUIDFunc(func(id uuid.UUID) (*dto.AgentCredentialOutput, error) {
		return agents.ResetToken(id.String())
	}, "id"))
	admin.POST("/agents/:id/enable", api.SingleParamUUIDFunc(func(id uuid.UUID) (*dto.AgentOutput, error) {
		return agents.Enable(id.String())
	}, "id"))
	admin.POST("/agents/:id/disable", api.SingleParamUUIDFunc(func(id uuid.UUID) (*dto.AgentOutput, error) {
		return agents.Disable(id.String())
	}, "id"))
	admin.DELETE("/agents/:id", api.SingleParamUUIDFunc(func(id uuid.UUID) (*gin.H, error) {
		if err := agents.Delete(id.String()); err != nil {
			return nil, err
		}
		output := gin.H{"deleted": true}
		return &output, nil
	}, "id"))
}

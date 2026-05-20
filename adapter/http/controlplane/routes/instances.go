package routes

import (
	"context"
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	"edge-pilot/internal/agent/application/managedcontainer"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/log"
	"github.com/real-uangi/allingo/common/result"
)

var instancesLogger = log.NewStdLogger("http.instances")

type instanceAdminActions interface {
	ListContainers() ([]dto.ManagedInstanceOutput, error)
	GetContainerDetails(agentID, containerID string) (*dto.ManagedInstanceDetailsOutput, error)
	StreamContainerLogs(ctx context.Context, agentID, containerID string, writer func(data string, stderr bool) error) error
}

func SetAdminInstanceRoutes(engine *gin.Engine, instances *managedcontainer.ManagedContainerService, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	registerAdminInstanceRoutes(admin, instances)
}

func registerAdminInstanceRoutes(admin *gin.RouterGroup, instances instanceAdminActions) {
	admin.GET("/instances", api.NoArgsFunc(func() ([]dto.ManagedInstanceOutput, error) {
		return instances.ListContainers()
	}))

	admin.GET("/instances/:agentId/:containerId", func(c *gin.Context) {
		agentID := c.Param("agentId")
		containerID := c.Param("containerId")
		output, err := instances.GetContainerDetails(agentID, containerID)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})

	admin.GET("/instances/:agentId/:containerId/logs/stream", func(c *gin.Context) {
		agentID := c.Param("agentId")
		containerID := c.Param("containerId")

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.String(http.StatusInternalServerError, "streaming unsupported")
			return
		}

		ctx := c.Request.Context()

		err := instances.StreamContainerLogs(ctx, agentID, containerID, func(data string, stderr bool) error {
			chunk := map[string]interface{}{
				"data":   data,
				"stderr": stderr,
				"time":   time.Now().Unix(),
			}
			jsonData, err := json.Marshal(chunk)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(c.Writer, "data: %s\n\n", jsonData)
			if err != nil {
				return err
			}
			flusher.Flush()
			return nil
		})

		if err != nil && err != context.Canceled {
			instancesLogger.Errorf(err, "sse log stream error: agentId=%s containerId=%s", agentID, containerID)
		}
	})
}

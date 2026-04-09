package routes

import (
	releaseapp "edge-pilot/internal/release/application"
	"edge-pilot/internal/shared/dto"
	"os"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

func SetIntegrationRoutes(engine *gin.Engine, releases *releaseapp.Service) {
	engine.POST("/api/integration/ci/releases", func(c *gin.Context) {
		sharedToken := os.Getenv("CI_SHARED_TOKEN")
		if sharedToken != "" && c.GetHeader("X-EdgePilot-Token") != sharedToken {
			c.Render(http.StatusUnauthorized, result.Custom[any](http.StatusUnauthorized, "Unauthorized", nil))
			return
		}
		var input dto.CreateReleaseFromCIRequest
		if err := c.BindJSON(&input); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		output, err := releases.CreateFromCI(input)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(output))
	})
}

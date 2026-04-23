package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	schedulerapp "edge-pilot/internal/scheduler/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/api"
	"github.com/real-uangi/allingo/common/result"
)

type schedulerAdminActions interface {
	CreateJob(req dto.UpsertSchedulerJobRequest) (*dto.SchedulerJobOutput, error)
	UpdateJob(id uuid.UUID, req dto.UpsertSchedulerJobRequest) (*dto.SchedulerJobOutput, error)
	GetJob(id uuid.UUID) (*dto.SchedulerJobOutput, error)
	ListJobs() ([]dto.SchedulerJobOutput, error)
	DeleteJob(id uuid.UUID) error
	SetJobEnabled(id uuid.UUID, enabled bool) (*dto.SchedulerJobOutput, error)
	TriggerNow(id uuid.UUID, override map[string]any) (*dto.SchedulerRunOutput, error)
	ListRuns(jobID uuid.UUID, limit int) ([]dto.SchedulerRunOutput, error)
	CreateExecutor(req dto.UpsertSchedulerExecutorRequest) (*dto.SchedulerExecutorOutput, error)
	ResetExecutorToken(executorID string) (*dto.SchedulerExecutorOutput, error)
	SetExecutorEnabled(executorID string, enabled bool) (*dto.SchedulerExecutorOutput, error)
	ListExecutors() ([]dto.SchedulerExecutorOutput, error)
	DeleteExecutor(executorID string) error
}

func SetSchedulerRoutes(engine *gin.Engine, scheduler *schedulerapp.Service, auth *adminauthapp.Service, cfg *config.AdminAuthConfig) {
	admin := engine.Group("/api/admin/scheduler")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	registerSchedulerRoutes(admin, scheduler)
}

func registerSchedulerRoutes(admin *gin.RouterGroup, scheduler schedulerAdminActions) {
	admin.GET("/jobs", api.NoArgsFunc(func() ([]dto.SchedulerJobOutput, error) {
		return scheduler.ListJobs()
	}))
	admin.POST("/jobs", api.JsonFunc(func(input dto.UpsertSchedulerJobRequest) (*dto.SchedulerJobOutput, error) {
		return scheduler.CreateJob(input)
	}))
	admin.GET("/jobs/:id", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		out, err := scheduler.GetJob(id)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})
	admin.PUT("/jobs/:id", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		var input dto.UpsertSchedulerJobRequest
		if err := c.BindJSON(&input); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		out, err := scheduler.UpdateJob(id, input)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})
	admin.DELETE("/jobs/:id", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		if err := scheduler.DeleteJob(id); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(gin.H{"deleted": true}))
	})
	admin.POST("/jobs/:id/enable", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		out, err := scheduler.SetJobEnabled(id, true)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})
	admin.POST("/jobs/:id/disable", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		out, err := scheduler.SetJobEnabled(id, false)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})
	admin.POST("/jobs/:id/trigger", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		var input dto.TriggerSchedulerJobRequest
		if err := c.BindJSON(&input); err != nil && err.Error() != "EOF" {
			c.Render(api.HandleErr(err))
			return
		}
		out, err := scheduler.TriggerNow(id, input.OverridePayload)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})
	admin.GET("/jobs/:id/runs", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		limit := 50
		if raw := c.Query("limit"); raw != "" {
			if v, convErr := strconv.Atoi(raw); convErr == nil && v > 0 && v <= 200 {
				limit = v
			}
		}
		out, err := scheduler.ListRuns(id, limit)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})

	admin.GET("/executors", api.NoArgsFunc(func() ([]dto.SchedulerExecutorOutput, error) {
		return scheduler.ListExecutors()
	}))
	admin.POST("/executors", api.JsonFunc(func(input dto.UpsertSchedulerExecutorRequest) (*dto.SchedulerExecutorOutput, error) {
		return scheduler.CreateExecutor(input)
	}))
	admin.POST("/executors/:id/reset-token", func(c *gin.Context) {
		out, err := scheduler.ResetExecutorToken(c.Param("id"))
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})
	admin.POST("/executors/:id/enable", func(c *gin.Context) {
		out, err := scheduler.SetExecutorEnabled(c.Param("id"), true)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})
	admin.POST("/executors/:id/disable", func(c *gin.Context) {
		out, err := scheduler.SetExecutorEnabled(c.Param("id"), false)
		if err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(out))
	})
	admin.DELETE("/executors/:id", func(c *gin.Context) {
		if err := scheduler.DeleteExecutor(c.Param("id")); err != nil {
			c.Render(api.HandleErr(err))
			return
		}
		c.Render(http.StatusOK, result.Ok(gin.H{"deleted": true}))
	})
}

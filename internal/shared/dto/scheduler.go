package dto

import (
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"github.com/google/uuid"
)

type UpsertSchedulerJobRequest struct {
	Name            string            `json:"name" binding:"required"`
	HandlerKey      string            `json:"handlerKey" binding:"required"`
	ServiceID       uuid.UUID         `json:"serviceId" binding:"required"`
	Payload         map[string]any    `json:"payload"`
	ScheduleKind    string            `json:"scheduleKind" binding:"required,oneof=one_time cron"`
	CronExpr        string            `json:"cronExpr"`
	RunAt           *time.Time        `json:"runAt"`
	Enabled         *bool             `json:"enabled"`
	DispatchPolicy  string            `json:"dispatchPolicy"`
	ExecutorGroup   string            `json:"executorGroup" binding:"required"`
	LeaseTimeoutSec int               `json:"leaseTimeoutSec"`
	MaxRetries      int               `json:"maxRetries"`
	Metadata        map[string]string `json:"metadata"`
}

type SchedulerJobOutput struct {
	ID              uuid.UUID                     `json:"id"`
	Name            string                        `json:"name"`
	HandlerKey      string                        `json:"handlerKey"`
	ServiceID       uuid.UUID                     `json:"serviceId"`
	Payload         map[string]any                `json:"payload"`
	ScheduleKind    model.SchedulerScheduleKind   `json:"scheduleKind"`
	CronExpr        string                        `json:"cronExpr"`
	RunAt           *time.Time                    `json:"runAt"`
	NextRunAt       *time.Time                    `json:"nextRunAt"`
	Enabled         *bool                         `json:"enabled"`
	DispatchPolicy  model.SchedulerDispatchPolicy `json:"dispatchPolicy"`
	ExecutorGroup   string                        `json:"executorGroup"`
	LeaseTimeoutSec int                           `json:"leaseTimeoutSec"`
	MaxRetries      int                           `json:"maxRetries"`
	Metadata        map[string]string             `json:"metadata"`
	CreatedAt       time.Time                     `json:"createdAt"`
	UpdatedAt       time.Time                     `json:"updatedAt"`
}

type SchedulerRunOutput struct {
	ID             uuid.UUID                   `json:"id"`
	JobID          uuid.UUID                   `json:"jobId"`
	HandlerKey     string                      `json:"handlerKey"`
	ServiceID      uuid.UUID                   `json:"serviceId"`
	Payload        map[string]any              `json:"payload"`
	Status         model.SchedulerJobRunStatus `json:"status"`
	Attempt        int                         `json:"attempt"`
	MaxRetries     int                         `json:"maxRetries"`
	ScheduledAt    time.Time                   `json:"scheduledAt"`
	DispatchedAt   *time.Time                  `json:"dispatchedAt"`
	StartedAt      *time.Time                  `json:"startedAt"`
	CompletedAt    *time.Time                  `json:"completedAt"`
	LeaseExpiresAt *time.Time                  `json:"leaseExpiresAt"`
	LeasedBy       string                      `json:"leasedBy"`
	NextRetryAt    *time.Time                  `json:"nextRetryAt"`
	ErrorMessage   string                      `json:"errorMessage"`
	CreatedAt      time.Time                   `json:"createdAt"`
	UpdatedAt      time.Time                   `json:"updatedAt"`
}

type SchedulerExecutorOutput struct {
	ID              string                             `json:"id"`
	Group           string                             `json:"group"`
	ChannelMode     model.SchedulerExecutorChannelMode `json:"channelMode"`
	RelayAgentID    string                             `json:"relayAgentId"`
	RelayRoutingKey string                             `json:"relayRoutingKey"`
	Enabled         *bool                              `json:"enabled"`
	LastSeenAt      *time.Time                         `json:"lastSeenAt"`
	LiveSlot        model.Slot                         `json:"liveSlot"`
	InstanceMeta    map[string]string                  `json:"instanceMeta"`
	Token           string                             `json:"token,omitempty"`
	CreatedAt       time.Time                          `json:"createdAt"`
	UpdatedAt       time.Time                          `json:"updatedAt"`
}

type UpsertSchedulerExecutorRequest struct {
	ExecutorID string            `json:"executorId" binding:"required"`
	Group      string            `json:"group" binding:"required"`
	Enabled    *bool             `json:"enabled"`
	LiveSlot   int               `json:"liveSlot"`
	Metadata   map[string]string `json:"metadata"`
}

type TriggerSchedulerJobRequest struct {
	OverridePayload map[string]any `json:"overridePayload"`
}

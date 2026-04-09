package dto

import (
	"edge-pilot/internal/shared/model"
	"time"

	"github.com/google/uuid"
)

type CreateReleaseFromCIRequest struct {
	ServiceKey  string `json:"serviceKey" binding:"required"`
	ImageRepo   string `json:"imageRepo"`
	ImageTag    string `json:"imageTag" binding:"required"`
	CommitSHA   string `json:"commitSha"`
	TriggeredBy string `json:"triggeredBy"`
	TraceID     string `json:"traceId"`
}

type ReleaseOutput struct {
	ID               uuid.UUID           `json:"id"`
	ServiceID        uuid.UUID           `json:"serviceId"`
	AgentID          string              `json:"agentId"`
	ImageRepo        string              `json:"imageRepo"`
	ImageTag         string              `json:"imageTag"`
	CommitSHA        string              `json:"commitSha"`
	TriggeredBy      string              `json:"triggeredBy"`
	TraceID          string              `json:"traceId"`
	Status           model.ReleaseStatus `json:"status"`
	TargetSlot       model.Slot          `json:"targetSlot"`
	PreviousLiveSlot model.Slot          `json:"previousLiveSlot"`
	CurrentTaskID    *uuid.UUID          `json:"currentTaskId"`
	SwitchConfirmed  *bool               `json:"switchConfirmed"`
	IsActive         bool                `json:"isActive"`
	QueuePosition    int                 `json:"queuePosition"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
	CompletedAt      *time.Time          `json:"completedAt"`
}

type StartReleaseRequest struct {
	Operator string `json:"operator"`
}

type SkipReleaseRequest struct {
	Operator string `json:"operator"`
}

type ConfirmSwitchRequest struct {
	Operator string `json:"operator"`
}

type RollbackRequest struct {
	Operator string `json:"operator"`
}

type TaskSnapshot struct {
	ID           uuid.UUID        `json:"id"`
	Type         model.TaskType   `json:"type"`
	Status       model.TaskStatus `json:"status"`
	LastError    string           `json:"lastError"`
	DispatchedAt *time.Time       `json:"dispatchedAt"`
	StartedAt    *time.Time       `json:"startedAt"`
	CompletedAt  *time.Time       `json:"completedAt"`
}

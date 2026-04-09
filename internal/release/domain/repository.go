package domain

import (
	"edge-pilot/internal/shared/model"

	"github.com/google/uuid"
)

type Repository interface {
	CreateRelease(*model.Release) error
	UpdateRelease(*model.Release) error
	GetRelease(uuid.UUID) (*model.Release, error)
	ListReleases(int) ([]model.Release, error)
	HasActiveRelease(uuid.UUID) (bool, error)
	CreateTask(*model.Task) error
	UpdateTask(*model.Task) error
	GetTask(uuid.UUID) (*model.Task, error)
	ListTasksByRelease(uuid.UUID) ([]model.Task, error)
	CreateTaskAttempt(*model.TaskAttempt) error
	UpsertRuntimeInstance(*model.RuntimeInstance) error
	GetRuntimeInstanceByServiceAndSlot(uuid.UUID, model.Slot) (*model.RuntimeInstance, error)
	ListRuntimeInstancesByService(uuid.UUID) ([]model.RuntimeInstance, error)
	CreateAudit(*model.AuditLog) error
}

type TaskDispatcher interface {
	DispatchTask(agentID string, task *model.Task) error
}

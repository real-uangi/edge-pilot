package domain

import (
	"edge-pilot/internal/shared/model"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	CreateRelease(*model.Release) error
	UpdateRelease(*model.Release) error
	GetRelease(uuid.UUID) (*model.Release, error)
	ListReleases(int) ([]model.Release, error)
	HasActiveRelease(uuid.UUID) (bool, error)
	FindQueuedOrActiveDuplicate(uuid.UUID, string, string) (*model.Release, error)
	CountQueuedBefore(uuid.UUID, time.Time, uuid.UUID) (int, error)
	CreateTask(*model.Task) error
	UpdateTask(*model.Task) error
	GetTask(uuid.UUID) (*model.Task, error)
	ListTasksByRelease(uuid.UUID) ([]model.Task, error)
	ListRecoverableTasksByAgent(string) ([]model.Task, error)
	ListStaleTasks(time.Time) ([]model.Task, error)
	CreateTaskAttempt(*model.TaskAttempt) error
	UpsertRuntimeInstance(*model.RuntimeInstance) error
	GetRuntimeInstanceByServiceAndSlot(uuid.UUID, model.Slot) (*model.RuntimeInstance, error)
	ListRuntimeInstancesByService(uuid.UUID) ([]model.RuntimeInstance, error)
	CreateAudit(*model.AuditLog) error
}

type TaskDispatcher interface {
	DispatchTask(agentID string, task *model.Task) error
	ReplayTask(agentID string, task *model.Task) (bool, error)
}

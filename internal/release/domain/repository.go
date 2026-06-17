package domain

import (
	"edge-pilot/internal/shared/model"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	CreateRelease(*model.Release) error
	UpdateRelease(*model.Release) error
	GetRelease(uuid.UUID) (*model.Release, error)
	ListReleases(int) ([]model.Release, error)
	ListQueuedBefore(uuid.UUID, time.Time, uuid.UUID) ([]model.Release, error)
	FindReadyToSwitchRelease(uuid.UUID) (*model.Release, error)
	HasActiveRelease(uuid.UUID) (bool, error)
	HasTrafficSplitRelease(uuid.UUID) (bool, error)
	HasNewerSuccessfulRelease(serviceID uuid.UUID, createdAt time.Time) (bool, error)
	FindQueuedOrActiveDuplicate(uuid.UUID, string, string) (*model.Release, error)
	CountQueuedBefore(uuid.UUID, time.Time, uuid.UUID) (int, error)
	CreateTask(*model.Task) error
	UpdateTask(*model.Task) error
	GetTask(uuid.UUID) (*model.Task, error)
	ListTasksByRelease(uuid.UUID) ([]model.Task, error)
	ListRecoverableTasksByAgent(string) ([]model.Task, error)
	ListActiveTasks() ([]model.Task, error)
	CreateTaskAttempt(*model.TaskAttempt) error
	ListTaskAttemptsByTask(uuid.UUID) ([]model.TaskAttempt, error)
	ListAuditsByAggregate(string, string) ([]model.AuditLog, error)
	UpsertRuntimeInstance(*model.RuntimeInstance) error
	GetRuntimeInstanceByServiceAndSlot(uuid.UUID, model.Slot) (*model.RuntimeInstance, error)
	ListRuntimeInstancesByService(uuid.UUID) ([]model.RuntimeInstance, error)
	CreateAudit(*model.AuditLog) error
}

var ErrAgentOffline = errors.New("agent offline")

type TaskDispatcher interface {
	DispatchTask(agentID string, task *model.Task) error
	ReplayTask(agentID string, task *model.Task) (bool, error)
}

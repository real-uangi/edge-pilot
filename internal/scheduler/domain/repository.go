package domain

import (
	"edge-pilot/internal/shared/model"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	CreateJob(job *model.SchedulerJob) error
	UpdateJob(job *model.SchedulerJob) error
	GetJob(id uuid.UUID) (*model.SchedulerJob, error)
	ListJobs() ([]model.SchedulerJob, error)
	ListJobsDue(now time.Time, limit int) ([]model.SchedulerJob, error)
	DeleteJob(id uuid.UUID) error

	CreateRun(run *model.SchedulerJobRun) error
	UpdateRun(run *model.SchedulerJobRun) error
	GetRun(id uuid.UUID) (*model.SchedulerJobRun, error)
	ListRunsByJob(jobID uuid.UUID, limit int) ([]model.SchedulerJobRun, error)
	ListAllRuns(limit int) ([]model.SchedulerJobRun, error)
	ListDispatchableRuns(now time.Time, limit int) ([]model.SchedulerJobRun, error)
	ClaimRun(runID uuid.UUID, leasedBy string, leaseExpiresAt time.Time, now time.Time) (bool, error)

	GetDispatchCursor(jobID uuid.UUID, executorGroup string) (*model.SchedulerDispatchCursor, error)
	SaveDispatchCursor(cursor *model.SchedulerDispatchCursor) error

	UpsertExecutor(executor *model.SchedulerExecutor) error
	GetExecutor(id string) (*model.SchedulerExecutor, error)
	ListExecutorsByGroup(group string) ([]model.SchedulerExecutor, error)
	ListExecutors() ([]model.SchedulerExecutor, error)
	DeleteExecutor(id string) error
	MarkExecutorSeen(id string, at time.Time) error

	WithTx(fn func(tx Repository) error) error
}

type LiveSlotResolver interface {
	ResolveLiveSlot(serviceID uuid.UUID) (model.Slot, error)
}

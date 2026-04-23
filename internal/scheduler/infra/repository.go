package infra

import (
	schedulerdomain "edge-pilot/internal/scheduler/domain"
	"edge-pilot/internal/shared/model"
	"time"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/db"
	"gorm.io/gorm"
)

type repository struct {
	conn *gorm.DB
}

func NewRepository(manager *db.Manager) schedulerdomain.Repository {
	return &repository{conn: manager.GetDB()}
}

func (r *repository) CreateJob(job *model.SchedulerJob) error {
	return r.conn.Create(job).Error
}

func (r *repository) UpdateJob(job *model.SchedulerJob) error {
	return r.conn.Model(job).Updates(job).Error
}

func (r *repository) GetJob(id uuid.UUID) (*model.SchedulerJob, error) {
	var job model.SchedulerJob
	result := r.conn.Where("id = ?", id).First(&job)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &job, nil
}

func (r *repository) ListJobs() ([]model.SchedulerJob, error) {
	var jobs []model.SchedulerJob
	if err := r.conn.Order("created_at desc").Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *repository) ListJobsDue(now time.Time, limit int) ([]model.SchedulerJob, error) {
	var jobs []model.SchedulerJob
	query := r.conn.Where("enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, now).
		Order("next_run_at asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *repository) DeleteJob(id uuid.UUID) error {
	if err := r.conn.Where("job_id = ?", id).Delete(&model.SchedulerJobRun{}).Error; err != nil {
		return err
	}
	if err := r.conn.Where("job_id = ?", id).Delete(&model.SchedulerDispatchCursor{}).Error; err != nil {
		return err
	}
	return r.conn.Where("id = ?", id).Delete(&model.SchedulerJob{}).Error
}

func (r *repository) CreateRun(run *model.SchedulerJobRun) error {
	return r.conn.Create(run).Error
}

func (r *repository) UpdateRun(run *model.SchedulerJobRun) error {
	return r.conn.Model(run).Updates(run).Error
}

func (r *repository) GetRun(id uuid.UUID) (*model.SchedulerJobRun, error) {
	var run model.SchedulerJobRun
	result := r.conn.Where("id = ?", id).First(&run)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &run, nil
}

func (r *repository) ListRunsByJob(jobID uuid.UUID, limit int) ([]model.SchedulerJobRun, error) {
	var runs []model.SchedulerJobRun
	query := r.conn.Where("job_id = ?", jobID).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

func (r *repository) ListDispatchableRuns(now time.Time, limit int) ([]model.SchedulerJobRun, error) {
	var runs []model.SchedulerJobRun
	query := r.conn.Where("status = ? OR (status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?) OR (status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?)",
		model.SchedulerJobRunStatusPending,
		model.SchedulerJobRunStatusRetryWaiting,
		now,
		model.SchedulerJobRunStatusDispatched,
		now,
		model.SchedulerJobRunStatusRunning,
		now,
	).Order("scheduled_at asc").Limit(limit)
	if err := query.Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

func (r *repository) ClaimRun(runID uuid.UUID, leasedBy string, leaseExpiresAt time.Time, now time.Time) (bool, error) {
	result := r.conn.Model(&model.SchedulerJobRun{}).
		Where("id = ? AND (status = ? OR (status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?) OR (status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?))",
			runID,
			model.SchedulerJobRunStatusPending,
			model.SchedulerJobRunStatusRetryWaiting,
			now,
			model.SchedulerJobRunStatusDispatched,
			now,
			model.SchedulerJobRunStatusRunning,
			now,
		).
		Updates(map[string]any{
			"status":           model.SchedulerJobRunStatusDispatched,
			"leased_by":        leasedBy,
			"lease_expires_at": leaseExpiresAt,
			"dispatched_at":    now,
			"next_retry_at":    nil,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) GetDispatchCursor(jobID uuid.UUID, executorGroup string) (*model.SchedulerDispatchCursor, error) {
	var cursor model.SchedulerDispatchCursor
	result := r.conn.Where("job_id = ? AND executor_group = ?", jobID, executorGroup).First(&cursor)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &cursor, nil
}

func (r *repository) SaveDispatchCursor(cursor *model.SchedulerDispatchCursor) error {
	if cursor.ID == uuid.Nil {
		cursor.ID = uuid.New()
	}
	var current model.SchedulerDispatchCursor
	result := r.conn.Where("job_id = ? AND executor_group = ?", cursor.JobID, cursor.ExecutorGroup).First(&current)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.conn.Create(cursor).Error
	}
	cursor.ID = current.ID
	cursor.CreatedAt = current.CreatedAt
	return r.conn.Model(cursor).Updates(cursor).Error
}

func (r *repository) UpsertExecutor(executor *model.SchedulerExecutor) error {
	var current model.SchedulerExecutor
	result := r.conn.Where("id = ?", executor.ID).First(&current)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.conn.Create(executor).Error
	}
	executor.CreatedAt = current.CreatedAt
	return r.conn.Model(executor).Where("id = ?", executor.ID).Updates(executor).Error
}

func (r *repository) GetExecutor(id string) (*model.SchedulerExecutor, error) {
	var executor model.SchedulerExecutor
	result := r.conn.Where("id = ?", id).First(&executor)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &executor, nil
}

func (r *repository) ListExecutorsByGroup(group string) ([]model.SchedulerExecutor, error) {
	var executors []model.SchedulerExecutor
	if err := r.conn.Where("\"group\" = ?", group).Order("id asc").Find(&executors).Error; err != nil {
		return nil, err
	}
	return executors, nil
}

func (r *repository) ListExecutors() ([]model.SchedulerExecutor, error) {
	var executors []model.SchedulerExecutor
	if err := r.conn.Order("created_at desc").Find(&executors).Error; err != nil {
		return nil, err
	}
	return executors, nil
}

func (r *repository) DeleteExecutor(id string) error {
	return r.conn.Where("id = ?", id).Delete(&model.SchedulerExecutor{}).Error
}

func (r *repository) MarkExecutorSeen(id string, at time.Time) error {
	return r.conn.Model(&model.SchedulerExecutor{}).Where("id = ?", id).Updates(map[string]any{"last_seen_at": at}).Error
}

func (r *repository) WithTx(fn func(tx schedulerdomain.Repository) error) error {
	return r.conn.Transaction(func(txConn *gorm.DB) error {
		tx := &repository{conn: txConn}
		return fn(tx)
	})
}

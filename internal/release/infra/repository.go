package infra

import (
	"edge-pilot/internal/release/domain"
	"edge-pilot/internal/shared/model"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/db"
	"gorm.io/gorm"
)

type repository struct {
	conn *gorm.DB
}

func NewRepository(manager *db.Manager) domain.Repository {
	return &repository{conn: manager.GetDB()}
}

func (r *repository) CreateRelease(release *model.Release) error {
	return r.conn.Create(release).Error
}

func (r *repository) UpdateRelease(release *model.Release) error {
	return r.conn.Model(release).Updates(release).Error
}

func (r *repository) GetRelease(id uuid.UUID) (*model.Release, error) {
	var release model.Release
	result := r.conn.Where("id = ?", id).First(&release)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &release, nil
}

func (r *repository) ListReleases(limit int) ([]model.Release, error) {
	var releases []model.Release
	query := r.conn.Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&releases).Error; err != nil {
		return nil, err
	}
	return releases, nil
}

func (r *repository) HasActiveRelease(serviceID uuid.UUID) (bool, error) {
	var count int64
	if err := r.conn.Model(&model.Release{}).
		Where("service_id = ? AND status IN ?", serviceID, []model.ReleaseStatus{
			model.ReleaseStatusPending,
			model.ReleaseStatusDispatching,
			model.ReleaseStatusDeploying,
			model.ReleaseStatusReadyToSwitch,
			model.ReleaseStatusSwitched,
		}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *repository) CreateTask(task *model.Task) error {
	return r.conn.Create(task).Error
}

func (r *repository) UpdateTask(task *model.Task) error {
	return r.conn.Model(task).Updates(task).Error
}

func (r *repository) GetTask(id uuid.UUID) (*model.Task, error) {
	var task model.Task
	result := r.conn.Where("id = ?", id).First(&task)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &task, nil
}

func (r *repository) ListTasksByRelease(releaseID uuid.UUID) ([]model.Task, error) {
	var tasks []model.Task
	if err := r.conn.Where("release_id = ?", releaseID).Order("created_at asc").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *repository) CreateTaskAttempt(attempt *model.TaskAttempt) error {
	return r.conn.Create(attempt).Error
}

func (r *repository) UpsertRuntimeInstance(instance *model.RuntimeInstance) error {
	var current model.RuntimeInstance
	result := r.conn.Where("service_id = ? AND slot = ?", instance.ServiceID, instance.Slot).First(&current)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.conn.Create(instance).Error
	}
	instance.ID = current.ID
	instance.CreatedAt = current.CreatedAt
	return r.conn.Model(instance).Updates(instance).Error
}

func (r *repository) GetRuntimeInstanceByServiceAndSlot(serviceID uuid.UUID, slot model.Slot) (*model.RuntimeInstance, error) {
	var instance model.RuntimeInstance
	result := r.conn.Where("service_id = ? AND slot = ?", serviceID, slot).First(&instance)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &instance, nil
}

func (r *repository) ListRuntimeInstancesByService(serviceID uuid.UUID) ([]model.RuntimeInstance, error) {
	var instances []model.RuntimeInstance
	if err := r.conn.Where("service_id = ?", serviceID).Order("created_at asc").Find(&instances).Error; err != nil {
		return nil, err
	}
	return instances, nil
}

func (r *repository) CreateAudit(log *model.AuditLog) error {
	return r.conn.Create(log).Error
}

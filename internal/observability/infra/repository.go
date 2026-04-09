package infra

import (
	"edge-pilot/internal/observability/domain"
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

func (r *repository) SaveBackendStats(stats []model.BackendStatSnapshot) error {
	if len(stats) == 0 {
		return nil
	}
	return r.conn.Create(&stats).Error
}

func (r *repository) ListBackendStats(serviceID uuid.UUID) ([]model.BackendStatSnapshot, error) {
	var stats []model.BackendStatSnapshot
	if err := r.conn.Where("service_id = ?", serviceID).Order("created_at desc").Limit(20).Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *repository) CountActiveInstances() (int64, error) {
	var count int64
	active := true
	if err := r.conn.Model(&model.RuntimeInstance{}).Where("active = ?", &active).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

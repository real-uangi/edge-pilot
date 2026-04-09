package infra

import (
	"edge-pilot/internal/servicecatalog/domain"
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

func (r *repository) Create(service *model.Service) error {
	return r.conn.Create(service).Error
}

func (r *repository) Update(service *model.Service) error {
	return r.conn.Model(service).Updates(service).Error
}

func (r *repository) GetByID(id uuid.UUID) (*model.Service, error) {
	var service model.Service
	result := r.conn.Where("id = ?", id).First(&service)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &service, nil
}

func (r *repository) GetByKey(key string) (*model.Service, error) {
	var service model.Service
	result := r.conn.Where("service_key = ?", key).First(&service)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &service, nil
}

func (r *repository) List() ([]model.Service, error) {
	var services []model.Service
	if err := r.conn.Order("created_at desc").Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

func (r *repository) UpdateLiveSlot(id uuid.UUID, slot model.Slot) error {
	return r.conn.Model(&model.Service{}).Where("id = ?", id).Update("current_live_slot", slot).Error
}

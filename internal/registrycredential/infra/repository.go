package infra

import (
	"edge-pilot/internal/registrycredential/domain"
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

func (r *repository) Create(item *model.RegistryCredential) error {
	return r.conn.Create(item).Error
}

func (r *repository) Update(item *model.RegistryCredential) error {
	return r.conn.Model(item).Updates(item).Error
}

func (r *repository) Delete(id uuid.UUID) error {
	return r.conn.Delete(&model.RegistryCredential{}, "id = ?", id).Error
}

func (r *repository) Get(id uuid.UUID) (*model.RegistryCredential, error) {
	var item model.RegistryCredential
	result := r.conn.Where("id = ?", id).First(&item)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &item, nil
}

func (r *repository) GetByRegistryHost(host string) (*model.RegistryCredential, error) {
	var item model.RegistryCredential
	result := r.conn.Where("registry_host = ?", host).First(&item)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &item, nil
}

func (r *repository) List() ([]model.RegistryCredential, error) {
	var items []model.RegistryCredential
	if err := r.conn.Order("created_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

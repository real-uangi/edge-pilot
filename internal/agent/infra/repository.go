package infra

import (
	"edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/model"

	"github.com/real-uangi/allingo/common/db"
	"gorm.io/gorm"
)

type repository struct {
	conn *gorm.DB
}

func NewRepository(manager *db.Manager) domain.Repository {
	return &repository{conn: manager.GetDB()}
}

func (r *repository) Save(node *model.AgentNode) error {
	return r.conn.Save(node).Error
}

func (r *repository) Get(id string) (*model.AgentNode, error) {
	var node model.AgentNode
	result := r.conn.Where("id = ?", id).First(&node)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &node, nil
}

func (r *repository) List() ([]model.AgentNode, error) {
	var nodes []model.AgentNode
	if err := r.conn.Order("created_at desc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *repository) MarkOffline(id string, lastError string) error {
	offline := false
	return r.conn.Model(&model.AgentNode{}).Where("id = ?", id).Updates(map[string]any{
		"online":     &offline,
		"last_error": lastError,
	}).Error
}

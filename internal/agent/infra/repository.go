package infra

import (
	"edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/model"
	"time"

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

func (r *repository) ListEnabled() ([]model.AgentNode, error) {
	var nodes []model.AgentNode
	if err := r.conn.Where("enabled = ?", true).Order("created_at desc").Find(&nodes).Error; err != nil {
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

func (r *repository) MarkOfflineStale(before time.Time) ([]string, error) {
	var ids []string
	if err := r.conn.Model(&model.AgentNode{}).
		Where("online = ? AND last_heartbeat_at IS NOT NULL AND last_heartbeat_at < ?", true, before).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	offline := false
	if err := r.conn.Model(&model.AgentNode{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"online":     &offline,
			"last_error": "heartbeat timeout",
		}).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

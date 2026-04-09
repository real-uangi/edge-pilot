package domain

import (
	"edge-pilot/internal/shared/model"
	"time"
)

type Repository interface {
	Save(*model.AgentNode) error
	Get(string) (*model.AgentNode, error)
	List() ([]model.AgentNode, error)
	ListEnabled() ([]model.AgentNode, error)
	MarkOffline(string, string) error
	MarkOfflineStale(time.Time) ([]string, error)
}

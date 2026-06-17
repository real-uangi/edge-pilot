package domain

import (
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/model"
)

type Repository interface {
	Save(*model.AgentNode) error
	Get(string) (*model.AgentNode, error)
	Delete(string) error
	List() ([]model.AgentNode, error)
	ListEnabled() ([]model.AgentNode, error)
	MarkOffline(string, string) error
	MarkOfflineStale(time.Time) ([]string, error)
}

type ServiceBindingChecker interface {
	CountByAgent(string) (int, error)
}

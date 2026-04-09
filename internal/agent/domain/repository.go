package domain

import (
	"edge-pilot/internal/shared/model"
)

type Repository interface {
	Save(*model.AgentNode) error
	Get(string) (*model.AgentNode, error)
	List() ([]model.AgentNode, error)
	MarkOffline(string, string) error
}

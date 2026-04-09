package domain

import (
	"edge-pilot/internal/shared/model"

	"github.com/google/uuid"
)

type Repository interface {
	Create(*model.Service) error
	Update(*model.Service) error
	GetByID(uuid.UUID) (*model.Service, error)
	GetByKey(string) (*model.Service, error)
	List() ([]model.Service, error)
	UpdateLiveSlot(uuid.UUID, model.Slot) error
}

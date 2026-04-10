package domain

import (
	"edge-pilot/internal/shared/model"

	"github.com/google/uuid"
)

type Repository interface {
	Create(*model.RegistryCredential) error
	Update(*model.RegistryCredential) error
	Delete(uuid.UUID) error
	Get(uuid.UUID) (*model.RegistryCredential, error)
	GetByRegistryHost(string) (*model.RegistryCredential, error)
	List() ([]model.RegistryCredential, error)
}

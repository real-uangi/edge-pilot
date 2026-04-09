package domain

import (
	"edge-pilot/internal/shared/model"

	"github.com/google/uuid"
)

type Repository interface {
	SaveBackendStats([]model.BackendStatSnapshot) error
	ListBackendStats(uuid.UUID) ([]model.BackendStatSnapshot, error)
	CountActiveInstances() (int64, error)
}

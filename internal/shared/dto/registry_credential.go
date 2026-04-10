package dto

import (
	"time"

	"github.com/google/uuid"
)

type UpsertRegistryCredentialRequest struct {
	RegistryHost string `json:"registryHost" binding:"required"`
	Username     string `json:"username" binding:"required"`
	Secret       string `json:"secret" binding:"required"`
}

type RegistryCredentialOutput struct {
	ID               uuid.UUID `json:"id"`
	RegistryHost     string    `json:"registryHost"`
	Username         string    `json:"username"`
	SecretConfigured bool      `json:"secretConfigured"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

package domain

import "github.com/real-uangi/edge-pilot/internal/shared/dto"

type ResolvedRegistryCredential = dto.ResolvedRegistryCredential

type RegistryCredentialResolver interface {
	ResolveForImageRepo(imageRepo string) (*ResolvedRegistryCredential, error)
}

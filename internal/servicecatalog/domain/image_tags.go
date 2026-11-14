package domain

import (
	"context"

	"github.com/real-uangi/edge-pilot/internal/shared/dto"
)

type ImageTagLister interface {
	ListTags(context.Context, string, string, *dto.ResolvedRegistryCredential) (*dto.ServiceImageTagsOutput, error)
}

type RegistryCredentialResolver interface {
	ResolveForImageRepo(string) (*dto.ResolvedRegistryCredential, error)
}

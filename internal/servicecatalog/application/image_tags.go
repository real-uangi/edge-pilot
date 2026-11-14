package application

import (
	"context"

	"github.com/real-uangi/edge-pilot/internal/servicecatalog/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/image"

	"github.com/real-uangi/allingo/common/business"
)

type ImageTagService struct {
	repo        domain.Repository
	credentials domain.RegistryCredentialResolver
	tags        domain.ImageTagLister
}

func NewImageTagService(repo domain.Repository, credentials domain.RegistryCredentialResolver, tags domain.ImageTagLister) *ImageTagService {
	return &ImageTagService{repo: repo, credentials: credentials, tags: tags}
}

func (s *ImageTagService) List(ctx context.Context, req dto.ListServiceImageTagsRequest) (*dto.ServiceImageTagsOutput, error) {
	if req.Cursor != "" && !image.ValidTag(req.Cursor) {
		return nil, business.NewBadRequest("cursor 必须是合法镜像 Tag")
	}
	service, err := s.repo.GetByID(req.ServiceID)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, business.ErrNotFound
	}
	credentials, err := s.credentials.ResolveForImageRepo(service.ImageRepo)
	if err != nil {
		return nil, err
	}
	return s.tags.ListTags(ctx, service.ImageRepo, req.Cursor, credentials)
}

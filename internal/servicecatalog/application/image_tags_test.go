package application

import (
	"context"
	"errors"
	"testing"

	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
)

type imageTagResolverFunc func(string) (*dto.ResolvedRegistryCredential, error)

func (f imageTagResolverFunc) ResolveForImageRepo(repo string) (*dto.ResolvedRegistryCredential, error) {
	return f(repo)
}

type imageTagListerFunc func(context.Context, string, string, *dto.ResolvedRegistryCredential) (*dto.ServiceImageTagsOutput, error)

func (f imageTagListerFunc) ListTags(ctx context.Context, repo, cursor string, credential *dto.ResolvedRegistryCredential) (*dto.ServiceImageTagsOutput, error) {
	return f(ctx, repo, cursor, credential)
}

func TestImageTagServiceUsesSavedRepositoryAndCredentials(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	id := uuid.New()
	repo.byID[id] = &model.Service{ID: id, ImageRepo: "ghcr.io/team/saved-app"}
	credential := &dto.ResolvedRegistryCredential{Host: "ghcr.io", Username: "robot", Secret: "secret"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service := NewImageTagService(repo, imageTagResolverFunc(func(imageRepo string) (*dto.ResolvedRegistryCredential, error) {
		if imageRepo != repo.byID[id].ImageRepo {
			t.Fatalf("resolved credentials for %q", imageRepo)
		}
		return credential, nil
	}), imageTagListerFunc(func(gotCtx context.Context, imageRepo, cursor string, gotCredential *dto.ResolvedRegistryCredential) (*dto.ServiceImageTagsOutput, error) {
		if gotCtx != ctx || imageRepo != repo.byID[id].ImageRepo || cursor != "v1" || gotCredential != credential {
			t.Fatalf("unexpected registry input: repo=%s cursor=%s credential=%#v", imageRepo, cursor, gotCredential)
		}
		return &dto.ServiceImageTagsOutput{ImageRepo: imageRepo, Tags: []string{"v2"}}, nil
	}))
	output, err := service.List(ctx, dto.ListServiceImageTagsRequest{ServiceID: id, Cursor: "v1"})
	if err != nil || output.ImageRepo != repo.byID[id].ImageRepo || output.Tags[0] != "v2" {
		t.Fatalf("List() = %#v, %v", output, err)
	}
}

func TestImageTagServiceRejectsMissingServiceAndInvalidCursor(t *testing.T) {
	service := NewImageTagService(newFakeServiceCatalogRepo(), nil, nil)
	if _, err := service.List(context.Background(), dto.ListServiceImageTagsRequest{ServiceID: uuid.New()}); !errors.Is(err, business.ErrNotFound) {
		t.Fatalf("expected missing service, got %v", err)
	}
	if _, err := service.List(context.Background(), dto.ListServiceImageTagsRequest{ServiceID: uuid.New(), Cursor: "https://other.example.com/"}); err == nil || errors.Is(err, business.ErrNotFound) {
		t.Fatalf("expected invalid cursor, got %v", err)
	}
}

func TestImageTagServicePropagatesCredentialFailureWithoutRegistryRequest(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	id := uuid.New()
	repo.byID[id] = &model.Service{ID: id, ImageRepo: "ghcr.io/team/app"}
	want := errors.New("credential decryption failed")
	service := NewImageTagService(repo, imageTagResolverFunc(func(string) (*dto.ResolvedRegistryCredential, error) {
		return nil, want
	}), nil)
	if _, err := service.List(context.Background(), dto.ListServiceImageTagsRequest{ServiceID: id}); !errors.Is(err, want) {
		t.Fatalf("expected credential failure, got %v", err)
	}
}

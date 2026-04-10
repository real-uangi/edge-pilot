package application

import (
	releasedomain "edge-pilot/internal/release/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"testing"

	"github.com/google/uuid"
)

func TestResolveRegistryHost(t *testing.T) {
	testCases := map[string]string{
		"ghcr.io/org/app":             "ghcr.io",
		"harbor.example.com/team/app": "harbor.example.com",
		"localhost:5000/app":          "localhost:5000",
		"nginx":                       "docker.io",
		"library/nginx":               "docker.io",
	}
	for imageRepo, expected := range testCases {
		if got := ResolveRegistryHost(imageRepo); got != expected {
			t.Fatalf("ResolveRegistryHost(%q) = %q, want %q", imageRepo, got, expected)
		}
	}
}

func TestServiceCreateListGetAndResolve(t *testing.T) {
	repo := newFakeRegistryCredentialRepo()
	service := NewService(repo, NewCrypto(&config.RegistryCredentialConfig{
		MasterKey:  []byte("0123456789abcdef0123456789abcdef"),
		KeyVersion: "v1",
	}))

	created, err := service.Create(dto.UpsertRegistryCredentialRequest{
		RegistryHost: "GHCR.IO",
		Username:     "octocat",
		Secret:       "token-value",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.RegistryHost != "ghcr.io" {
		t.Fatalf("expected normalized registry host, got %q", created.RegistryHost)
	}
	if !created.SecretConfigured {
		t.Fatal("expected secretConfigured to be true")
	}

	got, err := service.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Username != "octocat" {
		t.Fatalf("expected username octocat, got %q", got.Username)
	}

	items, err := service.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}

	resolved, err := service.ResolveForImageRepo("ghcr.io/openai/edge-pilot")
	if err != nil {
		t.Fatalf("ResolveForImageRepo() error = %v", err)
	}
	if resolved == nil {
		t.Fatal("expected resolved registry credential")
	}
	expected := &releasedomain.ResolvedRegistryCredential{
		Host:     "ghcr.io",
		Username: "octocat",
		Secret:   "token-value",
	}
	if *resolved != *expected {
		t.Fatalf("unexpected resolved credential: %#v", resolved)
	}
}

func TestServiceRejectsDuplicateRegistryHost(t *testing.T) {
	repo := newFakeRegistryCredentialRepo()
	service := NewService(repo, NewCrypto(&config.RegistryCredentialConfig{
		MasterKey:  []byte("0123456789abcdef0123456789abcdef"),
		KeyVersion: "v1",
	}))

	_, err := service.Create(dto.UpsertRegistryCredentialRequest{
		RegistryHost: "ghcr.io",
		Username:     "octocat",
		Secret:       "token-a",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.Create(dto.UpsertRegistryCredentialRequest{
		RegistryHost: "ghcr.io",
		Username:     "octocat",
		Secret:       "token-b",
	}); err == nil {
		t.Fatal("expected duplicate registry host to fail")
	}
}

type fakeRegistryCredentialRepo struct {
	items map[uuid.UUID]*model.RegistryCredential
}

func newFakeRegistryCredentialRepo() *fakeRegistryCredentialRepo {
	return &fakeRegistryCredentialRepo{items: map[uuid.UUID]*model.RegistryCredential{}}
}

func (r *fakeRegistryCredentialRepo) Create(item *model.RegistryCredential) error {
	copyItem := *item
	r.items[item.ID] = &copyItem
	return nil
}

func (r *fakeRegistryCredentialRepo) Update(item *model.RegistryCredential) error {
	copyItem := *item
	r.items[item.ID] = &copyItem
	return nil
}

func (r *fakeRegistryCredentialRepo) Delete(id uuid.UUID) error {
	delete(r.items, id)
	return nil
}

func (r *fakeRegistryCredentialRepo) Get(id uuid.UUID) (*model.RegistryCredential, error) {
	item := r.items[id]
	if item == nil {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

func (r *fakeRegistryCredentialRepo) GetByRegistryHost(host string) (*model.RegistryCredential, error) {
	for _, item := range r.items {
		if item.RegistryHost == host {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, nil
}

func (r *fakeRegistryCredentialRepo) List() ([]model.RegistryCredential, error) {
	out := make([]model.RegistryCredential, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, *item)
	}
	return out, nil
}

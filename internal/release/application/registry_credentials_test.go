package application

import (
	agentapp "edge-pilot/internal/agent/application"
	releasedomain "edge-pilot/internal/release/domain"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStartQueuedReleaseInjectsRegistryCredentialIntoDeployTask(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	resolver := fakeRegistryCredentialResolver{
		credential: &resolvedRegistryCredential{
			host:     "ghcr.io",
			username: "octocat",
			secret:   "token-value",
		},
	}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentapp.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewServiceWithRegistryCredentials(releaseRepo, dispatcher, serviceCatalog, registry, resolver)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "ghcr.io/openai/edge-pilot",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	agentRepo.nodes["agent-a"] = &model.AgentNode{
		ID:              "agent-a",
		Enabled:         &enabled,
		Online:          &online,
		LastHeartbeatAt: &now,
	}

	queued, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.0.0",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() error = %v", err)
	}
	if _, err := releaseService.Start(queued.ID, "admin"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if len(dispatcher.tasks) != 1 {
		t.Fatalf("expected one dispatched task, got %d", len(dispatcher.tasks))
	}
	payload := dispatcher.tasks[0].Payload.Get()
	if payload.RegistryHost != "ghcr.io" || payload.RegistryUsername != "octocat" || payload.RegistrySecret != "token-value" {
		t.Fatalf("expected registry credential fields to be injected, got %#v", payload)
	}
}

type resolvedRegistryCredential struct {
	host     string
	username string
	secret   string
}

type fakeRegistryCredentialResolver struct {
	credential *resolvedRegistryCredential
}

func (f fakeRegistryCredentialResolver) ResolveForImageRepo(string) (*releasedomain.ResolvedRegistryCredential, error) {
	if f.credential == nil {
		return nil, nil
	}
	return &releasedomain.ResolvedRegistryCredential{
		Host:     f.credential.host,
		Username: f.credential.username,
		Secret:   f.credential.secret,
	}, nil
}

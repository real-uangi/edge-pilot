package application

import (
	agentapp "edge-pilot/internal/agent/application"
	releasedomain "edge-pilot/internal/release/domain"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"edge-pilot/internal/shared/secret"
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
	codec := secret.NewCodec(&config.ServiceSecretConfig{
		MasterKey:  []byte("12345678901234567890123456789012"),
		KeyVersion: "v1",
	})
	releaseService := NewServiceWithRegistryCredentialsAndCodec(releaseRepo, dispatcher, serviceCatalog, registry, resolver, codec)

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
	if payload.RegistryHost != "ghcr.io" || payload.RegistryUsername != "octocat" {
		t.Fatalf("expected registry host fields to be injected, got %#v", payload)
	}
	if payload.RegistrySecret != "" {
		t.Fatalf("expected registry secret to be removed from plaintext payload, got %#v", payload)
	}
	var sensitive model.TaskSensitivePayload
	if err := codec.DecryptJSON(dispatcher.tasks[0].SensitiveCiphertext, dispatcher.tasks[0].SensitiveKeyVersion, &sensitive); err != nil {
		t.Fatalf("DecryptJSON() error = %v", err)
	}
	if sensitive.RegistrySecret != "token-value" {
		t.Fatalf("expected encrypted registry secret, got %#v", sensitive)
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

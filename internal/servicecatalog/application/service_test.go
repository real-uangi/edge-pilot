package application

import (
	"edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"edge-pilot/internal/shared/secret"
	"testing"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
)

func TestCreateRejectsDuplicateRouteOnSameAgent(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	publisher := &fakeProxyPublisher{}
	agents := &fakeAgentLookup{agents: map[string]*dto.AgentOutput{
		"11111111-1111-1111-1111-111111111111": {ID: "11111111-1111-1111-1111-111111111111", Enabled: boolPointer(true)},
	}}
	svc := NewServiceWithPublisher(repo, publisher, agents)

	first, err := svc.Create(dto.UpsertServiceRequest{
		Name:            "svc-a",
		ServiceKey:      "svc-a",
		AgentID:         "11111111-1111-1111-1111-111111111111",
		ImageRepo:       "repo/app",
		ContainerPort:   8080,
		RouteHost:       "Example.COM",
		RoutePathPrefix: "/api/",
	})
	if err != nil {
		t.Fatalf("Create() first error = %v", err)
	}
	if first.RouteHost != "example.com" || first.RoutePathPrefix != "/api" {
		t.Fatalf("expected normalized route, got host=%q path=%q", first.RouteHost, first.RoutePathPrefix)
	}
	if len(publisher.published) != 1 || publisher.published[0] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected publish for issued agent UUID, got %#v", publisher.published)
	}

	_, err = svc.Create(dto.UpsertServiceRequest{
		Name:            "svc-b",
		ServiceKey:      "svc-b",
		AgentID:         "11111111-1111-1111-1111-111111111111",
		ImageRepo:       "repo/app",
		ContainerPort:   8080,
		RouteHost:       "example.com",
		RoutePathPrefix: "/api",
	})
	if err == nil {
		t.Fatalf("expected duplicate route validation error")
	}
}

func TestBuildProxyServiceConfigsSortsLongestPathFirst(t *testing.T) {
	enabled := true
	configs := BuildProxyServiceConfigs([]model.Service{
		{
			ID:              uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			ServiceKey:      "svc-root",
			RouteHost:       "api.example.com",
			RoutePathPrefix: "/",
			CurrentLiveSlot: model.SlotBlue,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
		{
			ID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ServiceKey:      "svc-api",
			RouteHost:       "api.example.com",
			RoutePathPrefix: "/v1/internal",
			CurrentLiveSlot: model.SlotGreen,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
	})
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	if configs[0].ServiceKey != "svc-api" || configs[1].ServiceKey != "svc-root" {
		t.Fatalf("expected longest path first, got %#v", configs)
	}
	if configs[0].BackendName != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("unexpected backend name: %s", configs[0].BackendName)
	}
	if configs[0].CurrentLiveSlot != model.SlotGreen {
		t.Fatalf("expected current live slot to be preserved")
	}
}

func TestCreateRejectsDuplicatePublishedPortOnSameAgent(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	agents := &fakeAgentLookup{agents: map[string]*dto.AgentOutput{
		"11111111-1111-1111-1111-111111111111": {ID: "11111111-1111-1111-1111-111111111111", Enabled: boolPointer(true)},
	}}
	svc := NewServiceWithPublisher(repo, nil, agents)

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
		PublishedPorts: []dto.PublishedPort{
			{HostPort: 18080, ContainerPort: 8080},
		},
	}); err != nil {
		t.Fatalf("Create() first error = %v", err)
	}

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-b",
		ServiceKey:    "svc-b",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "b.example.com",
		PublishedPorts: []dto.PublishedPort{
			{HostPort: 18080, ContainerPort: 9090},
		},
	}); err == nil {
		t.Fatalf("expected duplicate published host port validation error")
	}
}

func TestCreateRejectsUnknownOrDisabledAgent(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, &fakeAgentLookup{agents: map[string]*dto.AgentOutput{
		"22222222-2222-2222-2222-222222222222": {ID: "22222222-2222-2222-2222-222222222222", Enabled: boolPointer(false)},
	}})

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "not-a-uuid",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
	}); err == nil {
		t.Fatalf("expected invalid uuid agentId to be rejected")
	}

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-b",
		ServiceKey:    "svc-b",
		AgentID:       "33333333-3333-3333-3333-333333333333",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "b.example.com",
	}); err == nil {
		t.Fatalf("expected unknown agent to be rejected")
	}

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-c",
		ServiceKey:    "svc-c",
		AgentID:       "22222222-2222-2222-2222-222222222222",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "c.example.com",
	}); err == nil {
		t.Fatalf("expected disabled agent to be rejected")
	}
}

func TestCreateEncryptsEnvAndGetDecryptsIt(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisherAndCodec(repo, nil, nil, newServiceSecretCodec())

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
		Env:           map[string]string{"A": "1"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	stored := repo.byID[created.ID]
	if stored == nil {
		t.Fatal("expected stored service")
	}
	if stored.Env != nil {
		t.Fatalf("expected plaintext env to be cleared, got %#v", stored.Env)
	}
	if stored.EnvCiphertext == "" || stored.EnvKeyVersion == "" {
		t.Fatalf("expected encrypted env to be stored, got ciphertext=%q version=%q", stored.EnvCiphertext, stored.EnvKeyVersion)
	}

	output, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if output.Env["A"] != "1" {
		t.Fatalf("expected decrypted env, got %#v", output.Env)
	}
}

func TestCreateRejectsNonEmptyEnvWithoutCodec(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
		Env:           map[string]string{"A": "1"},
	}); err == nil {
		t.Fatal("expected non-empty env to require codec")
	}
}

func TestGetFallsBackToPlaintextEnvForLegacyData(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)
	enabled := true
	repo.byID[uuid.MustParse("11111111-1111-1111-1111-111111111111")] = &model.Service{
		ID:            uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ServiceKey:    "svc-a",
		Name:          "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
		Env:           mustServiceEnvJSONB(map[string]string{"LEGACY": "1"}),
		Enabled:       &enabled,
	}

	output, err := svc.Get(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if output.Env["LEGACY"] != "1" {
		t.Fatalf("expected plaintext env fallback, got %#v", output.Env)
	}
}

type fakeServiceCatalogRepo struct {
	byID  map[uuid.UUID]*model.Service
	byKey map[string]*model.Service
}

func newFakeServiceCatalogRepo() *fakeServiceCatalogRepo {
	return &fakeServiceCatalogRepo{
		byID:  make(map[uuid.UUID]*model.Service),
		byKey: make(map[string]*model.Service),
	}
}

func (r *fakeServiceCatalogRepo) Create(service *model.Service) error {
	copyService := *service
	r.byID[service.ID] = &copyService
	r.byKey[service.ServiceKey] = &copyService
	return nil
}

func (r *fakeServiceCatalogRepo) Update(service *model.Service) error {
	copyService := *service
	r.byID[service.ID] = &copyService
	r.byKey[service.ServiceKey] = &copyService
	return nil
}

func (r *fakeServiceCatalogRepo) GetByID(id uuid.UUID) (*model.Service, error) {
	return r.byID[id], nil
}

func (r *fakeServiceCatalogRepo) GetByKey(key string) (*model.Service, error) {
	return r.byKey[key], nil
}

func (r *fakeServiceCatalogRepo) GetByRoute(agentID string, routeHost string, routePathPrefix string) (*model.Service, error) {
	for _, item := range r.byID {
		if item.AgentID == agentID && item.RouteHost == routeHost && item.RoutePathPrefix == routePathPrefix {
			return item, nil
		}
	}
	return nil, nil
}

func (r *fakeServiceCatalogRepo) List() ([]model.Service, error) {
	out := make([]model.Service, 0, len(r.byID))
	for _, item := range r.byID {
		out = append(out, *item)
	}
	return out, nil
}

func (r *fakeServiceCatalogRepo) ListByAgent(agentID string) ([]model.Service, error) {
	out := make([]model.Service, 0, len(r.byID))
	for _, item := range r.byID {
		if item.AgentID == agentID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (r *fakeServiceCatalogRepo) UpdateLiveSlot(id uuid.UUID, slot model.Slot) error {
	if item := r.byID[id]; item != nil {
		item.CurrentLiveSlot = slot
	}
	return nil
}

type fakeProxyPublisher struct {
	published []string
}

func (p *fakeProxyPublisher) PublishAgent(agentID string) error {
	p.published = append(p.published, agentID)
	return nil
}

type fakeAgentLookup struct {
	agents map[string]*dto.AgentOutput
}

func (f *fakeAgentLookup) GetAgent(id string) (*dto.AgentOutput, error) {
	return f.agents[id], nil
}

var _ domain.Repository = (*fakeServiceCatalogRepo)(nil)
var _ domain.ProxyConfigPublisher = (*fakeProxyPublisher)(nil)
var _ domain.AgentLookup = (*fakeAgentLookup)(nil)

func newServiceSecretCodec() *secret.Codec {
	return secret.NewCodec(&config.ServiceSecretConfig{
		MasterKey:  []byte("12345678901234567890123456789012"),
		KeyVersion: "v1",
	})
}

func mustServiceEnvJSONB(env map[string]string) *commondb.JSONB[map[string]string] {
	return commondb.NewJSONB(env)
}

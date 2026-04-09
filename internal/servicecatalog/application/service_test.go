package application

import (
	"edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"testing"

	"github.com/google/uuid"
)

func TestCreateRejectsDuplicateRouteOnSameAgent(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	publisher := &fakeProxyPublisher{}
	svc := NewServiceWithPublisher(repo, publisher)

	first, err := svc.Create(dto.UpsertServiceRequest{
		Name:            "svc-a",
		ServiceKey:      "svc-a",
		AgentID:         "agent-a",
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
	if len(publisher.published) != 1 || publisher.published[0] != "agent-a" {
		t.Fatalf("expected publish for agent-a, got %#v", publisher.published)
	}

	_, err = svc.Create(dto.UpsertServiceRequest{
		Name:            "svc-b",
		ServiceKey:      "svc-b",
		AgentID:         "agent-a",
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
	svc := NewServiceWithPublisher(repo, nil)

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "agent-a",
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
		AgentID:       "agent-a",
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

var _ domain.Repository = (*fakeServiceCatalogRepo)(nil)
var _ domain.ProxyConfigPublisher = (*fakeProxyPublisher)(nil)

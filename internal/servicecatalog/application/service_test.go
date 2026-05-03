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

func TestBuildStickyBetaPath(t *testing.T) {
	for _, item := range []struct {
		prefix string
		want   string
	}{
		{prefix: "/", want: "/__ep/beta"},
		{prefix: "/v1", want: "/v1/__ep/beta"},
		{prefix: "v1/", want: "/v1/__ep/beta"},
	} {
		if got := BuildStickyBetaPath(item.prefix); got != item.want {
			t.Fatalf("BuildStickyBetaPath(%q) = %q, want %q", item.prefix, got, item.want)
		}
	}
}

func TestCreateNormalizesRouteHosts(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, nil)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "Primary.EXAMPLE.com",
		RouteHosts: []string{
			" primary.example.com ",
			"ALT.example.com",
			"alt.example.com",
			"",
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.RouteHost != "primary.example.com" {
		t.Fatalf("expected normalized primary host, got %q", created.RouteHost)
	}
	want := []string{"primary.example.com", "alt.example.com"}
	if len(created.RouteHosts) != len(want) {
		t.Fatalf("expected routeHosts %#v, got %#v", want, created.RouteHosts)
	}
	for i := range want {
		if created.RouteHosts[i] != want[i] {
			t.Fatalf("expected routeHosts %#v, got %#v", want, created.RouteHosts)
		}
	}
}

func TestCreateRouteHostsFallbackToRouteHost(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, nil)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "Single.EXAMPLE.com",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.RouteHost != "single.example.com" {
		t.Fatalf("expected normalized routeHost, got %q", created.RouteHost)
	}
	if len(created.RouteHosts) != 1 || created.RouteHosts[0] != "single.example.com" {
		t.Fatalf("expected single routeHosts fallback, got %#v", created.RouteHosts)
	}
}

func TestCreateRejectsDuplicateRouteHostFromRouteHosts(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, nil)

	_, err := svc.Create(dto.UpsertServiceRequest{
		Name:            "svc-a",
		ServiceKey:      "svc-a",
		AgentID:         "11111111-1111-1111-1111-111111111111",
		ImageRepo:       "repo/app",
		ContainerPort:   8080,
		RouteHost:       "api.example.com",
		RouteHosts:      []string{"api.example.com", "shared.example.com"},
		RoutePathPrefix: "/api",
	})
	if err != nil {
		t.Fatalf("Create() first error = %v", err)
	}

	_, err = svc.Create(dto.UpsertServiceRequest{
		Name:            "svc-b",
		ServiceKey:      "svc-b",
		AgentID:         "11111111-1111-1111-1111-111111111111",
		ImageRepo:       "repo/app",
		ContainerPort:   8080,
		RouteHost:       "other.example.com",
		RouteHosts:      []string{"other.example.com", "shared.example.com"},
		RoutePathPrefix: "/api",
	})
	if err == nil {
		t.Fatalf("expected duplicate route host validation error")
	}
}

func TestCreateServiceSchedulerSDKConfig(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	agents := &fakeAgentLookup{agents: map[string]*dto.AgentOutput{
		"11111111-1111-1111-1111-111111111111": {ID: "11111111-1111-1111-1111-111111111111", Enabled: boolPointer(true)},
	}}
	svc := NewServiceWithPublisher(repo, nil, agents)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:             "svc-scheduler",
		ServiceKey:       "svc-scheduler",
		AgentID:          "11111111-1111-1111-1111-111111111111",
		ImageRepo:        "repo/app",
		ContainerPort:    8080,
		SchedulerSDKPort: 19091,
		RouteHost:        "scheduler.example.com",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.SchedulerSDKPort != 19091 || created.SchedulerExecutorGroup != "default" {
		t.Fatalf("unexpected scheduler sdk config: port=%d group=%q", created.SchedulerSDKPort, created.SchedulerExecutorGroup)
	}

	spec, err := svc.GetSpecByID(created.ID)
	if err != nil {
		t.Fatalf("GetSpecByID() error = %v", err)
	}
	if spec.SchedulerSDKPort != 19091 || spec.SchedulerExecutorGroup != "default" {
		t.Fatalf("unexpected deployment scheduler sdk config: port=%d group=%q", spec.SchedulerSDKPort, spec.SchedulerExecutorGroup)
	}
}

func TestCreateServiceRejectsInvalidSchedulerSDKPort(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, nil)

	_, err := svc.Create(dto.UpsertServiceRequest{
		Name:             "svc-bad",
		ServiceKey:       "svc-bad",
		AgentID:          "11111111-1111-1111-1111-111111111111",
		ImageRepo:        "repo/app",
		ContainerPort:    8080,
		SchedulerSDKPort: 70000,
		RouteHost:        "bad.example.com",
	})
	if err == nil {
		t.Fatalf("expected invalid scheduler sdk port error")
	}
}

func TestCreateServiceWithResourceLimits(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, nil)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-limited",
		ServiceKey:    "svc-limited",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		CPULimitCores: 0.5,
		MemoryLimitMB: 256,
		RouteHost:     "limited.example.com",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.CPULimitCores != 0.5 || created.MemoryLimitMB != 256 {
		t.Fatalf("unexpected resource limits: cpu=%v memory=%d", created.CPULimitCores, created.MemoryLimitMB)
	}

	spec, err := svc.GetSpecByID(created.ID)
	if err != nil {
		t.Fatalf("GetSpecByID() error = %v", err)
	}
	if spec.CPULimitCores != 0.5 || spec.MemoryLimitMB != 256 {
		t.Fatalf("unexpected deployment resource limits: cpu=%v memory=%d", spec.CPULimitCores, spec.MemoryLimitMB)
	}
}

func TestCreateServiceRejectsInvalidResourceLimits(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, nil)

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-negative-cpu",
		ServiceKey:    "svc-negative-cpu",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		CPULimitCores: -0.1,
		RouteHost:     "a.example.com",
	}); err == nil {
		t.Fatal("expected negative cpuLimitCores to be rejected")
	}

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-negative-memory",
		ServiceKey:    "svc-negative-memory",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		MemoryLimitMB: -1,
		RouteHost:     "b.example.com",
	}); err == nil {
		t.Fatal("expected negative memoryLimitMB to be rejected")
	}
}

func TestCreateRejectsServiceKeyLongerThan24(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, nil)

	_, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-too-long",
		ServiceKey:    "svc-abcdefghijklmnopqrstuvwxyz",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "bad.example.com",
	})
	if err == nil {
		t.Fatalf("expected serviceKey length validation error")
	}
}

func TestUpdateRejectsServiceKeyLongerThan24(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewServiceWithPublisher(repo, nil, nil)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	enabled := true
	repo.byID[id] = &model.Service{
		ID:            id,
		ServiceKey:    "svc-abcdefghijklmnopqrstuvwxyz",
		Name:          "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
		Enabled:       &enabled,
	}
	repo.byKey["svc-abcdefghijklmnopqrstuvwxyz"] = repo.byID[id]

	_, err := svc.Update(id, dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-abcdefghijklmnopqrstuvwxyz",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
	})
	if err == nil {
		t.Fatalf("expected serviceKey length validation error")
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

func TestCreateNormalizesHTTPHealthHeaders(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:              "svc-a",
		ServiceKey:        "svc-a",
		AgentID:           "11111111-1111-1111-1111-111111111111",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		RouteHost:         "a.example.com",
		HTTPHealthHeaders: map[string]string{" X-Probe ": " token ", "": "ignored", "X-Empty": ""},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.HTTPHealthHeaders["X-Probe"] != "token" {
		t.Fatalf("expected normalized header in output, got %#v", created.HTTPHealthHeaders)
	}
	if len(created.HTTPHealthHeaders) != 1 {
		t.Fatalf("expected only valid headers to be retained, got %#v", created.HTTPHealthHeaders)
	}
}

func TestCreateNormalizesNetworkAliases(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:           "svc-a",
		ServiceKey:     "svc-a",
		AgentID:        "11111111-1111-1111-1111-111111111111",
		ImageRepo:      "repo/app",
		ContainerPort:  8080,
		RouteHost:      "a.example.com",
		NetworkAliases: []string{"svc-a", "  ", "svc-b", "svc-a"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.NetworkAliases) != 2 || created.NetworkAliases[0] != "svc-a" || created.NetworkAliases[1] != "svc-b" {
		t.Fatalf("expected normalized network aliases in output, got %#v", created.NetworkAliases)
	}
	spec, err := svc.GetSpecByID(created.ID)
	if err != nil {
		t.Fatalf("GetSpecByID() error = %v", err)
	}
	if len(spec.NetworkAliases) != 2 || spec.NetworkAliases[0] != "svc-a" || spec.NetworkAliases[1] != "svc-b" {
		t.Fatalf("expected normalized network aliases in deployment spec, got %#v", spec.NetworkAliases)
	}
}

func TestCreateRejectsInvalidNetworkAliases(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:           "svc-a",
		ServiceKey:     "svc-a",
		AgentID:        "11111111-1111-1111-1111-111111111111",
		ImageRepo:      "repo/app",
		ContainerPort:  8080,
		RouteHost:      "a.example.com",
		NetworkAliases: []string{"Svc-A"},
	}); err == nil {
		t.Fatal("expected uppercase network alias to be rejected")
	}

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:           "svc-b",
		ServiceKey:     "svc-b",
		AgentID:        "11111111-1111-1111-1111-111111111111",
		ImageRepo:      "repo/app",
		ContainerPort:  8080,
		RouteHost:      "b.example.com",
		NetworkAliases: []string{"svc.alias"},
	}); err == nil {
		t.Fatal("expected network alias with dot to be rejected")
	}
}

func TestUpdateNormalizesNetworkAliases(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := svc.Update(created.ID, dto.UpsertServiceRequest{
		Name:           "svc-a",
		ServiceKey:     "svc-a",
		AgentID:        "11111111-1111-1111-1111-111111111111",
		ImageRepo:      "repo/app",
		ContainerPort:  8080,
		RouteHost:      "a.example.com",
		NetworkAliases: []string{"svc-candidate", "svc-a", "svc-candidate"},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(updated.NetworkAliases) != 2 || updated.NetworkAliases[0] != "svc-candidate" || updated.NetworkAliases[1] != "svc-a" {
		t.Fatalf("expected normalized network aliases in update output, got %#v", updated.NetworkAliases)
	}
}

func TestUpdateRejectsImmutableFields(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := svc.Update(created.ID, dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a-new",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
	}); err == nil {
		t.Fatal("expected serviceKey update to be rejected")
	}

	if _, err := svc.Update(created.ID, dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "22222222-2222-2222-2222-222222222222",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
	}); err == nil {
		t.Fatal("expected agentId update to be rejected")
	}
}

func TestDeleteServicePublishesAgentSnapshot(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	publisher := &fakeProxyPublisher{}
	svc := NewServiceWithPublisher(repo, publisher, nil)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	publisher.published = nil
	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if repo.byID[created.ID] != nil {
		t.Fatalf("expected service to be deleted from repo")
	}
	if _, ok := repo.byKey[created.ServiceKey]; ok {
		t.Fatalf("expected service key index to be removed")
	}
	if len(publisher.published) != 1 || publisher.published[0] != created.AgentID {
		t.Fatalf("expected delete to publish agent snapshot, got %#v", publisher.published)
	}
}

func TestDeleteServiceRejectsWhenReleaseActive(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	publisher := &fakeProxyPublisher{}
	checker := &fakeReleaseStateChecker{active: true}
	svc := NewServiceWithPublisherAndCodecAndReleases(repo, publisher, nil, nil, checker)

	created, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 8080,
		RouteHost:     "a.example.com",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	publisher.published = nil
	if err := svc.Delete(created.ID); err == nil {
		t.Fatal("expected active release to block delete")
	}
	if repo.byID[created.ID] == nil {
		t.Fatalf("expected service to be preserved when delete blocked")
	}
	if len(publisher.published) != 0 {
		t.Fatalf("expected no publish when delete blocked, got %#v", publisher.published)
	}
}

func TestCreateRejectsInvalidContainerPort(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:          "svc-a",
		ServiceKey:    "svc-a",
		AgentID:       "11111111-1111-1111-1111-111111111111",
		ImageRepo:     "repo/app",
		ContainerPort: 0,
		RouteHost:     "a.example.com",
	}); err == nil {
		t.Fatal("expected invalid containerPort to be rejected")
	}
}

func TestCreateRejectsInvalidRoutePathPrefix(t *testing.T) {
	repo := newFakeServiceCatalogRepo()
	svc := NewService(repo)

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:            "svc-a",
		ServiceKey:      "svc-a",
		AgentID:         "11111111-1111-1111-1111-111111111111",
		ImageRepo:       "repo/app",
		ContainerPort:   8080,
		RouteHost:       "a.example.com",
		RoutePathPrefix: "/api;v1",
	}); err == nil {
		t.Fatal("expected invalid routePathPrefix with semicolon to be rejected")
	}

	if _, err := svc.Create(dto.UpsertServiceRequest{
		Name:            "svc-b",
		ServiceKey:      "svc-b",
		AgentID:         "11111111-1111-1111-1111-111111111111",
		ImageRepo:       "repo/app",
		ContainerPort:   8080,
		RouteHost:       "b.example.com",
		RoutePathPrefix: "/api\tv1",
	}); err == nil {
		t.Fatal("expected invalid routePathPrefix with whitespace to be rejected")
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

func (r *fakeServiceCatalogRepo) Delete(id uuid.UUID) error {
	item := r.byID[id]
	if item == nil {
		return nil
	}
	delete(r.byID, id)
	delete(r.byKey, item.ServiceKey)
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

type fakeReleaseStateChecker struct {
	active bool
	split  bool
}

func (f *fakeReleaseStateChecker) HasActiveRelease(uuid.UUID) (bool, error) {
	return f.active, nil
}

func (f *fakeReleaseStateChecker) HasTrafficSplitRelease(uuid.UUID) (bool, error) {
	return f.split, nil
}

func (f *fakeAgentLookup) GetAgent(id string) (*dto.AgentOutput, error) {
	return f.agents[id], nil
}

var _ domain.Repository = (*fakeServiceCatalogRepo)(nil)
var _ domain.ProxyConfigPublisher = (*fakeProxyPublisher)(nil)
var _ domain.AgentLookup = (*fakeAgentLookup)(nil)
var _ domain.ReleaseStateChecker = (*fakeReleaseStateChecker)(nil)

func newServiceSecretCodec() *secret.Codec {
	return secret.NewCodec(&config.ServiceSecretConfig{
		MasterKey:  []byte("12345678901234567890123456789012"),
		KeyVersion: "v1",
	})
}

func mustServiceEnvJSONB(env map[string]string) *commondb.JSONB[map[string]string] {
	return commondb.NewJSONB(env)
}

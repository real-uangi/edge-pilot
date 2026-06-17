package application

import (
	agentregistry "edge-pilot/internal/agent/application/registry"
	agentdomain "edge-pilot/internal/agent/domain"
	releasedomain "edge-pilot/internal/release/domain"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	servicecatalogdomain "edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"edge-pilot/internal/shared/secret"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
)

func TestCreateFromCICreatesQueuedReleaseWithoutDispatch(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service

	output, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.0.0",
		CommitSHA:  "commit-1",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() error = %v", err)
	}
	if output.Status != model.ReleaseStatusQueued {
		t.Fatalf("expected queued release, got %v", output.Status)
	}
	if output.QueuePosition != 1 {
		t.Fatalf("expected queue position 1, got %d", output.QueuePosition)
	}
	if len(dispatcher.tasks) != 0 {
		t.Fatalf("expected no dispatched task, got %d", len(dispatcher.tasks))
	}
	if len(releaseRepo.tasks) != 0 {
		t.Fatalf("expected no persisted task, got %d", len(releaseRepo.tasks))
	}
}

func TestCreateFromCIDeduplicatesSameImageRequest(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		HTTPHealthPath:    "/health",
		HTTPExpectedCode:  200,
		HTTPTimeoutSecond: 5,
		RouteHost:         "svc-a.example.com",
		RoutePathPrefix:   "/",
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service

	first, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.0.0",
		CommitSHA:  "commit-1",
		TraceID:    "trace-1",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() error = %v", err)
	}
	second, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.0.0",
		CommitSHA:  "commit-1",
		TraceID:    "trace-2",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() duplicate error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected duplicate callback to reuse release %s, got %s", first.ID, second.ID)
	}
	if len(releaseRepo.releases) != 1 {
		t.Fatalf("expected one release after dedupe, got %d", len(releaseRepo.releases))
	}
	if len(dispatcher.tasks) != 0 {
		t.Fatalf("expected no dispatched tasks on dedupe path, got %d", len(dispatcher.tasks))
	}
}

func TestCreateFromCIPopulatesVerificationAccessInfo(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
		DockerHealthCheck: &dockerHealth,
		RouteHost:         "svc-a.example.com",
		RoutePathPrefix:   "/",
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service

	output, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: service.ServiceKey,
		ImageTag:   "v1.0.0",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() error = %v", err)
	}
	if output.VerificationURL != "//svc-a.example.com/?__ep_release_id="+output.ID.String() {
		t.Fatalf("unexpected verification URL: %q", output.VerificationURL)
	}
	if output.StickyCookieName != "ep_release_id_svc_a" {
		t.Fatalf("unexpected sticky cookie name: %q", output.StickyCookieName)
	}
	if output.CurrentReleaseHeaderName != servicecatalogapp.CurrentReleaseIDHeaderName {
		t.Fatalf("unexpected current release header name: %q", output.CurrentReleaseHeaderName)
	}
	if output.LiveReleaseHeaderName != servicecatalogapp.LiveReleaseIDHeaderName {
		t.Fatalf("unexpected live release header name: %q", output.LiveReleaseHeaderName)
	}
	if output.ReleaseRoleHeaderName != servicecatalogapp.ReleaseRoleHeaderName {
		t.Fatalf("unexpected release role header name: %q", output.ReleaseRoleHeaderName)
	}
}

func TestCreateFromCIAllowsMultipleQueuedRequestsForDifferentImages(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service

	first, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.0.0",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() first error = %v", err)
	}
	second, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.1.0",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() second error = %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected different queued releases for different images")
	}
	if len(releaseRepo.releases) != 2 {
		t.Fatalf("expected two queued releases, got %d", len(releaseRepo.releases))
	}
}

func TestStartQueuedReleaseDispatchesDeployTask(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                     uuid.New(),
		ServiceKey:             "svc-a",
		Name:                   "svc-a",
		AgentID:                "agent-a",
		ImageRepo:              "repo/app",
		ContainerPort:          8080,
		CPULimitCores:          0.5,
		MemoryLimitMB:          256,
		SchedulerSDKPort:       19091,
		SchedulerExecutorGroup: "default",
		DockerHealthCheck:      &dockerHealth,
		HTTPHealthPath:         "/health",
		HTTPExpectedCode:       200,
		HTTPTimeoutSecond:      5,
		RouteHost:              "svc-a.example.com",
		RoutePathPrefix:        "/",
		NetworkAliases:         commondb.NewJSONB([]string{"svc-a", "svc-a-canary"}),
		Enabled:                &enabled,
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
		ImageRepo:  "repo/override",
		ImageTag:   "v1.0.0",
		TraceID:    "trace-1",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() error = %v", err)
	}

	started, err := releaseService.Start(queued.ID, "admin")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.Status != model.ReleaseStatusDispatching {
		t.Fatalf("expected dispatching release, got %v", started.Status)
	}
	if started.IsActive != true {
		t.Fatalf("expected active release after start")
	}
	if started.QueuePosition != 0 {
		t.Fatalf("expected queue position reset after start, got %d", started.QueuePosition)
	}
	if len(dispatcher.tasks) != 1 {
		t.Fatalf("expected one dispatched task, got %d", len(dispatcher.tasks))
	}
	payload := dispatcher.tasks[0].Payload.Get()
	if payload.ImageRepo != "repo/override" {
		t.Fatalf("expected queued image repo to be preserved, got %s", payload.ImageRepo)
	}
	if payload.SchedulerSDKPort != 19091 || payload.SchedulerExecutorGroup != "default" {
		t.Fatalf("unexpected scheduler sdk config in task payload: port=%d group=%q", payload.SchedulerSDKPort, payload.SchedulerExecutorGroup)
	}
	if len(payload.NetworkAliases) != 2 || payload.NetworkAliases[0] != "svc-a" || payload.NetworkAliases[1] != "svc-a-canary" {
		t.Fatalf("unexpected network aliases in task payload: %#v", payload.NetworkAliases)
	}
	if payload.CPULimitCores != 0.5 || payload.MemoryLimitMB != 256 {
		t.Fatalf("unexpected resource limits in task payload: cpu=%v memory=%d", payload.CPULimitCores, payload.MemoryLimitMB)
	}
}

func TestStartRejectsInvalidServiceContainerPort(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     0,
		DockerHealthCheck: &dockerHealth,
		RouteHost:         "svc-a.example.com",
		RoutePathPrefix:   "/",
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
	if _, err := releaseService.Start(queued.ID, "admin"); err == nil {
		t.Fatal("expected invalid service containerPort to be rejected")
	}
	if len(dispatcher.tasks) != 0 {
		t.Fatalf("expected no dispatched task, got %d", len(dispatcher.tasks))
	}
}

func TestStartQueuedReleaseEncryptsSensitiveTaskPayload(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	codec := newReleaseServiceSecretCodec()

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewServiceWithRegistryCredentialsAndCodec(releaseRepo, dispatcher, serviceCatalog, registry, nil, nil, codec)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		RouteHost:         "svc-a.example.com",
		Env:               commondb.NewJSONB(map[string]string{"A": "1"}),
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
		TraceID:    "trace-1",
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
	if len(payload.Env) != 0 {
		t.Fatalf("expected plaintext env to be removed from payload, got %#v", payload.Env)
	}
	if dispatcher.tasks[0].SensitiveCiphertext == "" || dispatcher.tasks[0].SensitiveKeyVersion == "" {
		t.Fatalf("expected encrypted sensitive payload, got %#v", dispatcher.tasks[0])
	}
	var sensitive model.TaskSensitivePayload
	if err := codec.DecryptJSON(dispatcher.tasks[0].SensitiveCiphertext, dispatcher.tasks[0].SensitiveKeyVersion, &sensitive); err != nil {
		t.Fatalf("DecryptJSON() error = %v", err)
	}
	if sensitive.Env["A"] != "1" {
		t.Fatalf("expected encrypted env payload, got %#v", sensitive)
	}
}

func TestStartQueuedReleaseAllowsLegacyPlaintextEnvWithoutCodec(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		RouteHost:         "svc-a.example.com",
		Env:               commondb.NewJSONB(map[string]string{"LEGACY": "1"}),
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
		TraceID:    "trace-1",
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
	if payload.Env["LEGACY"] != "1" {
		t.Fatalf("expected legacy plaintext env fallback, got %#v", payload.Env)
	}
	if dispatcher.tasks[0].SensitiveCiphertext != "" || dispatcher.tasks[0].SensitiveKeyVersion != "" {
		t.Fatalf("expected no sensitive ciphertext for legacy plaintext fallback, got %#v", dispatcher.tasks[0])
	}
}

func TestStartQueuedReleaseRecalculatesTargetSlotFromCurrentLiveSlot(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
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

	service.CurrentLiveSlot = model.SlotGreen

	started, err := releaseService.Start(queued.ID, "admin")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.PreviousLiveSlot != model.SlotGreen {
		t.Fatalf("expected previous live slot to refresh to green, got %v", started.PreviousLiveSlot)
	}
	if started.TargetSlot != model.SlotBlue {
		t.Fatalf("expected target slot to refresh to blue, got %v", started.TargetSlot)
	}
}

func TestStartQueuedReleaseRejectsWhenAnotherReleaseIsActive(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service

	activeRelease := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.0.0",
		Status:    model.ReleaseStatusDeploying,
	}
	queuedRelease := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.1.0",
		Status:    model.ReleaseStatusQueued,
	}
	if err := releaseRepo.CreateRelease(activeRelease); err != nil {
		t.Fatalf("CreateRelease() active error = %v", err)
	}
	if err := releaseRepo.CreateRelease(queuedRelease); err != nil {
		t.Fatalf("CreateRelease() queued error = %v", err)
	}

	if _, err := releaseService.Start(queuedRelease.ID, "admin"); err == nil {
		t.Fatalf("expected start to fail when another release is active")
	} else if !strings.Contains(err.Error(), "active release") {
		t.Fatalf("expected active release error message, got %q", err.Error())
	}
}

func TestStartQueuedReleaseRejectsTrafficSplitInProgress(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
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

	splitRelease := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		TrafficPercent:   30,
		SwitchConfirmed:  boolPointer(false),
	}
	queuedRelease := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.1.0",
		Status:    model.ReleaseStatusQueued,
	}
	if err := releaseRepo.CreateRelease(splitRelease); err != nil {
		t.Fatalf("CreateRelease() split error = %v", err)
	}
	if err := releaseRepo.CreateRelease(queuedRelease); err != nil {
		t.Fatalf("CreateRelease() queued error = %v", err)
	}

	_, err := releaseService.Start(queuedRelease.ID, "admin")
	if err == nil {
		t.Fatalf("expected start to fail when traffic split in progress")
	}
	var statusCarrier interface{ GetStatusCode() int }
	if !errors.As(err, &statusCarrier) || statusCarrier.GetStatusCode() != 409 {
		t.Fatalf("expected 409 error, got %v", err)
	}
	if !strings.Contains(err.Error(), "1-99") {
		t.Fatalf("expected traffic split error message, got %q", err.Error())
	}
}

func TestStartQueuedReleaseAllowsWhenExistingReleaseIsAt100Percent(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
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

	switched := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusSwitched,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		TrafficPercent:   100,
		SwitchConfirmed:  boolPointer(true),
	}
	queued := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.1.0",
		Status:    model.ReleaseStatusQueued,
	}
	if err := releaseRepo.CreateRelease(switched); err != nil {
		t.Fatalf("CreateRelease() switched error = %v", err)
	}
	if err := releaseRepo.CreateRelease(queued); err != nil {
		t.Fatalf("CreateRelease() queued error = %v", err)
	}

	started, err := releaseService.Start(queued.ID, "admin")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.Status != model.ReleaseStatusDispatching {
		t.Fatalf("expected dispatching status, got %v", started.Status)
	}
}

func TestStartQueuedReleaseAutoSkipsEarlierQueuedReleases(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
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

	older := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v0.9.0",
		Status:    model.ReleaseStatusQueued,
	}
	current := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.0.0",
		Status:    model.ReleaseStatusQueued,
	}
	if err := releaseRepo.CreateRelease(older); err != nil {
		t.Fatalf("CreateRelease() older error = %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := releaseRepo.CreateRelease(current); err != nil {
		t.Fatalf("CreateRelease() current error = %v", err)
	}

	if _, err := releaseService.Start(current.ID, "admin"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	storedOlder := releaseRepo.releases[older.ID]
	if storedOlder == nil || storedOlder.Status != model.ReleaseStatusSkipped {
		t.Fatalf("expected older queued release skipped, got %#v", storedOlder)
	}
	if storedOlder.CompletedAt == nil {
		t.Fatalf("expected older queued release completedAt set")
	}
	foundAutoSkippedAudit := false
	for _, item := range releaseRepo.audits {
		if item.EventType != "release_auto_skipped" {
			continue
		}
		if item.AggregateID == older.ID.String() && strings.Contains(item.Message, current.ID.String()) {
			foundAutoSkippedAudit = true
			break
		}
	}
	if !foundAutoSkippedAudit {
		t.Fatalf("expected release_auto_skipped audit for older release, got %#v", releaseRepo.audits)
	}
}

func TestStartQueuedReleaseDoesNotAutoSkipWhenResolveRegistryCredentialFails(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	resolver := fakeRegistryCredentialResolver{err: errors.New("resolve failed")}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
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
		ImageRepo:         "repo/app",
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

	older := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v0.9.0",
		Status:    model.ReleaseStatusQueued,
	}
	current := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.0.0",
		Status:    model.ReleaseStatusQueued,
	}
	if err := releaseRepo.CreateRelease(older); err != nil {
		t.Fatalf("CreateRelease() older error = %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := releaseRepo.CreateRelease(current); err != nil {
		t.Fatalf("CreateRelease() current error = %v", err)
	}

	if _, err := releaseService.Start(current.ID, "admin"); err == nil {
		t.Fatalf("expected start to fail when registry credential resolve fails")
	}
	storedOlder := releaseRepo.releases[older.ID]
	if storedOlder == nil || storedOlder.Status != model.ReleaseStatusQueued {
		t.Fatalf("expected older queued release to remain queued, got %#v", storedOlder)
	}
	if storedOlder.CompletedAt != nil {
		t.Fatalf("expected older queued release completedAt to remain nil")
	}
	for _, item := range releaseRepo.audits {
		if item.EventType == "release_auto_skipped" && item.AggregateID == older.ID.String() {
			t.Fatalf("expected no release_auto_skipped audit for older release when start failed")
		}
	}
}

func TestStartQueuedReleaseRejectsOfflineAgentAndKeepsQueued(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service

	queued, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.0.0",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() error = %v", err)
	}
	if _, err := releaseService.Start(queued.ID, "admin"); err == nil {
		t.Fatalf("expected start to fail when agent is offline")
	}
	stored := releaseRepo.releases[queued.ID]
	if stored == nil || stored.Status != model.ReleaseStatusQueued {
		t.Fatalf("expected queued release to remain queued")
	}
	if len(dispatcher.tasks) != 0 {
		t.Fatalf("expected no dispatched task on offline agent")
	}
}

func TestRetryFailedReleaseDispatchesDeployTask(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
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

	previousTaskID := uuid.New()
	completedAt := now.Add(-time.Minute)
	failed := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-obsolete",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		CommitSHA:        "commit-1",
		TraceID:          "trace-1",
		Status:           model.ReleaseStatusFailed,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		CurrentTaskID:    &previousTaskID,
		CompletedAt:      &completedAt,
	}
	if err := releaseRepo.CreateRelease(failed); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}

	service.CurrentLiveSlot = model.SlotGreen

	retried, err := releaseService.Retry(failed.ID, "admin")
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if retried.ID != failed.ID {
		t.Fatalf("expected retry to reuse release id %s, got %s", failed.ID, retried.ID)
	}
	if retried.Status != model.ReleaseStatusDispatching {
		t.Fatalf("expected dispatching status after retry, got %v", retried.Status)
	}
	if retried.CompletedAt != nil {
		t.Fatalf("expected completedAt reset after retry")
	}
	if retried.CurrentTaskID == nil {
		t.Fatalf("expected retry to create deploy task")
	}
	if *retried.CurrentTaskID == previousTaskID {
		t.Fatalf("expected retry task id to change")
	}
	if retried.PreviousLiveSlot != model.SlotGreen {
		t.Fatalf("expected refreshed previous live slot, got %v", retried.PreviousLiveSlot)
	}
	if retried.TargetSlot != model.SlotBlue {
		t.Fatalf("expected refreshed target slot, got %v", retried.TargetSlot)
	}
	if len(releaseRepo.releases) != 1 {
		t.Fatalf("expected retry to reuse existing release record")
	}
	if len(dispatcher.tasks) != 1 {
		t.Fatalf("expected one dispatched task, got %d", len(dispatcher.tasks))
	}
	task := releaseRepo.tasks[*retried.CurrentTaskID]
	if task == nil {
		t.Fatalf("expected persisted retry task")
	}
	if task.ReleaseID != failed.ID {
		t.Fatalf("expected task to belong to retried release, got %s", task.ReleaseID)
	}
	if task.Type != model.TaskTypeDeployGreen {
		t.Fatalf("expected deploy task on retry, got %v", task.Type)
	}
	if len(releaseRepo.audits) == 0 || releaseRepo.audits[len(releaseRepo.audits)-1].EventType != "release_retried" {
		t.Fatalf("expected release_retried audit, got %#v", releaseRepo.audits)
	}
}

func TestRetryRejectsNonFailedRelease(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	release := &model.Release{
		ID:        uuid.New(),
		ServiceID: uuid.New(),
		Status:    model.ReleaseStatusQueued,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}

	if _, err := releaseService.Retry(release.ID, "admin"); err == nil {
		t.Fatalf("expected retry to reject non-failed release")
	}
	if len(releaseRepo.tasks) != 0 {
		t.Fatalf("expected no task created when retry is rejected")
	}
}

func TestRetryRejectsWhenAnotherReleaseIsActive(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	serviceID := uuid.New()
	failed := &model.Release{
		ID:        uuid.New(),
		ServiceID: serviceID,
		Status:    model.ReleaseStatusFailed,
	}
	active := &model.Release{
		ID:        uuid.New(),
		ServiceID: serviceID,
		Status:    model.ReleaseStatusDeploying,
	}
	if err := releaseRepo.CreateRelease(failed); err != nil {
		t.Fatalf("CreateRelease() failed error = %v", err)
	}
	if err := releaseRepo.CreateRelease(active); err != nil {
		t.Fatalf("CreateRelease() active error = %v", err)
	}

	if _, err := releaseService.Retry(failed.ID, "admin"); err == nil {
		t.Fatalf("expected retry to fail when another release is active")
	}
	if len(releaseRepo.tasks) != 0 {
		t.Fatalf("expected no task created when retry is rejected by active release")
	}
	if releaseRepo.releases[failed.ID].Status != model.ReleaseStatusFailed {
		t.Fatalf("expected failed release status to remain unchanged")
	}
}

func TestSkipQueuedReleaseMarksSkipped(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
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
	skipped, err := releaseService.Skip(queued.ID, "admin")
	if err != nil {
		t.Fatalf("Skip() error = %v", err)
	}
	if skipped.Status != model.ReleaseStatusSkipped {
		t.Fatalf("expected skipped release, got %v", skipped.Status)
	}
	if skipped.CompletedAt == nil {
		t.Fatalf("expected skipped release completed time")
	}
	started, err := releaseService.Start(queued.ID, "admin")
	if err != nil {
		t.Fatalf("expected skipped release to allow start, got error = %v", err)
	}
	if started.Status != model.ReleaseStatusDispatching {
		t.Fatalf("expected dispatching status after re-starting skipped release, got %v", started.Status)
	}
	if started.CompletedAt != nil {
		t.Fatalf("expected completedAt reset after re-starting skipped release")
	}
	if len(dispatcher.tasks) != 1 {
		t.Fatalf("expected one dispatched task after re-starting skipped release, got %d", len(dispatcher.tasks))
	}
}

func TestStartSkippedReleaseRejectsWhenNewerSuccessfulReleaseExists(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
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

	skipped := &model.Release{
		ID:        uuid.New(),
		ServiceID: service.ID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.0.0",
		Status:    model.ReleaseStatusSkipped,
	}
	skipped.CreatedAt = now.Add(-2 * time.Minute)
	completedAt := now.Add(-time.Minute)
	newerSuccessful := &model.Release{
		ID:             uuid.New(),
		ServiceID:      service.ID,
		AgentID:        "agent-a",
		ImageRepo:      "repo/app",
		ImageTag:       "v1.1.0",
		Status:         model.ReleaseStatusCompleted,
		TrafficPercent: 100,
		CompletedAt:    &completedAt,
	}
	newerSuccessful.CreatedAt = now.Add(-time.Minute)
	for _, release := range []*model.Release{skipped, newerSuccessful} {
		if err := releaseRepo.CreateRelease(release); err != nil {
			t.Fatalf("CreateRelease() error = %v", err)
		}
	}

	if _, err := releaseService.Start(skipped.ID, "admin"); err == nil {
		t.Fatalf("expected skipped release to reject start when newer successful release exists")
	}
	if len(dispatcher.tasks) != 0 {
		t.Fatalf("expected no dispatched task when newer successful release exists, got %d", len(dispatcher.tasks))
	}
}

func TestListIncludesQueuePositionAndActiveFlag(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	serviceID := uuid.New()
	now := time.Now()
	releaseA := &model.Release{
		ID:        uuid.New(),
		ServiceID: serviceID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.0.0",
		Status:    model.ReleaseStatusQueued,
	}
	releaseA.CreatedAt = now.Add(-2 * time.Minute)
	releaseB := &model.Release{
		ID:        uuid.New(),
		ServiceID: serviceID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.1.0",
		Status:    model.ReleaseStatusQueued,
	}
	releaseB.CreatedAt = now.Add(-1 * time.Minute)
	activeRelease := &model.Release{
		ID:        uuid.New(),
		ServiceID: serviceID,
		AgentID:   "agent-a",
		ImageRepo: "repo/app",
		ImageTag:  "v1.2.0",
		Status:    model.ReleaseStatusDeploying,
	}
	activeRelease.CreatedAt = now
	for _, release := range []*model.Release{releaseA, releaseB, activeRelease} {
		if err := releaseRepo.CreateRelease(release); err != nil {
			t.Fatalf("CreateRelease() error = %v", err)
		}
	}

	items, err := releaseService.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	byID := make(map[uuid.UUID]dto.ReleaseOutput, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	if byID[releaseA.ID].QueuePosition != 1 {
		t.Fatalf("expected first queued position 1, got %d", byID[releaseA.ID].QueuePosition)
	}
	if byID[releaseB.ID].QueuePosition != 2 {
		t.Fatalf("expected second queued position 2, got %d", byID[releaseB.ID].QueuePosition)
	}
	if byID[activeRelease.ID].QueuePosition != 0 {
		t.Fatalf("expected active release queue position 0, got %d", byID[activeRelease.ID].QueuePosition)
	}
	if !byID[activeRelease.ID].IsActive {
		t.Fatalf("expected active release flag to be true")
	}
	if byID[releaseA.ID].IsActive {
		t.Fatalf("expected queued release not to be active")
	}
}

func TestHandleTaskUpdateMovesReleaseToReadyToSwitch(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	releaseID := uuid.New()
	serviceID := uuid.New()
	taskID := uuid.New()
	payload := model.TaskPayload{
		ServiceID:      serviceID,
		ServiceKey:     "svc-a",
		TargetSlot:     model.SlotGreen,
		PublishedPorts: []model.PublishedPort{{HostPort: 18081, ContainerPort: 8080}},
		ServerName:     "srv_green",
	}

	switchConfirmed := false
	releaseRepo.releases[releaseID] = &model.Release{
		ID:              releaseID,
		ServiceID:       serviceID,
		AgentID:         "agent-a",
		ImageTag:        "v2.0.0",
		TraceID:         "trace-1",
		Status:          model.ReleaseStatusDeploying,
		TargetSlot:      model.SlotGreen,
		CurrentTaskID:   &taskID,
		SwitchConfirmed: &switchConfirmed,
	}
	releaseRepo.tasks[taskID] = &model.Task{
		ID:        taskID,
		ReleaseID: releaseID,
		ServiceID: serviceID,
		AgentID:   "agent-a",
		Type:      model.TaskTypeDeployGreen,
		Status:    model.TaskStatusRunning,
		Payload:   mustJSONB(payload),
	}

	err := releaseService.HandleTaskUpdate("agent-a", &grpcapi.TaskUpdate{
		TaskId:        taskID.String(),
		Status:        grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
		Step:          "healthy",
		ContainerId:   "container-1",
		ListenAddress: "127.0.0.1:18081",
		Slot:          grpcapi.Slot_SLOT_GREEN,
		ServerName:    "srv_green",
	})
	if err != nil {
		t.Fatalf("HandleTaskUpdate() error = %v", err)
	}
	if releaseRepo.releases[releaseID].Status != model.ReleaseStatusReadyToSwitch {
		t.Fatalf("expected ready_to_switch, got %v", releaseRepo.releases[releaseID].Status)
	}
	if len(releaseRepo.runtimeByService[serviceID]) != 1 {
		t.Fatalf("expected one runtime instance")
	}
}

func TestRecoverAgentTasksReplaysOnlyMissingTasks(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	releaseID := uuid.New()
	serviceID := uuid.New()
	taskID := uuid.New()
	switchConfirmed := false
	releaseRepo.releases[releaseID] = &model.Release{
		ID:              releaseID,
		ServiceID:       serviceID,
		AgentID:         "agent-a",
		TraceID:         "trace-1",
		Status:          model.ReleaseStatusDispatching,
		CurrentTaskID:   &taskID,
		SwitchConfirmed: &switchConfirmed,
	}
	releaseRepo.tasks[taskID] = &model.Task{
		ID:        taskID,
		ReleaseID: releaseID,
		ServiceID: serviceID,
		AgentID:   "agent-a",
		Type:      model.TaskTypeDeployGreen,
		Status:    model.TaskStatusDispatched,
	}

	if err := releaseService.RecoverAgentTasks("agent-a", nil); err != nil {
		t.Fatalf("RecoverAgentTasks() error = %v", err)
	}
	if len(dispatcher.replayedTasks) != 1 {
		t.Fatalf("expected one replayed task, got %d", len(dispatcher.replayedTasks))
	}
	if releaseRepo.tasks[taskID].Status != model.TaskStatusDispatched {
		t.Fatalf("expected dispatched status after replay, got %v", releaseRepo.tasks[taskID].Status)
	}
}

func TestRecoverAgentTasksMarksRunningWhenHeartbeatReportsTask(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	releaseID := uuid.New()
	serviceID := uuid.New()
	taskID := uuid.New()
	switchConfirmed := false
	releaseRepo.releases[releaseID] = &model.Release{
		ID:              releaseID,
		ServiceID:       serviceID,
		AgentID:         "agent-a",
		TraceID:         "trace-2",
		Status:          model.ReleaseStatusDispatching,
		CurrentTaskID:   &taskID,
		SwitchConfirmed: &switchConfirmed,
	}
	releaseRepo.tasks[taskID] = &model.Task{
		ID:        taskID,
		ReleaseID: releaseID,
		ServiceID: serviceID,
		AgentID:   "agent-a",
		Type:      model.TaskTypeDeployGreen,
		Status:    model.TaskStatusDispatched,
	}

	if err := releaseService.RecoverAgentTasks("agent-a", []string{taskID.String()}); err != nil {
		t.Fatalf("RecoverAgentTasks() error = %v", err)
	}
	if len(dispatcher.replayedTasks) != 0 {
		t.Fatalf("expected no replay when task is reported running")
	}
	if releaseRepo.tasks[taskID].Status != model.TaskStatusRunning {
		t.Fatalf("expected running status, got %v", releaseRepo.tasks[taskID].Status)
	}
}

func TestFailStaleTasksMarksReleaseFailed(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	releaseID := uuid.New()
	serviceID := uuid.New()
	taskID := uuid.New()
	switchConfirmed := false
	releaseRepo.releases[releaseID] = &model.Release{
		ID:              releaseID,
		ServiceID:       serviceID,
		AgentID:         "agent-a",
		TraceID:         "trace-3",
		Status:          model.ReleaseStatusDispatching,
		CurrentTaskID:   &taskID,
		SwitchConfirmed: &switchConfirmed,
	}
	staleAt := time.Now().Add(-11 * time.Minute)
	releaseRepo.tasks[taskID] = &model.Task{
		ID:        taskID,
		ReleaseID: releaseID,
		ServiceID: serviceID,
		AgentID:   "agent-a",
		Type:      model.TaskTypeDeployGreen,
		Status:    model.TaskStatusRunning,
		Payload:   mustJSONB(model.TaskPayload{HTTPTimeoutSecond: 90, StartupGraceSecond: 15}),
	}
	releaseRepo.taskUpdatedAt[taskID] = staleAt

	if err := releaseService.FailStaleTasks(time.Now()); err != nil {
		t.Fatalf("FailStaleTasks() error = %v", err)
	}
	if releaseRepo.tasks[taskID].Status != model.TaskStatusTimedOut {
		t.Fatalf("expected timed out task, got %v", releaseRepo.tasks[taskID].Status)
	}
	if releaseRepo.releases[releaseID].Status != model.ReleaseStatusFailed {
		t.Fatalf("expected failed release, got %v", releaseRepo.releases[releaseID].Status)
	}
}

func TestHandleTaskUpdateIgnoresLateSucceededUpdateAfterTimeout(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	releaseID := uuid.New()
	serviceID := uuid.New()
	taskID := uuid.New()
	switchConfirmed := false
	now := time.Now()
	releaseRepo.releases[releaseID] = &model.Release{
		ID:              releaseID,
		ServiceID:       serviceID,
		AgentID:         "agent-a",
		TraceID:         "trace-late",
		Status:          model.ReleaseStatusFailed,
		CurrentTaskID:   &taskID,
		SwitchConfirmed: &switchConfirmed,
		CompletedAt:     &now,
	}
	releaseRepo.tasks[taskID] = &model.Task{
		ID:          taskID,
		ReleaseID:   releaseID,
		ServiceID:   serviceID,
		AgentID:     "agent-a",
		Type:        model.TaskTypeDeployGreen,
		Status:      model.TaskStatusTimedOut,
		CompletedAt: &now,
	}

	if err := releaseService.HandleTaskUpdate("agent-a", &grpcapi.TaskUpdate{
		TaskId: taskID.String(),
		Status: grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
		Step:   "healthy",
	}); err != nil {
		t.Fatalf("HandleTaskUpdate() error = %v", err)
	}
	if releaseRepo.releases[releaseID].Status != model.ReleaseStatusFailed {
		t.Fatalf("expected failed release to remain terminal, got %v", releaseRepo.releases[releaseID].Status)
	}
	if len(releaseRepo.audits) == 0 || releaseRepo.audits[len(releaseRepo.audits)-1].EventType != "late_task_update_ignored" {
		t.Fatalf("expected late_task_update_ignored audit, got %#v", releaseRepo.audits)
	}
}

func TestStartQueuedReleaseDefersDispatchWhenAgentSessionDrops(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{dispatchErr: releasedomain.ErrAgentOffline}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
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
	task := releaseRepo.tasks[*releaseRepo.releases[queued.ID].CurrentTaskID]
	if task == nil || task.Status != model.TaskStatusPending || task.LastStep != "dispatch_deferred" {
		t.Fatalf("expected deferred task to stay pending, got %#v", task)
	}
	foundDeferredAudit := false
	for _, item := range releaseRepo.audits {
		if item.EventType == "dispatch_deferred" {
			foundDeferredAudit = true
			break
		}
	}
	if !foundDeferredAudit {
		t.Fatalf("expected dispatch_deferred audit, got %#v", releaseRepo.audits)
	}
}

func TestStartQueuedReleaseMarksFailedWhenDispatchFails(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{dispatchErr: errors.New("stream broken")}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	online := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
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
	if _, err := releaseService.Start(queued.ID, "admin"); err == nil {
		t.Fatalf("expected Start() to return dispatch error")
	}
	storedRelease := releaseRepo.releases[queued.ID]
	if storedRelease == nil || storedRelease.Status != model.ReleaseStatusFailed || storedRelease.CompletedAt == nil {
		t.Fatalf("expected release failed after dispatch error, got %#v", storedRelease)
	}
	task := releaseRepo.tasks[*storedRelease.CurrentTaskID]
	if task == nil || task.Status != model.TaskStatusFailed || task.LastStep != "dispatch_failed" {
		t.Fatalf("expected failed task after dispatch error, got %#v", task)
	}
	foundAudit := false
	for _, item := range releaseRepo.audits {
		if item.EventType == "dispatch_failed" {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected dispatch_failed audit, got %#v", releaseRepo.audits)
	}
}

func TestSetTrafficPercentAdjustsLiveSlotAndStatus(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   0,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}
	healthy := true
	if err := releaseRepo.UpsertRuntimeInstance(&model.RuntimeInstance{
		ID:        uuid.New(),
		ServiceID: service.ID,
		ReleaseID: release.ID,
		Slot:      model.SlotGreen,
		Healthy:   &healthy,
	}); err != nil {
		t.Fatalf("UpsertRuntimeInstance(release) error = %v", err)
	}

	output, err := releaseService.SetTrafficPercent(release.ID, 30, "admin")
	if err != nil {
		t.Fatalf("SetTrafficPercent() error = %v", err)
	}
	if output.TrafficPercent != 30 || output.Status != model.ReleaseStatusReadyToSwitch {
		t.Fatalf("unexpected release output: %#v", output)
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != model.SlotBlue {
		t.Fatalf("expected live slot stay blue, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
}

func TestSetTrafficPercentZeroKeepsReadyToSwitch(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   30,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}
	healthy := true
	if err := releaseRepo.UpsertRuntimeInstance(&model.RuntimeInstance{
		ID:        uuid.New(),
		ServiceID: service.ID,
		ReleaseID: uuid.New(),
		Slot:      model.SlotBlue,
		Healthy:   &healthy,
	}); err != nil {
		t.Fatalf("UpsertRuntimeInstance(previous) error = %v", err)
	}

	output, err := releaseService.SetTrafficPercent(release.ID, 0, "admin")
	if err != nil {
		t.Fatalf("SetTrafficPercent() error = %v", err)
	}
	if output.TrafficPercent != 0 || output.Status != model.ReleaseStatusReadyToSwitch || output.CompletedAt != nil {
		t.Fatalf("expected 0%% traffic adjustment to keep ready-to-switch, got %#v", output)
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != model.SlotBlue {
		t.Fatalf("expected live slot stay blue, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
}

func TestSetTrafficPercentRejectsMissingTargetRuntime(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   0,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}

	if _, err := releaseService.SetTrafficPercent(release.ID, 100, "admin"); err == nil {
		t.Fatalf("expected missing target runtime to reject traffic switch")
	}
	stored := releaseRepo.releases[release.ID]
	if stored.Status != model.ReleaseStatusReadyToSwitch || stored.TrafficPercent != 0 {
		t.Fatalf("expected release unchanged after rejected switch, got %#v", stored)
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != model.SlotBlue {
		t.Fatalf("expected live slot unchanged, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
}

func TestConfirmSwitchCompletesReleaseAndClearsPreviousTraffic(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	previous := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v0.9.0",
		Status:           model.ReleaseStatusSwitched,
		TargetSlot:       model.SlotBlue,
		PreviousLiveSlot: model.SlotGreen,
		SwitchConfirmed:  boolPointer(true),
		TrafficPercent:   100,
	}
	if err := releaseRepo.CreateRelease(previous); err != nil {
		t.Fatalf("CreateRelease(previous) error = %v", err)
	}
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   0,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}
	healthy := true
	if err := releaseRepo.UpsertRuntimeInstance(&model.RuntimeInstance{
		ID:        uuid.New(),
		ServiceID: service.ID,
		ReleaseID: previous.ID,
		Slot:      model.SlotBlue,
		Healthy:   &healthy,
	}); err != nil {
		t.Fatalf("UpsertRuntimeInstance(previous) error = %v", err)
	}
	if err := releaseRepo.UpsertRuntimeInstance(&model.RuntimeInstance{
		ID:        uuid.New(),
		ServiceID: service.ID,
		ReleaseID: release.ID,
		Slot:      model.SlotGreen,
		Healthy:   &healthy,
	}); err != nil {
		t.Fatalf("UpsertRuntimeInstance(release) error = %v", err)
	}

	output, err := releaseService.ConfirmSwitch(release.ID, "admin")
	if err != nil {
		t.Fatalf("ConfirmSwitch() error = %v", err)
	}
	if output.TrafficPercent != 100 || output.Status != model.ReleaseStatusCompleted || output.CompletedAt == nil {
		t.Fatalf("unexpected release output: %#v", output)
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != model.SlotGreen {
		t.Fatalf("expected live slot switch to green, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
	storedPrevious := releaseRepo.releases[previous.ID]
	if storedPrevious == nil || storedPrevious.Status != model.ReleaseStatusCompleted || storedPrevious.TrafficPercent != 0 {
		t.Fatalf("expected previous release to be completed with zero traffic, got %#v", storedPrevious)
	}
}

func TestConfirmSwitchAllowsFirstReleaseWithoutRollbackTarget(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotBlue,
		PreviousLiveSlot: 0,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   0,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}
	healthy := true
	if err := releaseRepo.UpsertRuntimeInstance(&model.RuntimeInstance{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		ReleaseID:        release.ID,
		Slot:             model.SlotBlue,
		Healthy:          &healthy,
		AcceptingTraffic: boolPointer(false),
	}); err != nil {
		t.Fatalf("UpsertRuntimeInstance(release) error = %v", err)
	}

	output, err := releaseService.ConfirmSwitch(release.ID, "admin")
	if err != nil {
		t.Fatalf("ConfirmSwitch() error = %v", err)
	}
	if output.TrafficPercent != 100 || output.Status != model.ReleaseStatusCompleted || output.CompletedAt == nil {
		t.Fatalf("unexpected release output: %#v", output)
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != model.SlotBlue {
		t.Fatalf("expected live slot switch to blue, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
	currentInstance := releaseRepo.runtimeByService[service.ID][model.SlotBlue]
	if currentInstance == nil || currentInstance.AcceptingTraffic == nil || !*currentInstance.AcceptingTraffic {
		t.Fatalf("expected first runtime to accept traffic, got %#v", currentInstance)
	}
}

func TestSetTrafficPercentRejectsFirstReleasePartialTraffic(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotBlue,
		PreviousLiveSlot: 0,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   0,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}

	if _, err := releaseService.SetTrafficPercent(release.ID, 30, "admin"); err == nil || !strings.Contains(err.Error(), "release has no live baseline for traffic split") {
		t.Fatalf("expected missing live baseline error, got %v", err)
	}
	stored := releaseRepo.releases[release.ID]
	if stored.Status != model.ReleaseStatusReadyToSwitch || stored.TrafficPercent != 0 || stored.CompletedAt != nil {
		t.Fatalf("expected release unchanged after rejected first split, got %#v", stored)
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != 0 {
		t.Fatalf("expected live slot unchanged, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
}

func TestRollbackRejectsFirstReleaseWithoutRollbackTarget(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotBlue,
		PreviousLiveSlot: 0,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   0,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}

	if _, err := releaseService.Rollback(release.ID, "admin"); err == nil || !strings.Contains(err.Error(), "release has no rollback target") {
		t.Fatalf("expected missing rollback target error, got %v", err)
	}
	stored := releaseRepo.releases[release.ID]
	if stored.Status != model.ReleaseStatusReadyToSwitch || stored.TrafficPercent != 0 || stored.CompletedAt != nil {
		t.Fatalf("expected release unchanged after rejected first rollback, got %#v", stored)
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != 0 {
		t.Fatalf("expected live slot unchanged, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
}

func TestRollbackCompletesReleaseAndRestoresPreviousTraffic(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotGreen,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   30,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}
	healthy := true
	if err := releaseRepo.UpsertRuntimeInstance(&model.RuntimeInstance{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		ReleaseID:        uuid.New(),
		Slot:             model.SlotBlue,
		Healthy:          &healthy,
		AcceptingTraffic: boolPointer(false),
	}); err != nil {
		t.Fatalf("UpsertRuntimeInstance(previous) error = %v", err)
	}
	if err := releaseRepo.UpsertRuntimeInstance(&model.RuntimeInstance{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		ReleaseID:        release.ID,
		Slot:             model.SlotGreen,
		Healthy:          &healthy,
		AcceptingTraffic: boolPointer(true),
	}); err != nil {
		t.Fatalf("UpsertRuntimeInstance(release) error = %v", err)
	}

	output, err := releaseService.Rollback(release.ID, "admin")
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if output.TrafficPercent != 0 || output.Status != model.ReleaseStatusRolledBack {
		t.Fatalf("unexpected release output: %#v", output)
	}
	if output.CompletedAt == nil {
		t.Fatalf("expected rollback to set completedAt")
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != model.SlotBlue {
		t.Fatalf("expected live slot rollback to blue, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
	previousInstance := releaseRepo.runtimeByService[service.ID][model.SlotBlue]
	if previousInstance == nil || previousInstance.AcceptingTraffic == nil || !*previousInstance.AcceptingTraffic {
		t.Fatalf("expected previous runtime to accept traffic, got %#v", previousInstance)
	}
	currentInstance := releaseRepo.runtimeByService[service.ID][model.SlotGreen]
	if currentInstance == nil || currentInstance.AcceptingTraffic == nil || *currentInstance.AcceptingTraffic {
		t.Fatalf("expected current runtime to stop accepting traffic, got %#v", currentInstance)
	}
}

func TestRollbackRejectsMissingPreviousRuntime(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	dockerHealth := true
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotGreen,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "v1.0.0",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   30,
	}
	if err := releaseRepo.CreateRelease(release); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}

	if _, err := releaseService.Rollback(release.ID, "admin"); err == nil {
		t.Fatalf("expected missing previous runtime to reject rollback")
	}
	stored := releaseRepo.releases[release.ID]
	if stored.Status != model.ReleaseStatusReadyToSwitch || stored.TrafficPercent != 30 || stored.CompletedAt != nil {
		t.Fatalf("expected release unchanged after rejected rollback, got %#v", stored)
	}
	if serviceRepo.byID[service.ID].CurrentLiveSlot != model.SlotGreen {
		t.Fatalf("expected live slot unchanged, got %v", serviceRepo.byID[service.ID].CurrentLiveSlot)
	}
}

func TestStartCompletesSupersededReleaseWhenSlotIsReused(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}
	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentregistry.NewRegistryService(config.LoadAgentAuthConfig(), agentRepo)
	releaseService := NewService(releaseRepo, dispatcher, serviceCatalog, registry)

	enabled := true
	online := true
	dockerHealth := true
	now := time.Now()
	service := &model.Service{
		ID:                uuid.New(),
		ServiceKey:        "svc-a",
		Name:              "svc-a",
		AgentID:           "agent-a",
		ImageRepo:         "repo/app",
		ContainerPort:     8080,
		CurrentLiveSlot:   model.SlotBlue,
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

	superseded := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "old",
		Status:           model.ReleaseStatusReadyToSwitch,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   0,
	}
	if err := releaseRepo.CreateRelease(superseded); err != nil {
		t.Fatalf("CreateRelease(superseded) error = %v", err)
	}
	queued := &model.Release{
		ID:               uuid.New(),
		ServiceID:        service.ID,
		AgentID:          "agent-a",
		ImageRepo:        "repo/app",
		ImageTag:         "new",
		Status:           model.ReleaseStatusQueued,
		TargetSlot:       model.SlotGreen,
		PreviousLiveSlot: model.SlotBlue,
		SwitchConfirmed:  boolPointer(false),
		TrafficPercent:   0,
	}
	if err := releaseRepo.CreateRelease(queued); err != nil {
		t.Fatalf("CreateRelease(queued) error = %v", err)
	}

	if _, err := releaseService.Start(queued.ID, "admin"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	stored := releaseRepo.releases[superseded.ID]
	if stored == nil || stored.Status != model.ReleaseStatusCompleted || stored.CompletedAt == nil {
		t.Fatalf("expected superseded release completed, got %#v", stored)
	}
}

type fakeServiceRepo struct {
	byID  map[uuid.UUID]*model.Service
	byKey map[string]*model.Service
}

func (r *fakeServiceRepo) ensure() {
	if r.byID == nil {
		r.byID = make(map[uuid.UUID]*model.Service)
	}
	if r.byKey == nil {
		r.byKey = make(map[string]*model.Service)
	}
}

func (r *fakeServiceRepo) Create(service *model.Service) error {
	r.ensure()
	r.byID[service.ID] = service
	r.byKey[service.ServiceKey] = service
	return nil
}

func (r *fakeServiceRepo) Update(service *model.Service) error {
	r.ensure()
	r.byID[service.ID] = service
	r.byKey[service.ServiceKey] = service
	return nil
}

func (r *fakeServiceRepo) Delete(id uuid.UUID) error {
	r.ensure()
	item := r.byID[id]
	if item == nil {
		return nil
	}
	delete(r.byID, id)
	delete(r.byKey, item.ServiceKey)
	return nil
}

func (r *fakeServiceRepo) GetByID(id uuid.UUID) (*model.Service, error) {
	r.ensure()
	return r.byID[id], nil
}

func (r *fakeServiceRepo) GetByKey(key string) (*model.Service, error) {
	r.ensure()
	return r.byKey[key], nil
}

func (r *fakeServiceRepo) GetByRoute(agentID string, routeHost string, routePathPrefix string) (*model.Service, error) {
	r.ensure()
	for _, item := range r.byID {
		if item.AgentID == agentID && item.RouteHost == routeHost && item.RoutePathPrefix == routePathPrefix {
			return item, nil
		}
	}
	return nil, nil
}

func (r *fakeServiceRepo) List() ([]model.Service, error) {
	r.ensure()
	out := make([]model.Service, 0, len(r.byID))
	for _, item := range r.byID {
		out = append(out, *item)
	}
	return out, nil
}

func (r *fakeServiceRepo) ListByAgent(agentID string) ([]model.Service, error) {
	r.ensure()
	out := make([]model.Service, 0, len(r.byID))
	for _, item := range r.byID {
		if item.AgentID == agentID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (r *fakeServiceRepo) UpdateLiveSlot(id uuid.UUID, slot model.Slot) error {
	r.ensure()
	if item := r.byID[id]; item != nil {
		item.CurrentLiveSlot = slot
	}
	return nil
}

var _ servicecatalogdomain.Repository = (*fakeServiceRepo)(nil)

type fakeAgentRepo struct {
	nodes map[string]*model.AgentNode
}

func (r *fakeAgentRepo) Save(node *model.AgentNode) error {
	if r.nodes == nil {
		r.nodes = make(map[string]*model.AgentNode)
	}
	copyNode := *node
	r.nodes[node.ID] = &copyNode
	return nil
}

func (r *fakeAgentRepo) Get(id string) (*model.AgentNode, error) {
	if r.nodes == nil {
		return nil, nil
	}
	node := r.nodes[id]
	if node == nil {
		return nil, nil
	}
	copyNode := *node
	return &copyNode, nil
}

func (r *fakeAgentRepo) Delete(id string) error {
	if r.nodes != nil {
		delete(r.nodes, id)
	}
	return nil
}

func (r *fakeAgentRepo) List() ([]model.AgentNode, error) {
	out := make([]model.AgentNode, 0, len(r.nodes))
	for _, item := range r.nodes {
		out = append(out, *item)
	}
	return out, nil
}

func (r *fakeAgentRepo) ListEnabled() ([]model.AgentNode, error) {
	out := make([]model.AgentNode, 0, len(r.nodes))
	for _, item := range r.nodes {
		if item != nil && item.Enabled != nil && *item.Enabled {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (r *fakeAgentRepo) MarkOffline(id string, reason string) error {
	if node := r.nodes[id]; node != nil {
		offline := false
		node.Online = &offline
		node.LastError = reason
	}
	return nil
}

func (r *fakeAgentRepo) MarkOfflineStale(before time.Time) ([]string, error) {
	var ids []string
	for id, node := range r.nodes {
		if node == nil || node.Online == nil || !*node.Online || node.LastHeartbeatAt == nil {
			continue
		}
		if node.LastHeartbeatAt.Before(before) {
			offline := false
			node.Online = &offline
			node.LastError = "heartbeat timeout"
			ids = append(ids, id)
		}
	}
	return ids, nil
}

var _ agentdomain.Repository = (*fakeAgentRepo)(nil)

type fakeReleaseRepo struct {
	releases         map[uuid.UUID]*model.Release
	tasks            map[uuid.UUID]*model.Task
	taskUpdatedAt    map[uuid.UUID]time.Time
	taskAttempts     []*model.TaskAttempt
	audits           []*model.AuditLog
	runtimeByService map[uuid.UUID]map[model.Slot]*model.RuntimeInstance
}

func newFakeReleaseRepo() *fakeReleaseRepo {
	return &fakeReleaseRepo{
		releases:         make(map[uuid.UUID]*model.Release),
		tasks:            make(map[uuid.UUID]*model.Task),
		taskUpdatedAt:    make(map[uuid.UUID]time.Time),
		runtimeByService: make(map[uuid.UUID]map[model.Slot]*model.RuntimeInstance),
	}
}

func (r *fakeReleaseRepo) CreateRelease(release *model.Release) error {
	copyRelease := *release
	now := time.Now()
	if copyRelease.CreatedAt.IsZero() {
		copyRelease.CreatedAt = now
	}
	copyRelease.UpdatedAt = copyRelease.CreatedAt
	r.releases[release.ID] = &copyRelease
	return nil
}

func (r *fakeReleaseRepo) UpdateRelease(release *model.Release) error {
	copyRelease := *release
	if copyRelease.CreatedAt.IsZero() {
		copyRelease.CreatedAt = time.Now()
	}
	copyRelease.UpdatedAt = time.Now()
	r.releases[release.ID] = &copyRelease
	return nil
}

func (r *fakeReleaseRepo) GetRelease(id uuid.UUID) (*model.Release, error) {
	if item := r.releases[id]; item != nil {
		copyRelease := *item
		return &copyRelease, nil
	}
	return nil, nil
}

func (r *fakeReleaseRepo) ListReleases(limit int) ([]model.Release, error) {
	out := make([]model.Release, 0, len(r.releases))
	for _, item := range r.releases {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeReleaseRepo) ListQueuedBefore(serviceID uuid.UUID, createdAt time.Time, releaseID uuid.UUID) ([]model.Release, error) {
	out := make([]model.Release, 0)
	for _, item := range r.releases {
		if item.ID == releaseID || item.ServiceID != serviceID || item.Status != model.ReleaseStatusQueued {
			continue
		}
		if item.CreatedAt.Before(createdAt) {
			out = append(out, *item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (r *fakeReleaseRepo) FindReadyToSwitchRelease(serviceID uuid.UUID) (*model.Release, error) {
	var matched []*model.Release
	for _, item := range r.releases {
		if item.ServiceID != serviceID || item.Status != model.ReleaseStatusReadyToSwitch {
			continue
		}
		copyRelease := *item
		matched = append(matched, &copyRelease)
	}
	if len(matched) == 0 {
		return nil, nil
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
	})
	return matched[0], nil
}

func (r *fakeReleaseRepo) HasActiveRelease(serviceID uuid.UUID) (bool, error) {
	for _, item := range r.releases {
		if item.ServiceID != serviceID {
			continue
		}
		if item.Status == model.ReleaseStatusDispatching || item.Status == model.ReleaseStatusDeploying {
			return true, nil
		}
		if (item.Status == model.ReleaseStatusReadyToSwitch || item.Status == model.ReleaseStatusSwitched) &&
			item.TrafficPercent >= 1 && item.TrafficPercent <= 99 {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeReleaseRepo) HasTrafficSplitRelease(serviceID uuid.UUID) (bool, error) {
	for _, item := range r.releases {
		if item.ServiceID != serviceID {
			continue
		}
		if (item.Status == model.ReleaseStatusReadyToSwitch || item.Status == model.ReleaseStatusSwitched) &&
			item.TrafficPercent >= 1 && item.TrafficPercent <= 99 {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeReleaseRepo) HasNewerSuccessfulRelease(serviceID uuid.UUID, createdAt time.Time) (bool, error) {
	for _, item := range r.releases {
		if item.ServiceID != serviceID {
			continue
		}
		if item.Status != model.ReleaseStatusCompleted && item.Status != model.ReleaseStatusSwitched {
			continue
		}
		if item.TrafficPercent != 100 {
			continue
		}
		if item.CreatedAt.After(createdAt) {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeReleaseRepo) FindQueuedOrActiveDuplicate(serviceID uuid.UUID, imageTag string, commitSHA string) (*model.Release, error) {
	var matched []*model.Release
	for _, item := range r.releases {
		if item.ServiceID != serviceID || item.ImageTag != imageTag {
			continue
		}
		if !(item.Status.IsQueued() || item.Status.IsActive()) {
			continue
		}
		if commitSHA != "" && item.CommitSHA != commitSHA {
			continue
		}
		copyRelease := *item
		matched = append(matched, &copyRelease)
	}
	if len(matched) == 0 {
		return nil, nil
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.Before(matched[j].CreatedAt)
	})
	return matched[0], nil
}

func (r *fakeReleaseRepo) CountQueuedBefore(serviceID uuid.UUID, createdAt time.Time, releaseID uuid.UUID) (int, error) {
	count := 0
	for _, item := range r.releases {
		if item.ID == releaseID || item.ServiceID != serviceID || item.Status != model.ReleaseStatusQueued {
			continue
		}
		if item.CreatedAt.Before(createdAt) {
			count++
		}
	}
	return count, nil
}

func (r *fakeReleaseRepo) CreateTask(task *model.Task) error {
	copyTask := *task
	now := time.Now()
	if copyTask.CreatedAt.IsZero() {
		copyTask.CreatedAt = now
	}
	copyTask.UpdatedAt = copyTask.CreatedAt
	r.tasks[task.ID] = &copyTask
	r.taskUpdatedAt[task.ID] = copyTask.UpdatedAt
	return nil
}

func (r *fakeReleaseRepo) UpdateTask(task *model.Task) error {
	copyTask := *task
	if copyTask.CreatedAt.IsZero() {
		copyTask.CreatedAt = time.Now()
	}
	copyTask.UpdatedAt = time.Now()
	r.tasks[task.ID] = &copyTask
	r.taskUpdatedAt[task.ID] = copyTask.UpdatedAt
	return nil
}

func (r *fakeReleaseRepo) GetTask(id uuid.UUID) (*model.Task, error) {
	if item := r.tasks[id]; item != nil {
		copyTask := *item
		return &copyTask, nil
	}
	return nil, nil
}

func (r *fakeReleaseRepo) ListTasksByRelease(releaseID uuid.UUID) ([]model.Task, error) {
	out := make([]model.Task, 0)
	for _, item := range r.tasks {
		if item.ReleaseID == releaseID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (r *fakeReleaseRepo) ListRecoverableTasksByAgent(agentID string) ([]model.Task, error) {
	out := make([]model.Task, 0)
	for _, task := range r.tasks {
		if task.AgentID != agentID {
			continue
		}
		release := r.releases[task.ReleaseID]
		if release == nil || release.CurrentTaskID == nil || *release.CurrentTaskID != task.ID {
			continue
		}
		if task.Status == model.TaskStatusPending || task.Status == model.TaskStatusDispatched || task.Status == model.TaskStatusRunning {
			out = append(out, *task)
		}
	}
	return out, nil
}

func (r *fakeReleaseRepo) ListActiveTasks() ([]model.Task, error) {
	out := make([]model.Task, 0)
	for _, task := range r.tasks {
		release := r.releases[task.ReleaseID]
		if release == nil || release.CurrentTaskID == nil || *release.CurrentTaskID != task.ID {
			continue
		}
		if !(task.Status == model.TaskStatusPending || task.Status == model.TaskStatusDispatched || task.Status == model.TaskStatusRunning) {
			continue
		}
		copyTask := *task
		if updatedAt, ok := r.taskUpdatedAt[task.ID]; ok {
			copyTask.UpdatedAt = updatedAt
		}
		out = append(out, copyTask)
	}
	return out, nil
}

func (r *fakeReleaseRepo) CreateTaskAttempt(attempt *model.TaskAttempt) error {
	copyAttempt := *attempt
	r.taskAttempts = append(r.taskAttempts, &copyAttempt)
	return nil
}

func (r *fakeReleaseRepo) UpsertRuntimeInstance(instance *model.RuntimeInstance) error {
	if r.runtimeByService[instance.ServiceID] == nil {
		r.runtimeByService[instance.ServiceID] = make(map[model.Slot]*model.RuntimeInstance)
	}
	copyInstance := *instance
	r.runtimeByService[instance.ServiceID][instance.Slot] = &copyInstance
	return nil
}

func (r *fakeReleaseRepo) GetRuntimeInstanceByServiceAndSlot(serviceID uuid.UUID, slot model.Slot) (*model.RuntimeInstance, error) {
	item := r.runtimeByService[serviceID][slot]
	if item == nil {
		return nil, nil
	}
	copyInstance := *item
	return &copyInstance, nil
}

func (r *fakeReleaseRepo) ListRuntimeInstancesByService(serviceID uuid.UUID) ([]model.RuntimeInstance, error) {
	items := r.runtimeByService[serviceID]
	out := make([]model.RuntimeInstance, 0, len(items))
	for _, item := range items {
		out = append(out, *item)
	}
	return out, nil
}

func (r *fakeReleaseRepo) CreateAudit(log *model.AuditLog) error {
	copyLog := *log
	r.audits = append(r.audits, &copyLog)
	return nil
}

func (r *fakeReleaseRepo) ListTaskAttemptsByTask(taskID uuid.UUID) ([]model.TaskAttempt, error) {
	return nil, nil
}

func (r *fakeReleaseRepo) ListAuditsByAggregate(aggregateType string, aggregateID string) ([]model.AuditLog, error) {
	out := make([]model.AuditLog, 0, len(r.audits))
	for _, a := range r.audits {
		out = append(out, *a)
	}
	return out, nil
}

type fakeDispatcher struct {
	tasks         []*model.Task
	replayedTasks []*model.Task
	dispatchErr   error
}

func (d *fakeDispatcher) DispatchTask(agentID string, task *model.Task) error {
	copyTask := *task
	d.tasks = append(d.tasks, &copyTask)
	return d.dispatchErr
}

func (d *fakeDispatcher) ReplayTask(agentID string, task *model.Task) (bool, error) {
	copyTask := *task
	d.replayedTasks = append(d.replayedTasks, &copyTask)
	return true, nil
}

func mustJSONB(payload model.TaskPayload) *commondb.JSONB[model.TaskPayload] {
	return commondb.NewJSONB(payload)
}

func newReleaseServiceSecretCodec() *secret.Codec {
	return secret.NewCodec(&config.ServiceSecretConfig{
		MasterKey:  []byte("12345678901234567890123456789012"),
		KeyVersion: "v1",
	})
}

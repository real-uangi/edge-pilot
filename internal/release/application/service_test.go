package application

import (
	agentapp "edge-pilot/internal/agent/application"
	agentdomain "edge-pilot/internal/agent/domain"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	servicecatalogdomain "edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"sort"
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
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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

func TestCreateFromCIAllowsMultipleQueuedRequestsForDifferentImages(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	agentRepo.nodes["agent-a"] = &model.AgentNode{
		ID:              "agent-a",
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
}

func TestStartQueuedReleaseRecalculatesTargetSlotFromCurrentLiveSlot(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{nodes: map[string]*model.AgentNode{}}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	}
}

func TestStartQueuedReleaseRejectsOfflineAgentAndKeepsQueued(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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

func TestSkipQueuedReleaseMarksSkipped(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	if _, err := releaseService.Start(queued.ID, "admin"); err == nil {
		t.Fatalf("expected skipped release to reject start")
	}
}

func TestListIncludesQueuePositionAndActiveFlag(t *testing.T) {
	serviceRepo := &fakeServiceRepo{}
	agentRepo := &fakeAgentRepo{}
	releaseRepo := newFakeReleaseRepo()
	dispatcher := &fakeDispatcher{}

	serviceCatalog := servicecatalogapp.NewService(serviceRepo)
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	registry := agentapp.NewRegistryService(&config.AgentAuthConfig{SharedToken: "token"}, agentRepo)
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
	}
	releaseRepo.taskUpdatedAt[taskID] = staleAt

	if err := releaseService.FailStaleTasks(time.Now().Add(-10 * time.Minute)); err != nil {
		t.Fatalf("FailStaleTasks() error = %v", err)
	}
	if releaseRepo.tasks[taskID].Status != model.TaskStatusTimedOut {
		t.Fatalf("expected timed out task, got %v", releaseRepo.tasks[taskID].Status)
	}
	if releaseRepo.releases[releaseID].Status != model.ReleaseStatusFailed {
		t.Fatalf("expected failed release, got %v", releaseRepo.releases[releaseID].Status)
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

func (r *fakeAgentRepo) List() ([]model.AgentNode, error) {
	out := make([]model.AgentNode, 0, len(r.nodes))
	for _, item := range r.nodes {
		out = append(out, *item)
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

func (r *fakeReleaseRepo) HasActiveRelease(serviceID uuid.UUID) (bool, error) {
	for _, item := range r.releases {
		if item.ServiceID == serviceID && item.Status.IsActive() {
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

func (r *fakeReleaseRepo) ListStaleTasks(before time.Time) ([]model.Task, error) {
	out := make([]model.Task, 0)
	for _, task := range r.tasks {
		release := r.releases[task.ReleaseID]
		if release == nil || release.CurrentTaskID == nil || *release.CurrentTaskID != task.ID {
			continue
		}
		if !(task.Status == model.TaskStatusPending || task.Status == model.TaskStatusDispatched || task.Status == model.TaskStatusRunning) {
			continue
		}
		if updatedAt, ok := r.taskUpdatedAt[task.ID]; ok && updatedAt.Before(before) {
			out = append(out, *task)
		}
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

type fakeDispatcher struct {
	tasks         []*model.Task
	replayedTasks []*model.Task
}

func (d *fakeDispatcher) DispatchTask(agentID string, task *model.Task) error {
	copyTask := *task
	d.tasks = append(d.tasks, &copyTask)
	return nil
}

func (d *fakeDispatcher) ReplayTask(agentID string, task *model.Task) (bool, error) {
	copyTask := *task
	d.replayedTasks = append(d.replayedTasks, &copyTask)
	return true, nil
}

func mustJSONB(payload model.TaskPayload) *commondb.JSONB[model.TaskPayload] {
	return commondb.NewJSONB(payload)
}

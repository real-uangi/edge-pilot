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
	"testing"
	"time"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
)

func TestCreateFromCIRejectsOfflineAgent(t *testing.T) {
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
		BlueHostPort:      18080,
		GreenHostPort:     18081,
		DockerHealthCheck: &dockerHealth,
		Enabled:           &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service

	_, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.0.0",
	})
	if err == nil {
		t.Fatalf("expected error when agent is offline")
	}
}

func TestCreateFromCIDispatchesDeployTask(t *testing.T) {
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
		ID:                 uuid.New(),
		ServiceKey:         "svc-a",
		Name:               "svc-a",
		AgentID:            "agent-a",
		ImageRepo:          "repo/app",
		ContainerPort:      8080,
		BlueHostPort:       18080,
		GreenHostPort:      18081,
		DockerHealthCheck:  &dockerHealth,
		HTTPHealthPath:     "/health",
		HTTPExpectedCode:   200,
		HTTPTimeoutSecond:  5,
		HAProxyBackend:     "be_api",
		HAProxyBlueServer:  "srv_blue",
		HAProxyGreenServer: "srv_green",
		Enabled:            &enabled,
	}
	serviceRepo.ensure()
	serviceRepo.byID[service.ID] = service
	serviceRepo.byKey[service.ServiceKey] = service
	agentRepo.nodes["agent-a"] = &model.AgentNode{
		ID:              "agent-a",
		Online:          &online,
		LastHeartbeatAt: &now,
	}

	output, err := releaseService.CreateFromCI(dto.CreateReleaseFromCIRequest{
		ServiceKey: "svc-a",
		ImageTag:   "v1.0.0",
		TraceID:    "trace-1",
	})
	if err != nil {
		t.Fatalf("CreateFromCI() error = %v", err)
	}
	if output.Status != model.ReleaseStatusDispatching {
		t.Fatalf("unexpected release status: %v", output.Status)
	}
	if len(dispatcher.tasks) != 1 {
		t.Fatalf("expected one dispatched task, got %d", len(dispatcher.tasks))
	}
	if dispatcher.tasks[0].Type != model.TaskTypeDeployGreen {
		t.Fatalf("expected deploy task, got %v", dispatcher.tasks[0].Type)
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
		ServiceID:  serviceID,
		ServiceKey: "svc-a",
		TargetSlot: model.SlotGreen,
		HostPort:   18081,
		ServerName: "srv_green",
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
		TaskID:        taskID.String(),
		Status:        "succeeded",
		Step:          "healthy",
		ContainerID:   "container-1",
		ListenAddress: "127.0.0.1:18081",
		Slot:          int(model.SlotGreen),
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

func (r *fakeServiceRepo) List() ([]model.Service, error) {
	r.ensure()
	out := make([]model.Service, 0, len(r.byID))
	for _, item := range r.byID {
		out = append(out, *item)
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

var _ agentdomain.Repository = (*fakeAgentRepo)(nil)

type fakeReleaseRepo struct {
	releases         map[uuid.UUID]*model.Release
	tasks            map[uuid.UUID]*model.Task
	taskAttempts     []*model.TaskAttempt
	audits           []*model.AuditLog
	runtimeByService map[uuid.UUID]map[model.Slot]*model.RuntimeInstance
}

func newFakeReleaseRepo() *fakeReleaseRepo {
	return &fakeReleaseRepo{
		releases:         make(map[uuid.UUID]*model.Release),
		tasks:            make(map[uuid.UUID]*model.Task),
		runtimeByService: make(map[uuid.UUID]map[model.Slot]*model.RuntimeInstance),
	}
}

func (r *fakeReleaseRepo) CreateRelease(release *model.Release) error {
	copyRelease := *release
	r.releases[release.ID] = &copyRelease
	return nil
}

func (r *fakeReleaseRepo) UpdateRelease(release *model.Release) error {
	copyRelease := *release
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
	return out, nil
}

func (r *fakeReleaseRepo) HasActiveRelease(serviceID uuid.UUID) (bool, error) {
	for _, item := range r.releases {
		if item.ServiceID == serviceID && item.Status != model.ReleaseStatusCompleted && item.Status != model.ReleaseStatusFailed && item.Status != model.ReleaseStatusRolledBack {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeReleaseRepo) CreateTask(task *model.Task) error {
	copyTask := *task
	r.tasks[task.ID] = &copyTask
	return nil
}

func (r *fakeReleaseRepo) UpdateTask(task *model.Task) error {
	copyTask := *task
	r.tasks[task.ID] = &copyTask
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
	tasks []*model.Task
}

func (d *fakeDispatcher) DispatchTask(agentID string, task *model.Task) error {
	copyTask := *task
	d.tasks = append(d.tasks, &copyTask)
	return nil
}

func mustJSONB(payload model.TaskPayload) *commondb.JSONB[model.TaskPayload] {
	return commondb.NewJSONB(payload)
}

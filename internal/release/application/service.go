package application

import (
	"edge-pilot/internal/agent/application"
	releasedomain "edge-pilot/internal/release/domain"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
	commondb "github.com/real-uangi/allingo/common/db"
)

type Service struct {
	repo          releasedomain.Repository
	dispatcher    releasedomain.TaskDispatcher
	services      *servicecatalogapp.Service
	agentRegistry *application.RegistryService
}

func NewService(
	repo releasedomain.Repository,
	dispatcher releasedomain.TaskDispatcher,
	services *servicecatalogapp.Service,
	agentRegistry *application.RegistryService,
) *Service {
	return &Service{
		repo:          repo,
		dispatcher:    dispatcher,
		services:      services,
		agentRegistry: agentRegistry,
	}
}

func (s *Service) CreateFromCI(req dto.CreateReleaseFromCIRequest) (*dto.ReleaseOutput, error) {
	spec, err := s.services.GetSpecByKey(req.ServiceKey)
	if err != nil {
		return nil, err
	}
	if !spec.Enabled {
		return nil, business.NewBadRequest("service 已禁用")
	}
	duplicate, err := s.repo.FindQueuedOrActiveDuplicate(spec.ID, req.ImageTag, req.CommitSHA)
	if err != nil {
		return nil, err
	}
	if duplicate != nil {
		if err := s.repo.CreateAudit(newAudit("release", duplicate.ID.String(), "release_deduplicated", req.TraceID, duplicate.ID.String())); err != nil {
			return nil, err
		}
		output, err := s.enrichReleaseOutput(duplicate)
		if err != nil {
			return nil, err
		}
		return &output, nil
	}
	release := &model.Release{
		ID:               uuid.New(),
		ServiceID:        spec.ID,
		AgentID:          spec.AgentID,
		ImageRepo:        firstNonEmpty(req.ImageRepo, spec.ImageRepo),
		ImageTag:         req.ImageTag,
		CommitSHA:        req.CommitSHA,
		TriggeredBy:      req.TriggeredBy,
		TraceID:          req.TraceID,
		Status:           model.ReleaseStatusQueued,
		TargetSlot:       nextSlot(spec.CurrentLiveSlot),
		PreviousLiveSlot: spec.CurrentLiveSlot,
		SwitchConfirmed:  boolPointer(false),
	}

	if err := s.repo.CreateRelease(release); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "release_requested", req.TraceID, "release queued")); err != nil {
		return nil, err
	}
	output, err := s.enrichReleaseOutput(release)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Service) Start(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	release, err := s.repo.GetRelease(id)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, business.ErrNotFound
	}
	if !release.Status.IsQueued() {
		return nil, business.NewErrorWithCode("release is not queued", 409)
	}
	active, err := s.repo.HasActiveRelease(release.ServiceID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, business.NewErrorWithCode("service has active release", 409)
	}
	spec, err := s.services.GetSpecByID(release.ServiceID)
	if err != nil {
		return nil, err
	}
	if !spec.Enabled {
		return nil, business.NewBadRequest("service 已禁用")
	}
	online, err := s.agentRegistry.IsOnline(spec.AgentID)
	if err != nil {
		return nil, err
	}
	if !online {
		return nil, business.NewErrorWithCode("agent not online", 409)
	}
	release.AgentID = spec.AgentID
	release.PreviousLiveSlot = spec.CurrentLiveSlot
	release.TargetSlot = nextSlot(spec.CurrentLiveSlot)
	task := s.newDeployTask(release, spec, dto.CreateReleaseFromCIRequest{
		ImageRepo: release.ImageRepo,
		ImageTag:  release.ImageTag,
		CommitSHA: release.CommitSHA,
		TraceID:   release.TraceID,
	})
	release.CurrentTaskID = &task.ID
	release.Status = model.ReleaseStatusDispatching
	if err := s.repo.CreateTask(task); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateRelease(release); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "release_started", release.TraceID, operator)); err != nil {
		return nil, err
	}
	if err := s.dispatch(task); err != nil {
		return nil, err
	}
	output, err := s.enrichReleaseOutput(release)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Service) Skip(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	release, err := s.repo.GetRelease(id)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, business.ErrNotFound
	}
	if !release.Status.IsQueued() {
		return nil, business.NewErrorWithCode("release is not queued", 409)
	}
	now := time.Now()
	release.Status = model.ReleaseStatusSkipped
	release.CompletedAt = &now
	if err := s.repo.UpdateRelease(release); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "release_skipped", release.TraceID, operator)); err != nil {
		return nil, err
	}
	output, err := s.enrichReleaseOutput(release)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Service) ConfirmSwitch(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	release, err := s.repo.GetRelease(id)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, business.ErrNotFound
	}
	if release.Status != model.ReleaseStatusReadyToSwitch {
		return nil, business.NewErrorWithCode("release is not ready to switch", 409)
	}
	spec, err := s.services.GetSpecByID(release.ServiceID)
	if err != nil {
		return nil, err
	}
	task := s.newSwitchTask(release, spec, model.TaskTypeSwitchTraffic)
	if err := s.repo.CreateTask(task); err != nil {
		return nil, err
	}
	release.CurrentTaskID = &task.ID
	release.SwitchConfirmed = boolPointer(true)
	if err := s.repo.UpdateRelease(release); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "switch_confirmed", release.TraceID, operator)); err != nil {
		return nil, err
	}
	if err := s.dispatch(task); err != nil {
		return nil, err
	}
	output := toReleaseOutput(release)
	return &output, nil
}

func (s *Service) Rollback(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	release, err := s.repo.GetRelease(id)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, business.ErrNotFound
	}
	if release.Status == model.ReleaseStatusQueued || release.Status == model.ReleaseStatusSkipped {
		return nil, business.NewErrorWithCode("release has not started", 409)
	}
	if release.PreviousLiveSlot == 0 {
		return nil, business.NewErrorWithCode("release has no rollback target", 409)
	}
	spec, err := s.services.GetSpecByID(release.ServiceID)
	if err != nil {
		return nil, err
	}
	task := s.newRollbackTask(release, spec)
	if err := s.repo.CreateTask(task); err != nil {
		return nil, err
	}
	release.CurrentTaskID = &task.ID
	if err := s.repo.UpdateRelease(release); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "rollback_requested", release.TraceID, operator)); err != nil {
		return nil, err
	}
	if err := s.dispatch(task); err != nil {
		return nil, err
	}
	output, err := s.enrichReleaseOutput(release)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Service) Get(id uuid.UUID) (*dto.ReleaseOutput, error) {
	release, err := s.repo.GetRelease(id)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, business.ErrNotFound
	}
	output, err := s.enrichReleaseOutput(release)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Service) List() ([]dto.ReleaseOutput, error) {
	releases, err := s.repo.ListReleases(50)
	if err != nil {
		return nil, err
	}
	output := make([]dto.ReleaseOutput, 0, len(releases))
	for i := range releases {
		item, err := s.enrichReleaseOutput(&releases[i])
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	return output, nil
}

func (s *Service) ListTaskSnapshots(releaseID uuid.UUID) ([]dto.TaskSnapshot, error) {
	tasks, err := s.repo.ListTasksByRelease(releaseID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.TaskSnapshot, 0, len(tasks))
	for i := range tasks {
		out = append(out, dto.TaskSnapshot{
			ID:           tasks[i].ID,
			Type:         tasks[i].Type,
			Status:       tasks[i].Status,
			LastError:    tasks[i].LastError,
			DispatchedAt: tasks[i].DispatchedAt,
			StartedAt:    tasks[i].StartedAt,
			CompletedAt:  tasks[i].CompletedAt,
		})
	}
	return out, nil
}

func (s *Service) HandleTaskUpdate(agentID string, update *grpcapi.TaskUpdate) error {
	taskID, err := uuid.Parse(update.GetTaskId())
	if err != nil {
		return err
	}
	task, err := s.repo.GetTask(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return business.ErrNotFound
	}
	release, err := s.repo.GetRelease(task.ReleaseID)
	if err != nil {
		return err
	}
	if release == nil {
		return business.ErrNotFound
	}
	now := time.Now()
	switch update.GetStatus() {
	case grpcapi.TaskStatus_TASK_STATUS_RUNNING:
		task.Status = model.TaskStatusRunning
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
		if err := s.repo.UpdateTask(task); err != nil {
			return err
		}
		if err := s.repo.CreateTaskAttempt(&model.TaskAttempt{
			ID:        uuid.New(),
			TaskID:    task.ID,
			AgentID:   agentID,
			Status:    model.TaskStatusRunning,
			Message:   update.GetStep(),
			StartedAt: &now,
		}); err != nil {
			return err
		}
		return s.recordRunningAudit(release, task, update)
	case grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED:
		task.Status = model.TaskStatusSucceeded
		task.CompletedAt = &now
		if err := s.repo.UpdateTask(task); err != nil {
			return err
		}
		if err := s.repo.CreateTaskAttempt(&model.TaskAttempt{
			ID:          uuid.New(),
			TaskID:      task.ID,
			AgentID:     agentID,
			Status:      model.TaskStatusSucceeded,
			Message:     update.GetStep(),
			CompletedAt: &now,
		}); err != nil {
			return err
		}
		return s.applyTaskSuccess(release, task, update, now)
	case grpcapi.TaskStatus_TASK_STATUS_FAILED:
		task.Status = model.TaskStatusFailed
		task.CompletedAt = &now
		task.LastError = update.GetErrorMessage()
		release.Status = model.ReleaseStatusFailed
		release.CompletedAt = &now
		if err := s.repo.UpdateTask(task); err != nil {
			return err
		}
		if err := s.repo.UpdateRelease(release); err != nil {
			return err
		}
		if err := s.repo.CreateTaskAttempt(&model.TaskAttempt{
			ID:          uuid.New(),
			TaskID:      task.ID,
			AgentID:     agentID,
			Status:      model.TaskStatusFailed,
			Message:     coalesceNonEmpty(update.GetErrorMessage(), update.GetStep()),
			CompletedAt: &now,
		}); err != nil {
			return err
		}
		if update.GetStep() == "managed_container_conflict" {
			return s.repo.CreateAudit(newAudit("release", release.ID.String(), "managed_container_conflict", release.TraceID, update.GetErrorMessage()))
		}
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "task_failed", release.TraceID, update.GetErrorMessage()))
	default:
		return nil
	}
}

func (s *Service) RecoverAgentTasks(agentID string, runningTaskIDs []string) error {
	tasks, err := s.repo.ListRecoverableTasksByAgent(agentID)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}
	running := make(map[string]struct{}, len(runningTaskIDs))
	for _, taskID := range runningTaskIDs {
		running[taskID] = struct{}{}
	}
	now := time.Now()
	for i := range tasks {
		task := tasks[i]
		if _, ok := running[task.ID.String()]; ok {
			if task.Status != model.TaskStatusRunning {
				task.Status = model.TaskStatusRunning
				if task.StartedAt == nil {
					task.StartedAt = &now
				}
				if err := s.repo.UpdateTask(&task); err != nil {
					return err
				}
				if err := s.repo.CreateTaskAttempt(&model.TaskAttempt{
					ID:        uuid.New(),
					TaskID:    task.ID,
					AgentID:   agentID,
					Status:    model.TaskStatusRunning,
					Message:   "recovered_running",
					StartedAt: &now,
				}); err != nil {
					return err
				}
			}
			continue
		}
		replayed, err := s.dispatcher.ReplayTask(agentID, &task)
		if err != nil {
			return err
		}
		if !replayed {
			continue
		}
		task.Status = model.TaskStatusDispatched
		task.DispatchedAt = &now
		if err := s.repo.UpdateTask(&task); err != nil {
			return err
		}
		if err := s.repo.CreateTaskAttempt(&model.TaskAttempt{
			ID:        uuid.New(),
			TaskID:    task.ID,
			AgentID:   agentID,
			Status:    model.TaskStatusDispatched,
			Message:   "replayed_after_reconnect",
			StartedAt: &now,
		}); err != nil {
			return err
		}
		release, err := s.repo.GetRelease(task.ReleaseID)
		if err != nil {
			return err
		}
		if release != nil {
			if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "task_replayed", release.TraceID, task.ID.String())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) FailStaleTasks(before time.Time) error {
	tasks, err := s.repo.ListStaleTasks(before)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}
	now := time.Now()
	for i := range tasks {
		task := tasks[i]
		release, err := s.repo.GetRelease(task.ReleaseID)
		if err != nil {
			return err
		}
		if release == nil {
			continue
		}
		task.Status = model.TaskStatusTimedOut
		task.CompletedAt = &now
		task.LastError = "task timed out"
		if err := s.repo.UpdateTask(&task); err != nil {
			return err
		}
		release.Status = model.ReleaseStatusFailed
		release.CompletedAt = &now
		if err := s.repo.UpdateRelease(release); err != nil {
			return err
		}
		if err := s.repo.CreateTaskAttempt(&model.TaskAttempt{
			ID:          uuid.New(),
			TaskID:      task.ID,
			AgentID:     task.AgentID,
			Status:      model.TaskStatusTimedOut,
			Message:     "task timed out",
			CompletedAt: &now,
		}); err != nil {
			return err
		}
		if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "task_timed_out", release.TraceID, task.ID.String())); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) RecordAgentOfflineTimeout(agentID string) error {
	return s.repo.CreateAudit(newAudit("agent", agentID, "agent_offline_timeout", "", "agent offline due to heartbeat timeout"))
}

func (s *Service) GetRuntimeInstances(serviceID uuid.UUID) ([]model.RuntimeInstance, error) {
	return s.repo.ListRuntimeInstancesByService(serviceID)
}

func (s *Service) dispatch(task *model.Task) error {
	now := time.Now()
	task.Status = model.TaskStatusDispatched
	task.DispatchedAt = &now
	if err := s.repo.UpdateTask(task); err != nil {
		return err
	}
	if err := s.repo.CreateTaskAttempt(&model.TaskAttempt{
		ID:        uuid.New(),
		TaskID:    task.ID,
		AgentID:   task.AgentID,
		Status:    model.TaskStatusDispatched,
		Message:   "dispatched",
		StartedAt: &now,
	}); err != nil {
		return err
	}
	return s.dispatcher.DispatchTask(task.AgentID, task)
}

func (s *Service) recordRunningAudit(release *model.Release, task *model.Task, update *grpcapi.TaskUpdate) error {
	if release == nil {
		return nil
	}
	step := update.GetStep()
	switch {
	case strings.HasPrefix(step, "cleanup_pruned"):
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "cleanup_pruned", release.TraceID, step))
	case step == "cleanup_failed":
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "cleanup_failed", release.TraceID, coalesceNonEmpty(update.GetErrorMessage(), step)))
	default:
		return nil
	}
}

func (s *Service) applyTaskSuccess(release *model.Release, task *model.Task, update *grpcapi.TaskUpdate, now time.Time) error {
	payload := getJSON(task.Payload)
	switch task.Type {
	case model.TaskTypeDeployGreen:
		healthy := true
		accepting := false
		active := true
		instance := &model.RuntimeInstance{
			ID:               uuid.New(),
			ServiceID:        task.ServiceID,
			ReleaseID:        task.ReleaseID,
			Slot:             model.Slot(update.GetSlot()),
			ContainerID:      update.GetContainerId(),
			ImageTag:         release.ImageTag,
			ListenAddress:    update.GetListenAddress(),
			HostPort:         firstPublishedHostPort(payload.PublishedPorts),
			ServerName:       payload.ServerName,
			Healthy:          &healthy,
			AcceptingTraffic: &accepting,
			Active:           &active,
		}
		if err := s.repo.UpsertRuntimeInstance(instance); err != nil {
			return err
		}
		release.Status = model.ReleaseStatusReadyToSwitch
		if err := s.repo.UpdateRelease(release); err != nil {
			return err
		}
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "ready_to_switch", release.TraceID, update.GetListenAddress()))
	case model.TaskTypeSwitchTraffic:
		release.Status = model.ReleaseStatusCompleted
		release.CompletedAt = &now
		if err := s.services.UpdateLiveSlot(task.ServiceID, payload.TargetSlot); err != nil {
			return err
		}
		if err := s.updateTrafficFlags(task.ServiceID, payload.TargetSlot, release.PreviousLiveSlot); err != nil {
			return err
		}
		if err := s.repo.UpdateRelease(release); err != nil {
			return err
		}
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "traffic_switched", release.TraceID, update.GetServerName()))
	case model.TaskTypeRollback:
		release.Status = model.ReleaseStatusRolledBack
		release.CompletedAt = &now
		if err := s.services.UpdateLiveSlot(task.ServiceID, payload.TargetSlot); err != nil {
			return err
		}
		if err := s.updateTrafficFlags(task.ServiceID, payload.TargetSlot, payload.CurrentLiveSlot); err != nil {
			return err
		}
		if err := s.repo.UpdateRelease(release); err != nil {
			return err
		}
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "rolled_back", release.TraceID, update.GetServerName()))
	default:
		return nil
	}
}

func (s *Service) updateTrafficFlags(serviceID uuid.UUID, liveSlot model.Slot, oldSlot model.Slot) error {
	current, err := s.repo.GetRuntimeInstanceByServiceAndSlot(serviceID, liveSlot)
	if err != nil {
		return err
	}
	if current != nil {
		healthy := true
		accepting := true
		active := true
		current.Healthy = &healthy
		current.AcceptingTraffic = &accepting
		current.Active = &active
		if err := s.repo.UpsertRuntimeInstance(current); err != nil {
			return err
		}
	}
	old, err := s.repo.GetRuntimeInstanceByServiceAndSlot(serviceID, oldSlot)
	if err != nil {
		return err
	}
	if old != nil {
		healthy := true
		accepting := false
		active := true
		old.Healthy = &healthy
		old.AcceptingTraffic = &accepting
		old.Active = &active
		if err := s.repo.UpsertRuntimeInstance(old); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) newDeployTask(release *model.Release, spec *dto.ServiceDeploymentSpec, req dto.CreateReleaseFromCIRequest) *model.Task {
	payload := model.TaskPayload{
		ServiceID:         spec.ID,
		ServiceKey:        spec.ServiceKey,
		ImageRepo:         firstNonEmpty(req.ImageRepo, spec.ImageRepo),
		ImageTag:          req.ImageTag,
		CommitSHA:         req.CommitSHA,
		TraceID:           req.TraceID,
		TargetSlot:        release.TargetSlot,
		CurrentLiveSlot:   spec.CurrentLiveSlot,
		ContainerPort:     spec.ContainerPort,
		DockerHealthCheck: spec.DockerHealthCheck,
		HTTPHealthPath:    firstNonEmpty(spec.HTTPHealthPath, "/health"),
		HTTPExpectedCode:  defaultInt(spec.HTTPExpectedCode, 200),
		HTTPTimeoutSecond: defaultInt(spec.HTTPTimeoutSecond, 5),
		BackendName:       servicecatalogapp.BackendName(spec.ID),
		ServerName:        servicecatalogapp.ServerName(release.TargetSlot),
		PreviousServer:    servicecatalogapp.ServerName(spec.CurrentLiveSlot),
		Env:               spec.Env,
		Command:           spec.Command,
		Entrypoint:        spec.Entrypoint,
		Volumes:           toModelVolumeMounts(spec.Volumes),
		PublishedPorts:    toModelPublishedPorts(spec.PublishedPorts),
	}
	return &model.Task{
		ID:        uuid.New(),
		ReleaseID: release.ID,
		ServiceID: spec.ID,
		AgentID:   spec.AgentID,
		Type:      model.TaskTypeDeployGreen,
		Status:    model.TaskStatusPending,
		Payload:   commondb.NewJSONB(payload),
	}
}

func (s *Service) newSwitchTask(release *model.Release, spec *dto.ServiceDeploymentSpec, taskType model.TaskType) *model.Task {
	payload := model.TaskPayload{
		ServiceID:         spec.ID,
		ServiceKey:        spec.ServiceKey,
		ImageRepo:         spec.ImageRepo,
		ImageTag:          release.ImageTag,
		CommitSHA:         release.CommitSHA,
		TraceID:           release.TraceID,
		TargetSlot:        release.TargetSlot,
		CurrentLiveSlot:   release.PreviousLiveSlot,
		ContainerPort:     spec.ContainerPort,
		DockerHealthCheck: spec.DockerHealthCheck,
		HTTPHealthPath:    firstNonEmpty(spec.HTTPHealthPath, "/health"),
		HTTPExpectedCode:  defaultInt(spec.HTTPExpectedCode, 200),
		HTTPTimeoutSecond: defaultInt(spec.HTTPTimeoutSecond, 5),
		BackendName:       servicecatalogapp.BackendName(spec.ID),
		ServerName:        servicecatalogapp.ServerName(release.TargetSlot),
		PreviousServer:    servicecatalogapp.ServerName(release.PreviousLiveSlot),
		Env:               spec.Env,
		Command:           spec.Command,
		Entrypoint:        spec.Entrypoint,
		Volumes:           toModelVolumeMounts(spec.Volumes),
		PublishedPorts:    toModelPublishedPorts(spec.PublishedPorts),
	}
	return &model.Task{
		ID:        uuid.New(),
		ReleaseID: release.ID,
		ServiceID: spec.ID,
		AgentID:   spec.AgentID,
		Type:      taskType,
		Status:    model.TaskStatusPending,
		Payload:   commondb.NewJSONB(payload),
	}
}

func (s *Service) newRollbackTask(release *model.Release, spec *dto.ServiceDeploymentSpec) *model.Task {
	payload := model.TaskPayload{
		ServiceID:         spec.ID,
		ServiceKey:        spec.ServiceKey,
		ImageRepo:         spec.ImageRepo,
		ImageTag:          release.ImageTag,
		CommitSHA:         release.CommitSHA,
		TraceID:           release.TraceID,
		TargetSlot:        release.PreviousLiveSlot,
		CurrentLiveSlot:   spec.CurrentLiveSlot,
		ContainerPort:     spec.ContainerPort,
		DockerHealthCheck: spec.DockerHealthCheck,
		BackendName:       servicecatalogapp.BackendName(spec.ID),
		ServerName:        servicecatalogapp.ServerName(release.PreviousLiveSlot),
		PreviousServer:    servicecatalogapp.ServerName(spec.CurrentLiveSlot),
		Env:               spec.Env,
		Command:           spec.Command,
		Entrypoint:        spec.Entrypoint,
		Volumes:           toModelVolumeMounts(spec.Volumes),
		PublishedPorts:    toModelPublishedPorts(spec.PublishedPorts),
	}
	return &model.Task{
		ID:        uuid.New(),
		ReleaseID: release.ID,
		ServiceID: spec.ID,
		AgentID:   spec.AgentID,
		Type:      model.TaskTypeRollback,
		Status:    model.TaskStatusPending,
		Payload:   commondb.NewJSONB(payload),
	}
}

func toReleaseOutput(release *model.Release) dto.ReleaseOutput {
	return dto.ReleaseOutput{
		ID:               release.ID,
		ServiceID:        release.ServiceID,
		AgentID:          release.AgentID,
		ImageRepo:        release.ImageRepo,
		ImageTag:         release.ImageTag,
		CommitSHA:        release.CommitSHA,
		TriggeredBy:      release.TriggeredBy,
		TraceID:          release.TraceID,
		Status:           release.Status,
		TargetSlot:       release.TargetSlot,
		PreviousLiveSlot: release.PreviousLiveSlot,
		CurrentTaskID:    release.CurrentTaskID,
		SwitchConfirmed:  release.SwitchConfirmed,
		IsActive:         release.Status.IsActive(),
		CreatedAt:        release.CreatedAt,
		UpdatedAt:        release.UpdatedAt,
		CompletedAt:      release.CompletedAt,
	}
}

func (s *Service) enrichReleaseOutput(release *model.Release) (dto.ReleaseOutput, error) {
	output := toReleaseOutput(release)
	if !release.Status.IsQueued() {
		return output, nil
	}
	count, err := s.repo.CountQueuedBefore(release.ServiceID, release.CreatedAt, release.ID)
	if err != nil {
		return dto.ReleaseOutput{}, err
	}
	output.QueuePosition = count + 1
	return output, nil
}

func newAudit(aggregateType string, aggregateID string, eventType string, traceID string, message string) *model.AuditLog {
	return &model.AuditLog{
		ID:            uuid.New(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		TraceID:       traceID,
		Message:       message,
		Metadata:      commondb.NewJSONB(map[string]string{"message": message}),
	}
}

func coalesceNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nextSlot(current model.Slot) model.Slot {
	if current == model.SlotBlue {
		return model.SlotGreen
	}
	return model.SlotBlue
}

func toModelVolumeMounts(items []dto.VolumeMount) []model.VolumeMount {
	out := make([]model.VolumeMount, 0, len(items))
	for _, item := range items {
		out = append(out, model.VolumeMount{
			Source:   item.Source,
			Target:   item.Target,
			ReadOnly: item.ReadOnly,
		})
	}
	return out
}

func toModelPublishedPorts(items []dto.PublishedPort) []model.PublishedPort {
	out := make([]model.PublishedPort, 0, len(items))
	for _, item := range items {
		out = append(out, model.PublishedPort{
			HostPort:      item.HostPort,
			ContainerPort: item.ContainerPort,
		})
	}
	return out
}

func firstPublishedHostPort(items []model.PublishedPort) int {
	if len(items) == 0 {
		return 0
	}
	return items[0].HostPort
}

func getJSON[T any](value *commondb.JSONB[T]) T {
	var zero T
	if value == nil {
		return zero
	}
	return value.Get()
}

func boolPointer(v bool) *bool {
	return &v
}

func firstNonEmpty(v string, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func defaultInt(v int, fallback int) int {
	if v != 0 {
		return v
	}
	return fallback
}

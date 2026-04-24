package application

import (
	releasedomain "edge-pilot/internal/release/domain"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	servicecatalogdomain "edge-pilot/internal/servicecatalog/domain"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
	"edge-pilot/internal/shared/secret"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
	commondb "github.com/real-uangi/allingo/common/db"
)

type AgentOnlineChecker interface {
	IsOnline(agentID string) (bool, error)
}

type Service struct {
	repo          releasedomain.Repository
	dispatcher    releasedomain.TaskDispatcher
	services      *servicecatalogapp.Service
	agentRegistry AgentOnlineChecker
	proxyConfigs  servicecatalogdomain.ProxyConfigPublisher
	registryAuth  releasedomain.RegistryCredentialResolver
	codec         *secret.Codec
}

const (
	deployImagePullBudget = 5 * time.Minute
	deployTimeoutBuffer   = 30 * time.Second
	switchTaskTimeout     = 1 * time.Minute
	cleanupTaskTimeout    = 1 * time.Minute
)

func NewService(
	repo releasedomain.Repository,
	dispatcher releasedomain.TaskDispatcher,
	services *servicecatalogapp.Service,
	agentRegistry AgentOnlineChecker,
) *Service {
	return NewServiceWithRegistryCredentialsAndCodec(repo, dispatcher, services, agentRegistry, nil, nil, nil)
}

func NewServiceWithRegistryCredentials(
	repo releasedomain.Repository,
	dispatcher releasedomain.TaskDispatcher,
	services *servicecatalogapp.Service,
	agentRegistry AgentOnlineChecker,
	registryAuth releasedomain.RegistryCredentialResolver,
) *Service {
	return NewServiceWithRegistryCredentialsAndCodec(repo, dispatcher, services, agentRegistry, nil, registryAuth, nil)
}

func NewServiceWithRegistryCredentialsAndCodec(
	repo releasedomain.Repository,
	dispatcher releasedomain.TaskDispatcher,
	services *servicecatalogapp.Service,
	agentRegistry AgentOnlineChecker,
	proxyConfigs servicecatalogdomain.ProxyConfigPublisher,
	registryAuth releasedomain.RegistryCredentialResolver,
	codec *secret.Codec,
) *Service {
	if registryAuth == nil {
		registryAuth = noopRegistryCredentialResolver{}
	}
	return &Service{
		repo:          repo,
		dispatcher:    dispatcher,
		services:      services,
		agentRegistry: agentRegistry,
		proxyConfigs:  proxyConfigs,
		registryAuth:  registryAuth,
		codec:         codec,
	}
}

type noopRegistryCredentialResolver struct{}

func (noopRegistryCredentialResolver) ResolveForImageRepo(string) (*releasedomain.ResolvedRegistryCredential, error) {
	return nil, nil
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
		TrafficPercent:   0,
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
		split, err := s.repo.HasTrafficSplitRelease(release.ServiceID)
		if err != nil {
			return nil, err
		}
		if split {
			return nil, business.NewErrorWithCode("service has in-progress traffic split (1-99%), finish at 0% or 100% before starting a new release", 409)
		}
		return nil, business.NewErrorWithCode("service has active release", 409)
	}
	spec, err := s.services.GetSpecByID(release.ServiceID)
	if err != nil {
		return nil, err
	}
	if !spec.Enabled {
		return nil, business.NewBadRequest("service 已禁用")
	}
	if err := validateDeploymentSpec(spec); err != nil {
		return nil, err
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
	release.TrafficPercent = 0
	if err := s.completeSupersededRelease(release, operator); err != nil {
		return nil, err
	}
	registryAuth, err := s.registryAuth.ResolveForImageRepo(release.ImageRepo)
	if err != nil {
		return nil, err
	}
	task, err := s.newDeployTask(release, spec, dto.CreateReleaseFromCIRequest{
		ImageRepo: release.ImageRepo,
		ImageTag:  release.ImageTag,
		CommitSHA: release.CommitSHA,
		TraceID:   release.TraceID,
	}, registryAuth)
	if err != nil {
		return nil, err
	}
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
	if err := s.autoSkipQueuedBeforeStart(release, operator); err != nil {
		return nil, err
	}
	output, err := s.enrichReleaseOutput(release)
	if err != nil {
		return nil, err
	}
	return &output, nil
}

func (s *Service) Retry(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	release, err := s.repo.GetRelease(id)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, business.ErrNotFound
	}
	if release.Status != model.ReleaseStatusFailed {
		return nil, business.NewErrorWithCode("release is not failed", 409)
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
	if err := validateDeploymentSpec(spec); err != nil {
		return nil, err
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
	release.TrafficPercent = 0
	release.SwitchConfirmed = boolPointer(false)
	if err := s.completeSupersededRelease(release, operator); err != nil {
		return nil, err
	}
	release.CompletedAt = nil
	registryAuth, err := s.registryAuth.ResolveForImageRepo(release.ImageRepo)
	if err != nil {
		return nil, err
	}
	task, err := s.newDeployTask(release, spec, dto.CreateReleaseFromCIRequest{
		ImageRepo: release.ImageRepo,
		ImageTag:  release.ImageTag,
		CommitSHA: release.CommitSHA,
		TraceID:   release.TraceID,
	}, registryAuth)
	if err != nil {
		return nil, err
	}
	release.CurrentTaskID = &task.ID
	release.Status = model.ReleaseStatusDispatching
	if err := s.repo.CreateTask(task); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateRelease(release); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "release_retried", release.TraceID, operator)); err != nil {
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
	return s.SetTrafficPercent(id, 100, operator)
}

func (s *Service) Rollback(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	return s.SetTrafficPercent(id, 0, operator)
}

func (s *Service) SetTrafficPercent(id uuid.UUID, percent int, operator string) (*dto.ReleaseOutput, error) {
	if percent < 0 || percent > 100 {
		return nil, business.NewBadRequest("traffic percent must be within 0..100")
	}
	release, err := s.repo.GetRelease(id)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, business.ErrNotFound
	}
	if release.Status == model.ReleaseStatusQueued || release.Status == model.ReleaseStatusSkipped || release.Status == model.ReleaseStatusDispatching || release.Status == model.ReleaseStatusDeploying {
		return nil, business.NewErrorWithCode("release is not ready for traffic adjustment", 409)
	}
	if release.Status.IsTerminal() {
		return nil, business.NewErrorWithCode("release already finished", 409)
	}
	if release.PreviousLiveSlot == 0 {
		return nil, business.NewErrorWithCode("release has no rollback target", 409)
	}
	release.TrafficPercent = percent
	release.SwitchConfirmed = boolPointer(percent == 100)
	release.CompletedAt = nil
	if percent == 100 {
		now := time.Now()
		release.Status = model.ReleaseStatusCompleted
		release.CompletedAt = &now
		if err := s.services.UpdateLiveSlot(release.ServiceID, release.TargetSlot); err != nil {
			return nil, err
		}
		if err := s.updateTrafficFlags(release.ServiceID, release.TargetSlot, release.PreviousLiveSlot); err != nil {
			return nil, err
		}
		if err := s.completePreviousLiveRelease(release, operator, now); err != nil {
			return nil, err
		}
	} else {
		release.Status = model.ReleaseStatusReadyToSwitch
		if err := s.services.UpdateLiveSlot(release.ServiceID, release.PreviousLiveSlot); err != nil {
			return nil, err
		}
		if err := s.updateTrafficFlags(release.ServiceID, release.PreviousLiveSlot, release.TargetSlot); err != nil {
			return nil, err
		}
	}
	if err := s.repo.UpdateRelease(release); err != nil {
		return nil, err
	}
	if err := s.publishAgent(release.AgentID); err != nil {
		return nil, err
	}
	event := "traffic_percent_updated"
	message := fmt.Sprintf("percent=%d operator=%s", percent, strings.TrimSpace(operator))
	if percent == 100 {
		event = "switch_confirmed"
		message = operator
	} else if percent == 0 {
		event = "rollback_requested"
		message = operator
	}
	if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), event, release.TraceID, message)); err != nil {
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
			ID:               tasks[i].ID,
			Type:             tasks[i].Type,
			Status:           tasks[i].Status,
			LastError:        tasks[i].LastError,
			LastStep:         tasks[i].LastStep,
			DockerHealth:     tasks[i].DockerHealth,
			FailureLogs:      tasks[i].FailureLogs,
			CleanupCompleted: tasks[i].CleanupCompleted,
			DispatchedAt:     tasks[i].DispatchedAt,
			StartedAt:        tasks[i].StartedAt,
			CompletedAt:      tasks[i].CompletedAt,
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
	if release.CurrentTaskID == nil || *release.CurrentTaskID != task.ID {
		return s.recordLateTaskUpdate(release, task, update, "not_current_task")
	}
	if task.Status.IsTerminal() || release.Status.IsTerminal() {
		return s.recordLateTaskUpdate(release, task, update, "task_or_release_terminal")
	}
	now := time.Now()
	switch update.GetStatus() {
	case grpcapi.TaskStatus_TASK_STATUS_RUNNING:
		previousStatus := task.Status
		previousStep := task.LastStep
		task.Status = model.TaskStatusRunning
		task.LastStep = update.GetStep()
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
		if task.Type == model.TaskTypeDeployGreen && release.Status == model.ReleaseStatusDispatching {
			release.Status = model.ReleaseStatusDeploying
			if err := s.repo.UpdateRelease(release); err != nil {
				return err
			}
		}
		if err := s.repo.UpdateTask(task); err != nil {
			return err
		}
		if previousStatus != model.TaskStatusRunning || previousStep != update.GetStep() {
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
		}
		return s.recordRunningAudit(release, task, update)
	case grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED:
		task.Status = model.TaskStatusSucceeded
		task.CompletedAt = &now
		task.LastStep = update.GetStep()
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
		task.LastStep = update.GetStep()
		task.DockerHealth = update.GetDockerHealth()
		task.FailureLogs = update.GetFailureLogs()
		task.CleanupCompleted = boolPointer(update.GetCleanupCompleted())
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
		if update.GetStep() == "health_check_failed" {
			if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "health_check_failed", release.TraceID, coalesceNonEmpty(update.GetDockerHealth(), update.GetErrorMessage()))); err != nil {
				return err
			}
		}
		if strings.TrimSpace(update.GetFailureLogs()) != "" {
			if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "failure_logs_uploaded", release.TraceID, truncateForAudit(update.GetFailureLogs()))); err != nil {
				return err
			}
		}
		if update.GetCleanupCompleted() {
			if err := s.repo.CreateAudit(newAudit("release", release.ID.String(), "failed_container_cleaned", release.TraceID, task.ID.String())); err != nil {
				return err
			}
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

func (s *Service) FailStaleTasks(now time.Time) error {
	tasks, err := s.repo.ListActiveTasks()
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}
	for i := range tasks {
		task := tasks[i]
		if !taskTimedOut(&task, now) {
			continue
		}
		release, err := s.repo.GetRelease(task.ReleaseID)
		if err != nil {
			return err
		}
		if release == nil {
			continue
		}
		if release.CurrentTaskID == nil || *release.CurrentTaskID != task.ID {
			continue
		}
		if task.Status.IsTerminal() || release.Status.IsTerminal() {
			continue
		}
		task.Status = model.TaskStatusTimedOut
		task.CompletedAt = &now
		task.LastError = "task timed out"
		task.LastStep = "task_timed_out"
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
	task.LastStep = "dispatched"
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
	if err := s.dispatcher.DispatchTask(task.AgentID, task); err != nil {
		if !errors.Is(err, releasedomain.ErrAgentOffline) {
			return err
		}
		task.Status = model.TaskStatusPending
		task.DispatchedAt = nil
		task.LastStep = "dispatch_deferred"
		if updateErr := s.repo.UpdateTask(task); updateErr != nil {
			return updateErr
		}
		release, getErr := s.repo.GetRelease(task.ReleaseID)
		if getErr != nil {
			return getErr
		}
		if release != nil {
			if auditErr := s.repo.CreateAudit(newAudit("release", release.ID.String(), "dispatch_deferred", release.TraceID, task.ID.String())); auditErr != nil {
				return auditErr
			}
		}
		return nil
	}
	return nil
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
	case step == "startup_grace_started":
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "startup_grace_started", release.TraceID, step))
	case step == "health_probe_retry":
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "health_probe_retry", release.TraceID, coalesceNonEmpty(update.GetErrorMessage(), step)))
	default:
		return nil
	}
}

func (s *Service) applyTaskSuccess(release *model.Release, task *model.Task, update *grpcapi.TaskUpdate, now time.Time) error {
	_ = now
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
		release.TrafficPercent = 0
		release.SwitchConfirmed = boolPointer(false)
		if err := s.repo.UpdateRelease(release); err != nil {
			return err
		}
		if err := s.publishAgent(task.AgentID); err != nil {
			return err
		}
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "ready_to_switch", release.TraceID, update.GetListenAddress()))
	case model.TaskTypeSwitchTraffic:
		release.Status = model.ReleaseStatusCompleted
		release.TrafficPercent = 100
		release.SwitchConfirmed = boolPointer(true)
		release.CompletedAt = &now
		if err := s.services.UpdateLiveSlot(task.ServiceID, payload.TargetSlot); err != nil {
			return err
		}
		if err := s.updateTrafficFlags(task.ServiceID, payload.TargetSlot, release.PreviousLiveSlot); err != nil {
			return err
		}
		if err := s.completePreviousLiveRelease(release, update.GetServerName(), now); err != nil {
			return err
		}
		if err := s.repo.UpdateRelease(release); err != nil {
			return err
		}
		if err := s.publishAgent(task.AgentID); err != nil {
			return err
		}
		return s.repo.CreateAudit(newAudit("release", release.ID.String(), "traffic_switched", release.TraceID, update.GetServerName()))
	case model.TaskTypeRollback:
		release.Status = model.ReleaseStatusReadyToSwitch
		release.TrafficPercent = 0
		release.SwitchConfirmed = boolPointer(false)
		release.CompletedAt = nil
		if err := s.services.UpdateLiveSlot(task.ServiceID, payload.TargetSlot); err != nil {
			return err
		}
		if err := s.updateTrafficFlags(task.ServiceID, payload.TargetSlot, payload.CurrentLiveSlot); err != nil {
			return err
		}
		if err := s.repo.UpdateRelease(release); err != nil {
			return err
		}
		if err := s.publishAgent(task.AgentID); err != nil {
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
		accepting := true
		active := true
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
		accepting := false
		active := true
		old.AcceptingTraffic = &accepting
		old.Active = &active
		if err := s.repo.UpsertRuntimeInstance(old); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) newDeployTask(release *model.Release, spec *dto.ServiceDeploymentSpec, req dto.CreateReleaseFromCIRequest, registryAuth *releasedomain.ResolvedRegistryCredential) (*model.Task, error) {
	payload := model.TaskPayload{
		ServiceID:               spec.ID,
		ServiceKey:              spec.ServiceKey,
		ImageRepo:               firstNonEmpty(req.ImageRepo, spec.ImageRepo),
		ImageTag:                req.ImageTag,
		CommitSHA:               req.CommitSHA,
		TraceID:                 req.TraceID,
		TargetSlot:              release.TargetSlot,
		CurrentLiveSlot:         spec.CurrentLiveSlot,
		ContainerPort:           spec.ContainerPort,
		DockerHealthCheck:       spec.DockerHealthCheck,
		HTTPHealthPath:          firstNonEmpty(spec.HTTPHealthPath, "/health"),
		HTTPHealthHeaders:       cloneStringMap(spec.HTTPHealthHeaders),
		HTTPExpectedCode:        defaultInt(spec.HTTPExpectedCode, model.DefaultHTTPExpectedCode),
		HTTPTimeoutSecond:       defaultInt(spec.HTTPTimeoutSecond, model.DefaultHTTPTimeoutSecond),
		StartupGraceSecond:      defaultInt(spec.StartupGraceSecond, model.DefaultStartupGraceSecond),
		HTTPProbeTimeoutSecond:  defaultInt(spec.HTTPProbeTimeoutSecond, model.DefaultHTTPProbeTimeoutSecond),
		HTTPProbeIntervalSecond: defaultInt(spec.HTTPProbeIntervalSecond, model.DefaultHTTPProbeIntervalSecond),
		HTTPSuccessThreshold:    defaultInt(spec.HTTPSuccessThreshold, model.DefaultHTTPSuccessThreshold),
		BackendName:             servicecatalogapp.BackendNameForRelease(spec.ID, release.ID.String()),
		ServerName:              servicecatalogapp.ServerName(release.TargetSlot),
		PreviousServer:          servicecatalogapp.ServerName(spec.CurrentLiveSlot),
		Command:                 spec.Command,
		Entrypoint:              spec.Entrypoint,
		Volumes:                 toModelVolumeMounts(spec.Volumes),
		PublishedPorts:          toModelPublishedPorts(spec.PublishedPorts),
	}
	sensitive := model.TaskSensitivePayload{
		Env: spec.Env,
	}
	if registryAuth != nil {
		payload.RegistryHost = registryAuth.Host
		payload.RegistryUsername = registryAuth.Username
		sensitive.RegistrySecret = registryAuth.Secret
	}
	ciphertext, keyVersion, plaintextSensitive, err := s.prepareTaskSensitive(spec, sensitive)
	if err != nil {
		return nil, err
	}
	payload.Env = plaintextSensitive.Env
	payload.RegistrySecret = plaintextSensitive.RegistrySecret
	return &model.Task{
		ID:                  uuid.New(),
		ReleaseID:           release.ID,
		ServiceID:           spec.ID,
		AgentID:             spec.AgentID,
		Type:                model.TaskTypeDeployGreen,
		Status:              model.TaskStatusPending,
		Payload:             commondb.NewJSONB(payload),
		SensitiveCiphertext: ciphertext,
		SensitiveKeyVersion: keyVersion,
	}, nil
}

func validateDeploymentSpec(spec *dto.ServiceDeploymentSpec) error {
	if spec == nil {
		return business.NewBadRequest("service 配置不存在")
	}
	if spec.ContainerPort <= 0 || spec.ContainerPort > 65535 {
		return business.NewBadRequest("service.containerPort 非法，请先更新服务配置后再发布")
	}
	return nil
}

func (s *Service) newSwitchTask(release *model.Release, spec *dto.ServiceDeploymentSpec, taskType model.TaskType) (*model.Task, error) {
	payload := model.TaskPayload{
		ServiceID:               spec.ID,
		ServiceKey:              spec.ServiceKey,
		ImageRepo:               spec.ImageRepo,
		ImageTag:                release.ImageTag,
		CommitSHA:               release.CommitSHA,
		TraceID:                 release.TraceID,
		TargetSlot:              release.TargetSlot,
		CurrentLiveSlot:         release.PreviousLiveSlot,
		ContainerPort:           spec.ContainerPort,
		DockerHealthCheck:       spec.DockerHealthCheck,
		HTTPHealthPath:          firstNonEmpty(spec.HTTPHealthPath, "/health"),
		HTTPHealthHeaders:       cloneStringMap(spec.HTTPHealthHeaders),
		HTTPExpectedCode:        defaultInt(spec.HTTPExpectedCode, model.DefaultHTTPExpectedCode),
		HTTPTimeoutSecond:       defaultInt(spec.HTTPTimeoutSecond, model.DefaultHTTPTimeoutSecond),
		StartupGraceSecond:      defaultInt(spec.StartupGraceSecond, model.DefaultStartupGraceSecond),
		HTTPProbeTimeoutSecond:  defaultInt(spec.HTTPProbeTimeoutSecond, model.DefaultHTTPProbeTimeoutSecond),
		HTTPProbeIntervalSecond: defaultInt(spec.HTTPProbeIntervalSecond, model.DefaultHTTPProbeIntervalSecond),
		HTTPSuccessThreshold:    defaultInt(spec.HTTPSuccessThreshold, model.DefaultHTTPSuccessThreshold),
		BackendName:             servicecatalogapp.BackendNameForRelease(spec.ID, release.ID.String()),
		ServerName:              servicecatalogapp.ServerName(release.TargetSlot),
		PreviousServer:          servicecatalogapp.ServerName(release.PreviousLiveSlot),
		Command:                 spec.Command,
		Entrypoint:              spec.Entrypoint,
		Volumes:                 toModelVolumeMounts(spec.Volumes),
		PublishedPorts:          toModelPublishedPorts(spec.PublishedPorts),
	}
	ciphertext, keyVersion, plaintextSensitive, err := s.prepareTaskSensitive(spec, model.TaskSensitivePayload{Env: spec.Env})
	if err != nil {
		return nil, err
	}
	payload.Env = plaintextSensitive.Env
	return &model.Task{
		ID:                  uuid.New(),
		ReleaseID:           release.ID,
		ServiceID:           spec.ID,
		AgentID:             spec.AgentID,
		Type:                taskType,
		Status:              model.TaskStatusPending,
		Payload:             commondb.NewJSONB(payload),
		SensitiveCiphertext: ciphertext,
		SensitiveKeyVersion: keyVersion,
	}, nil
}

func (s *Service) newRollbackTask(release *model.Release, spec *dto.ServiceDeploymentSpec) (*model.Task, error) {
	payload := model.TaskPayload{
		ServiceID:               spec.ID,
		ServiceKey:              spec.ServiceKey,
		ImageRepo:               spec.ImageRepo,
		ImageTag:                release.ImageTag,
		CommitSHA:               release.CommitSHA,
		TraceID:                 release.TraceID,
		TargetSlot:              release.PreviousLiveSlot,
		CurrentLiveSlot:         spec.CurrentLiveSlot,
		ContainerPort:           spec.ContainerPort,
		DockerHealthCheck:       spec.DockerHealthCheck,
		HTTPHealthPath:          firstNonEmpty(spec.HTTPHealthPath, "/health"),
		HTTPHealthHeaders:       cloneStringMap(spec.HTTPHealthHeaders),
		HTTPExpectedCode:        defaultInt(spec.HTTPExpectedCode, model.DefaultHTTPExpectedCode),
		HTTPTimeoutSecond:       defaultInt(spec.HTTPTimeoutSecond, model.DefaultHTTPTimeoutSecond),
		StartupGraceSecond:      defaultInt(spec.StartupGraceSecond, model.DefaultStartupGraceSecond),
		HTTPProbeTimeoutSecond:  defaultInt(spec.HTTPProbeTimeoutSecond, model.DefaultHTTPProbeTimeoutSecond),
		HTTPProbeIntervalSecond: defaultInt(spec.HTTPProbeIntervalSecond, model.DefaultHTTPProbeIntervalSecond),
		HTTPSuccessThreshold:    defaultInt(spec.HTTPSuccessThreshold, model.DefaultHTTPSuccessThreshold),
		BackendName:             servicecatalogapp.BackendNameForRelease(spec.ID, release.ID.String()),
		ServerName:              servicecatalogapp.ServerName(release.PreviousLiveSlot),
		PreviousServer:          servicecatalogapp.ServerName(spec.CurrentLiveSlot),
		Command:                 spec.Command,
		Entrypoint:              spec.Entrypoint,
		Volumes:                 toModelVolumeMounts(spec.Volumes),
		PublishedPorts:          toModelPublishedPorts(spec.PublishedPorts),
	}
	ciphertext, keyVersion, plaintextSensitive, err := s.prepareTaskSensitive(spec, model.TaskSensitivePayload{Env: spec.Env})
	if err != nil {
		return nil, err
	}
	payload.Env = plaintextSensitive.Env
	return &model.Task{
		ID:                  uuid.New(),
		ReleaseID:           release.ID,
		ServiceID:           spec.ID,
		AgentID:             spec.AgentID,
		Type:                model.TaskTypeRollback,
		Status:              model.TaskStatusPending,
		Payload:             commondb.NewJSONB(payload),
		SensitiveCiphertext: ciphertext,
		SensitiveKeyVersion: keyVersion,
	}, nil
}

func (s *Service) prepareTaskSensitive(spec *dto.ServiceDeploymentSpec, sensitive model.TaskSensitivePayload) (string, string, model.TaskSensitivePayload, error) {
	if len(sensitive.Env) == 0 && strings.TrimSpace(sensitive.RegistrySecret) == "" {
		return "", "", model.TaskSensitivePayload{}, nil
	}
	if s.codec == nil {
		if !spec.EnvEncrypted && len(sensitive.Env) > 0 && strings.TrimSpace(sensitive.RegistrySecret) == "" {
			return "", "", model.TaskSensitivePayload{Env: sensitive.Env}, nil
		}
		return "", "", model.TaskSensitivePayload{}, business.NewErrorWithCode("service secret master key not configured", 500)
	}
	ciphertext, keyVersion, err := s.codec.EncryptJSON(sensitive)
	if err != nil {
		return "", "", model.TaskSensitivePayload{}, err
	}
	return ciphertext, keyVersion, model.TaskSensitivePayload{}, nil
}

func toReleaseOutput(release *model.Release) dto.ReleaseOutput {
	return dto.ReleaseOutput{
		ID:                       release.ID,
		ServiceID:                release.ServiceID,
		AgentID:                  release.AgentID,
		ImageRepo:                release.ImageRepo,
		ImageTag:                 release.ImageTag,
		CommitSHA:                release.CommitSHA,
		TriggeredBy:              release.TriggeredBy,
		TraceID:                  release.TraceID,
		Status:                   release.Status,
		TrafficPercent:           release.TrafficPercent,
		TargetSlot:               release.TargetSlot,
		PreviousLiveSlot:         release.PreviousLiveSlot,
		CurrentTaskID:            release.CurrentTaskID,
		SwitchConfirmed:          release.SwitchConfirmed,
		StickyCookieTTL:          servicecatalogapp.StickyCookieMaxAgeSec,
		CurrentReleaseHeaderName: servicecatalogapp.CurrentReleaseIDHeaderName,
		LiveReleaseHeaderName:    servicecatalogapp.LiveReleaseIDHeaderName,
		IsActive:                 release.Status.IsActive(),
		CreatedAt:                release.CreatedAt,
		UpdatedAt:                release.UpdatedAt,
		CompletedAt:              release.CompletedAt,
	}
}

func (s *Service) enrichReleaseOutput(release *model.Release) (dto.ReleaseOutput, error) {
	output := toReleaseOutput(release)
	if spec, err := s.services.GetSpecByID(release.ServiceID); err != nil {
		var statusCarrier interface{ GetStatusCode() int }
		if errors.As(err, &statusCarrier) && statusCarrier.GetStatusCode() == 404 {
			// Preserve historical release visibility even if the service has been removed.
		} else {
			return dto.ReleaseOutput{}, err
		}
	} else {
		output.VerificationURL = servicecatalogapp.BuildVerificationURL(spec.RouteHost, spec.RoutePathPrefix, release.ID.String())
		output.StickyCookieName = servicecatalogapp.StickyCookieName(spec.ServiceKey)
	}
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

func (s *Service) publishAgent(agentID string) error {
	if s.proxyConfigs == nil || strings.TrimSpace(agentID) == "" {
		return nil
	}
	return s.proxyConfigs.PublishAgent(agentID)
}

func (s *Service) completeSupersededRelease(current *model.Release, operator string) error {
	if current == nil || current.ServiceID == uuid.Nil || current.TargetSlot == 0 {
		return nil
	}
	releases, err := s.repo.ListReleases(0)
	if err != nil {
		return err
	}
	for i := range releases {
		item := releases[i]
		if item.ID == current.ID || item.ServiceID != current.ServiceID || item.TargetSlot != current.TargetSlot {
			continue
		}
		if item.Status != model.ReleaseStatusReadyToSwitch && item.Status != model.ReleaseStatusSwitched {
			continue
		}
		if item.TrafficPercent != 0 {
			continue
		}
		now := time.Now()
		item.Status = model.ReleaseStatusCompleted
		item.CompletedAt = &now
		if err := s.repo.UpdateRelease(&item); err != nil {
			return err
		}
		if err := s.repo.CreateAudit(newAudit("release", item.ID.String(), "release_superseded", item.TraceID, operator)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) completePreviousLiveRelease(current *model.Release, operator string, now time.Time) error {
	if current == nil || current.ServiceID == uuid.Nil || current.PreviousLiveSlot == 0 {
		return nil
	}
	instance, err := s.repo.GetRuntimeInstanceByServiceAndSlot(current.ServiceID, current.PreviousLiveSlot)
	if err != nil {
		return err
	}
	if instance == nil || instance.ReleaseID == uuid.Nil || instance.ReleaseID == current.ID {
		return nil
	}
	previous, err := s.repo.GetRelease(instance.ReleaseID)
	if err != nil {
		return err
	}
	if previous == nil {
		return nil
	}
	switch previous.Status {
	case model.ReleaseStatusReadyToSwitch, model.ReleaseStatusSwitched, model.ReleaseStatusCompleted:
	default:
		return nil
	}
	if previous.Status == model.ReleaseStatusCompleted && previous.TrafficPercent == 0 && previous.CompletedAt != nil {
		return nil
	}
	previous.Status = model.ReleaseStatusCompleted
	previous.TrafficPercent = 0
	if previous.CompletedAt == nil {
		previous.CompletedAt = &now
	}
	if err := s.repo.UpdateRelease(previous); err != nil {
		return err
	}
	return s.repo.CreateAudit(newAudit("release", previous.ID.String(), "release_superseded", previous.TraceID, operator))
}

func (s *Service) autoSkipQueuedBeforeStart(current *model.Release, operator string) error {
	if current == nil || current.ServiceID == uuid.Nil {
		return nil
	}
	releases, err := s.repo.ListQueuedBefore(current.ServiceID, current.CreatedAt, current.ID)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("operator=%s started_release_id=%s", strings.TrimSpace(operator), current.ID.String())
	for i := range releases {
		item := releases[i]
		now := time.Now()
		item.Status = model.ReleaseStatusSkipped
		item.CompletedAt = &now
		if err := s.repo.UpdateRelease(&item); err != nil {
			return err
		}
		if err := s.repo.CreateAudit(newAudit("release", item.ID.String(), "release_auto_skipped", item.TraceID, message)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recordLateTaskUpdate(release *model.Release, task *model.Task, update *grpcapi.TaskUpdate, reason string) error {
	if release == nil {
		return nil
	}
	message := fmt.Sprintf("reason=%s taskId=%s status=%s step=%s", reason, task.ID.String(), update.GetStatus().String(), update.GetStep())
	return s.repo.CreateAudit(newAudit("release", release.ID.String(), "late_task_update_ignored", release.TraceID, message))
}

func taskTimedOut(task *model.Task, now time.Time) bool {
	if task == nil {
		return false
	}
	lastUpdatedAt := task.UpdatedAt
	if lastUpdatedAt.IsZero() {
		lastUpdatedAt = task.CreatedAt
	}
	if lastUpdatedAt.IsZero() {
		return false
	}
	return lastUpdatedAt.Add(timeoutForTask(task)).Before(now)
}

func timeoutForTask(task *model.Task) time.Duration {
	if task == nil {
		return switchTaskTimeout
	}
	payload := getJSON(task.Payload)
	switch task.Type {
	case model.TaskTypeDeployGreen:
		return deployImagePullBudget +
			time.Duration(defaultInt(payload.StartupGraceSecond, model.DefaultStartupGraceSecond))*time.Second +
			time.Duration(defaultInt(payload.HTTPTimeoutSecond, model.DefaultHTTPTimeoutSecond))*time.Second +
			deployTimeoutBuffer
	case model.TaskTypeSwitchTraffic, model.TaskTypeRollback:
		return switchTaskTimeout
	case model.TaskTypeCleanupOld:
		return cleanupTaskTimeout
	default:
		return switchTaskTimeout
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

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
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

func truncateForAudit(value string) string {
	const maxLen = 512
	if len(value) <= maxLen {
		return value
	}
	return value[len(value)-maxLen:]
}

package application

import (
	"edge-pilot/internal/scheduler/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"edge-pilot/internal/shared/model"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/business"
	commondb "github.com/real-uangi/allingo/common/db"
)

type OnlineExecutor struct {
	ExecutorID      string
	Group           string
	LiveSlot        model.Slot
	ServiceInstance bool
	ServiceID       string
}

type RunDispatcher interface {
	ListOnlineExecutors(group string) []OnlineExecutor
	DispatchRun(executorID string, run *model.SchedulerJobRun) error
}

type Service struct {
	repo     domain.Repository
	auth     *config.AgentAuthConfig
	cfg      *config.SchedulerConfig
	liveSlot domain.LiveSlotResolver
}

func NewService(repo domain.Repository, auth *config.AgentAuthConfig, cfg *config.SchedulerConfig, liveSlot domain.LiveSlotResolver) *Service {
	if cfg == nil {
		cfg = config.LoadSchedulerConfig()
	}
	if auth == nil {
		auth = config.LoadAgentAuthConfig()
	}
	return &Service{repo: repo, auth: auth, cfg: cfg, liveSlot: liveSlot}
}

func (s *Service) CreateJob(req dto.UpsertSchedulerJobRequest) (*dto.SchedulerJobOutput, error) {
	entity, err := s.toJobEntity(nil, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateJob(entity); err != nil {
		return nil, err
	}
	out := toSchedulerJobOutput(entity)
	return &out, nil
}

func (s *Service) UpdateJob(id uuid.UUID, req dto.UpsertSchedulerJobRequest) (*dto.SchedulerJobOutput, error) {
	current, err := s.repo.GetJob(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, business.ErrNotFound
	}
	entity, err := s.toJobEntity(current, req)
	if err != nil {
		return nil, err
	}
	entity.ID = id
	entity.CreatedAt = current.CreatedAt
	if err := s.repo.UpdateJob(entity); err != nil {
		return nil, err
	}
	out := toSchedulerJobOutput(entity)
	return &out, nil
}

func (s *Service) GetJob(id uuid.UUID) (*dto.SchedulerJobOutput, error) {
	job, err := s.repo.GetJob(id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, business.ErrNotFound
	}
	out := toSchedulerJobOutput(job)
	return &out, nil
}

func (s *Service) ListJobs() ([]dto.SchedulerJobOutput, error) {
	jobs, err := s.repo.ListJobs()
	if err != nil {
		return nil, err
	}
	out := make([]dto.SchedulerJobOutput, 0, len(jobs))
	for i := range jobs {
		out = append(out, toSchedulerJobOutput(&jobs[i]))
	}
	return out, nil
}

func (s *Service) DeleteJob(id uuid.UUID) error {
	job, err := s.repo.GetJob(id)
	if err != nil {
		return err
	}
	if job == nil {
		return business.ErrNotFound
	}
	return s.repo.DeleteJob(id)
}

func (s *Service) SetJobEnabled(id uuid.UUID, enabled bool) (*dto.SchedulerJobOutput, error) {
	job, err := s.repo.GetJob(id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, business.ErrNotFound
	}
	job.Enabled = boolPtr(enabled)
	if err := s.repo.UpdateJob(job); err != nil {
		return nil, err
	}
	out := toSchedulerJobOutput(job)
	return &out, nil
}

func (s *Service) TriggerNow(id uuid.UUID, override map[string]any) (*dto.SchedulerRunOutput, error) {
	job, err := s.repo.GetJob(id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, business.ErrNotFound
	}
	now := time.Now().UTC()
	payload := jobPayload(job)
	for k, v := range override {
		payload[k] = v
	}
	run := &model.SchedulerJobRun{
		ID:              uuid.New(),
		JobID:           job.ID,
		TaskType:        job.TaskType,
		Payload:         commondb.NewJSONB(payload),
		Status:          model.SchedulerJobRunStatusPending,
		Attempt:         1,
		MaxRetries:      effectiveMaxRetries(job.MaxRetries, s.cfg.DefaultMaxRetries),
		LeaseTimeoutSec: normalizeLeaseTimeout(job.LeaseTimeoutSec, s.cfg.DefaultLeaseSec),
		ScheduledAt:     now,
		DispatchPolicy:  job.DispatchPolicy,
		ExecutorGroup:   job.ExecutorGroup,
	}
	run.IdempotencyKey = run.ID.String()
	if err := s.repo.CreateRun(run); err != nil {
		return nil, err
	}
	out := toSchedulerRunOutput(run)
	return &out, nil
}

func (s *Service) ListRuns(jobID uuid.UUID, limit int) ([]dto.SchedulerRunOutput, error) {
	runs, err := s.repo.ListRunsByJob(jobID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SchedulerRunOutput, 0, len(runs))
	for i := range runs {
		out = append(out, toSchedulerRunOutput(&runs[i]))
	}
	return out, nil
}

func (s *Service) ListAllRuns(limit int) ([]dto.SchedulerRunOutput, error) {
	runs, err := s.repo.ListAllRuns(limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SchedulerRunOutput, 0, len(runs))
	for i := range runs {
		out = append(out, toSchedulerRunOutput(&runs[i]))
	}
	return out, nil
}

func (s *Service) CreateExecutor(req dto.UpsertSchedulerExecutorRequest) (*dto.SchedulerExecutorOutput, error) {
	if strings.TrimSpace(req.ExecutorID) == "" {
		return nil, business.NewBadRequest("executorId required")
	}
	if strings.TrimSpace(req.Group) == "" {
		return nil, business.NewBadRequest("group required")
	}
	token, hash, err := s.auth.GenerateToken()
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	executor := &model.SchedulerExecutor{
		ID:              req.ExecutorID,
		TokenHash:       hash,
		Group:           req.Group,
		ChannelMode:     model.SchedulerExecutorChannelModeDirect,
		RelayAgentID:    "",
		RelayRoutingKey: "",
		Enabled:         boolPtr(enabled),
		LiveSlot:        model.Slot(req.LiveSlot),
		InstanceMeta:    commondb.NewJSONB(copyStringMap(req.Metadata)),
	}
	if err := s.repo.UpsertExecutor(executor); err != nil {
		return nil, err
	}
	out := toSchedulerExecutorOutput(executor)
	out.Token = token
	return &out, nil
}

func (s *Service) ResetExecutorToken(executorID string) (*dto.SchedulerExecutorOutput, error) {
	executor, err := s.repo.GetExecutor(executorID)
	if err != nil {
		return nil, err
	}
	if executor == nil {
		return nil, business.ErrNotFound
	}
	token, hash, err := s.auth.GenerateToken()
	if err != nil {
		return nil, err
	}
	executor.TokenHash = hash
	if err := s.repo.UpsertExecutor(executor); err != nil {
		return nil, err
	}
	out := toSchedulerExecutorOutput(executor)
	out.Token = token
	return &out, nil
}

func (s *Service) SetExecutorEnabled(executorID string, enabled bool) (*dto.SchedulerExecutorOutput, error) {
	executor, err := s.repo.GetExecutor(executorID)
	if err != nil {
		return nil, err
	}
	if executor == nil {
		return nil, business.ErrNotFound
	}
	executor.Enabled = boolPtr(enabled)
	if err := s.repo.UpsertExecutor(executor); err != nil {
		return nil, err
	}
	out := toSchedulerExecutorOutput(executor)
	return &out, nil
}

func (s *Service) ListExecutors() ([]dto.SchedulerExecutorOutput, error) {
	executors, err := s.repo.ListExecutors()
	if err != nil {
		return nil, err
	}
	out := make([]dto.SchedulerExecutorOutput, 0, len(executors))
	for i := range executors {
		out = append(out, toSchedulerExecutorOutput(&executors[i]))
	}
	return out, nil
}

func (s *Service) DeleteExecutor(executorID string) error {
	executor, err := s.repo.GetExecutor(executorID)
	if err != nil {
		return err
	}
	if executor == nil {
		return business.ErrNotFound
	}
	return s.repo.DeleteExecutor(executorID)
}

func (s *Service) AuthenticateExecutor(executorID string, token string, group string, liveSlot model.Slot, metadata map[string]string, relayAgentID string, relayRoutingKey string) (*model.SchedulerExecutor, error) {
	executor, err := s.repo.GetExecutor(executorID)
	if err != nil {
		return nil, err
	}
	if executor == nil || executor.Enabled == nil || !*executor.Enabled {
		return nil, business.ErrUnauthorized
	}
	if !s.auth.ValidateHash(executor.TokenHash, token) {
		return nil, business.ErrUnauthorized
	}
	if strings.TrimSpace(group) != "" && strings.TrimSpace(group) != executor.Group {
		return nil, business.ErrUnauthorized
	}
	now := time.Now().UTC()
	if strings.TrimSpace(relayAgentID) == "" {
		executor.ChannelMode = model.SchedulerExecutorChannelModeDirect
		executor.RelayAgentID = ""
		executor.RelayRoutingKey = ""
	} else {
		executor.ChannelMode = model.SchedulerExecutorChannelModeAgentRelay
		executor.RelayAgentID = strings.TrimSpace(relayAgentID)
		executor.RelayRoutingKey = strings.TrimSpace(relayRoutingKey)
	}
	if liveSlot != 0 {
		executor.LiveSlot = liveSlot
	}
	executor.LastSeenAt = &now
	if len(metadata) > 0 {
		executor.InstanceMeta = commondb.NewJSONB(sanitizeExecutorMetadata(metadata))
	}
	if err := s.repo.UpsertExecutor(executor); err != nil {
		return nil, err
	}
	return executor, nil
}

func (s *Service) RegisterServiceInstanceExecutor(executorID string, group string, liveSlot model.Slot, metadata map[string]string, relayAgentID string, relayRoutingKey string) (*model.SchedulerExecutor, error) {
	executorID = strings.TrimSpace(executorID)
	group = strings.TrimSpace(group)
	relayAgentID = strings.TrimSpace(relayAgentID)
	relayRoutingKey = strings.TrimSpace(relayRoutingKey)
	if executorID == "" {
		return nil, business.NewBadRequest("executorId required")
	}
	if group == "" {
		return nil, business.NewBadRequest("group required")
	}
	if relayAgentID == "" {
		return nil, business.NewBadRequest("relayAgentID required")
	}
	cleanMeta := sanitizeExecutorMetadata(metadata)
	serviceID, err := serviceIDFromServiceInstanceMetadata(cleanMeta)
	if err != nil {
		return nil, err
	}
	releaseID := strings.TrimSpace(cleanMeta["release_id"])
	if releaseID == "" {
		return nil, business.NewBadRequest("release_id required in metadata")
	}
	containerID := strings.TrimSpace(cleanMeta["container_id"])
	if containerID == "" {
		return nil, business.NewBadRequest("container_id required in metadata")
	}
	expectedExecutorID := schedulerServiceInstanceExecutorID(serviceID.String(), releaseID, liveSlot, containerID)
	if expectedExecutorID == "" || executorID != expectedExecutorID {
		return nil, business.NewBadRequest("executorId mismatch with service instance metadata")
	}
	if relayRoutingKey != "" && relayRoutingKey != executorID {
		return nil, business.ErrUnauthorized
	}
	existing, err := s.repo.GetExecutor(executorID)
	if err != nil {
		return nil, err
	}
	if existing != nil && strings.TrimSpace(existing.RelayAgentID) != "" && strings.TrimSpace(existing.RelayAgentID) != relayAgentID {
		return nil, business.ErrUnauthorized
	}
	now := time.Now().UTC()
	enabled := true
	executor := &model.SchedulerExecutor{
		ID:              executorID,
		TokenHash:       "agent-relay-service-instance",
		Group:           group,
		ChannelMode:     model.SchedulerExecutorChannelModeAgentRelay,
		RelayAgentID:    relayAgentID,
		RelayRoutingKey: relayRoutingKey,
		Enabled:         &enabled,
		LiveSlot:        liveSlot,
		LastSeenAt:      &now,
		InstanceMeta:    commondb.NewJSONB(cleanMeta),
	}
	if err := s.repo.UpsertExecutor(executor); err != nil {
		return nil, err
	}
	return executor, nil
}

func (s *Service) HeartbeatExecutor(executorID string) error {
	return s.repo.MarkExecutorSeen(executorID, time.Now().UTC())
}

func (s *Service) MarkRunRunning(runID uuid.UUID, executorID string) error {
	run, err := s.repo.GetRun(runID)
	if err != nil {
		return err
	}
	if run == nil {
		return business.ErrNotFound
	}
	now := time.Now().UTC()
	run.Status = model.SchedulerJobRunStatusRunning
	run.StartedAt = &now
	run.LeasedBy = executorID
	return s.repo.UpdateRun(run)
}

func (s *Service) RenewRunLease(runID uuid.UUID, executorID string) error {
	run, err := s.repo.GetRun(runID)
	if err != nil {
		return err
	}
	if run == nil {
		return business.ErrNotFound
	}
	if run.LeasedBy != executorID {
		return business.NewErrorWithCode("lease owner mismatch", 409)
	}
	now := time.Now().UTC()
	lease := now.Add(time.Duration(effectiveLeaseTimeout(run, s.cfg.DefaultLeaseSec)) * time.Second)
	run.LeaseExpiresAt = &lease
	return s.repo.UpdateRun(run)
}

func (s *Service) CompleteRun(runID uuid.UUID, executorID string, success bool, retryable bool, errMsg string) error {
	run, err := s.repo.GetRun(runID)
	if err != nil {
		return err
	}
	if run == nil {
		return business.ErrNotFound
	}
	if run.LeasedBy != executorID {
		return business.NewErrorWithCode("lease owner mismatch", 409)
	}
	now := time.Now().UTC()
	if success {
		run.Status = model.SchedulerJobRunStatusSucceeded
		run.CompletedAt = &now
		run.ErrorMessage = ""
		return s.repo.UpdateRun(run)
	}
	run.ErrorMessage = strings.TrimSpace(errMsg)
	if retryable && run.Attempt <= run.MaxRetries {
		run.Status = model.SchedulerJobRunStatusRetryWaiting
		run.NextRetryAt = timePtr(now.Add(retryBackoff(run.Attempt)))
		run.LeaseExpiresAt = nil
		run.LeasedBy = ""
		run.Attempt++
		return s.repo.UpdateRun(run)
	}
	run.Status = model.SchedulerJobRunStatusFailed
	run.CompletedAt = &now
	run.LeaseExpiresAt = nil
	run.LeasedBy = ""
	return s.repo.UpdateRun(run)
}

func (s *Service) EnqueueDueJobs(now time.Time) error {
	dueJobs, err := s.repo.ListJobsDue(now.UTC(), s.cfg.DispatchBatchSize)
	if err != nil {
		return err
	}
	for i := range dueJobs {
		job := dueJobs[i]
		if err := s.repo.WithTx(func(tx domain.Repository) error {
			fresh, err := tx.GetJob(job.ID)
			if err != nil {
				return err
			}
			if fresh == nil || fresh.Enabled == nil || !*fresh.Enabled || fresh.NextRunAt == nil || fresh.NextRunAt.After(now.UTC()) {
				return nil
			}
			scheduled := fresh.NextRunAt.UTC()
			run := &model.SchedulerJobRun{
				ID:              uuid.New(),
				JobID:           fresh.ID,
				TaskType:        fresh.TaskType,
				Payload:         commondb.NewJSONB(jobPayload(fresh)),
				Status:          model.SchedulerJobRunStatusPending,
				Attempt:         1,
				MaxRetries:      effectiveMaxRetries(fresh.MaxRetries, s.cfg.DefaultMaxRetries),
				LeaseTimeoutSec: normalizeLeaseTimeout(fresh.LeaseTimeoutSec, s.cfg.DefaultLeaseSec),
				ScheduledAt:     scheduled,
				DispatchPolicy:  fresh.DispatchPolicy,
				ExecutorGroup:   fresh.ExecutorGroup,
			}
			run.IdempotencyKey = run.ID.String()
			if err := tx.CreateRun(run); err != nil {
				return err
			}
			next, enabled, err := s.computeNextSchedule(fresh, scheduled)
			if err != nil {
				return err
			}
			fresh.NextRunAt = next
			fresh.Enabled = boolPtr(enabled)
			return tx.UpdateJob(fresh)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) DispatchDueRuns(now time.Time, dispatcher RunDispatcher) error {
	if dispatcher == nil {
		return nil
	}
	runs, err := s.repo.ListDispatchableRuns(now.UTC(), s.cfg.DispatchBatchSize)
	if err != nil {
		return err
	}
	for i := range runs {
		run := runs[i]
		executorID, leaseExpiresAt, err := s.pickExecutorAndClaim(&run, now.UTC(), dispatcher)
		if err != nil {
			stored, getErr := s.repo.GetRun(run.ID)
			if getErr == nil && stored != nil {
				stored.ErrorMessage = err.Error()
				_ = s.repo.UpdateRun(stored)
			}
			continue
		}
		if executorID == "" {
			continue
		}
		if dispatchErr := dispatcher.DispatchRun(executorID, &run); dispatchErr != nil {
			stored, getErr := s.repo.GetRun(run.ID)
			if getErr != nil || stored == nil {
				continue
			}
			stored.Status = model.SchedulerJobRunStatusPending
			stored.LeasedBy = ""
			stored.LeaseExpiresAt = nil
			stored.ErrorMessage = dispatchErr.Error()
			_ = s.repo.UpdateRun(stored)
			continue
		}
		stored, getErr := s.repo.GetRun(run.ID)
		if getErr != nil || stored == nil {
			continue
		}
		stored.LeaseExpiresAt = &leaseExpiresAt
		stored.LeasedBy = executorID
		_ = s.repo.UpdateRun(stored)
	}
	return nil
}

func (s *Service) pickExecutorAndClaim(run *model.SchedulerJobRun, now time.Time, dispatcher RunDispatcher) (string, time.Time, error) {
	online := dispatcher.ListOnlineExecutors(run.ExecutorGroup)
	if len(online) == 0 {
		return "", time.Time{}, domain.ErrExecutorOffline
	}
	sort.Slice(online, func(i, j int) bool { return online[i].ExecutorID < online[j].ExecutorID })

	chosen, err := s.pickExecutorID(run, online)
	if err != nil {
		return "", time.Time{}, err
	}
	if chosen == "" {
		return "", time.Time{}, domain.ErrExecutorOffline
	}
	leaseExpiresAt := now.Add(time.Duration(effectiveLeaseTimeout(run, s.cfg.DefaultLeaseSec)) * time.Second)
	claimed, err := s.repo.ClaimRun(run.ID, chosen, leaseExpiresAt, now)
	if err != nil {
		return "", time.Time{}, err
	}
	if !claimed {
		return "", time.Time{}, nil
	}
	return chosen, leaseExpiresAt, nil
}

func (s *Service) pickExecutorID(run *model.SchedulerJobRun, online []OnlineExecutor) (string, error) {
	if run.DispatchPolicy == model.SchedulerDispatchPolicyFixedLiveSlot {
		runServiceID, err := serviceIDFromPayload(runPayload(run))
		if err != nil {
			return "", err
		}
		targetSlot := model.SlotBlue
		if s.liveSlot != nil {
			targetSlot, err = s.liveSlot.ResolveLiveSlot(runServiceID)
			if err != nil {
				return "", err
			}
		}
		for i := range online {
			if online[i].LiveSlot != targetSlot || !online[i].ServiceInstance {
				continue
			}
			onlineServiceID, ok := serviceInstanceServiceID(online[i])
			if !ok || onlineServiceID != runServiceID {
				continue
			}
			return online[i].ExecutorID, nil
		}
		for i := range online {
			if online[i].LiveSlot == targetSlot && !online[i].ServiceInstance {
				return online[i].ExecutorID, nil
			}
		}
		return "", domain.ErrExecutorOffline
	}
	cursor, err := s.repo.GetDispatchCursor(run.JobID, run.ExecutorGroup)
	if err != nil {
		return "", err
	}
	start := 0
	if cursor != nil && cursor.LastExecutorID != "" {
		for i := range online {
			if online[i].ExecutorID == cursor.LastExecutorID {
				start = (i + 1) % len(online)
				break
			}
		}
	}
	chosen := online[start].ExecutorID
	if cursor == nil {
		cursor = &model.SchedulerDispatchCursor{JobID: run.JobID, ExecutorGroup: run.ExecutorGroup, LastExecutorID: chosen}
	} else {
		cursor.LastExecutorID = chosen
	}
	if err := s.repo.SaveDispatchCursor(cursor); err != nil {
		return "", err
	}
	return chosen, nil
}

func (s *Service) computeNextSchedule(job *model.SchedulerJob, from time.Time) (*time.Time, bool, error) {
	switch job.ScheduleKind {
	case model.SchedulerScheduleKindOneTime:
		return nil, false, nil
	case model.SchedulerScheduleKindCron:
		next, err := nextCronTimeUTC(job.CronExpr, from.UTC())
		if err != nil {
			return nil, false, err
		}
		return &next, true, nil
	default:
		return nil, false, business.NewBadRequest("unknown schedule kind")
	}
}

func (s *Service) toJobEntity(current *model.SchedulerJob, req dto.UpsertSchedulerJobRequest) (*model.SchedulerJob, error) {
	now := time.Now().UTC()
	kind, err := parseScheduleKind(req.ScheduleKind)
	if err != nil {
		return nil, err
	}
	policy, err := parseDispatchPolicy(req.DispatchPolicy)
	if err != nil {
		return nil, err
	}
	taskType := strings.TrimSpace(req.TaskType)
	if taskType == "" {
		return nil, business.NewBadRequest("taskType required")
	}
	if policy == model.SchedulerDispatchPolicyFixedLiveSlot && !isReleaseLinkedTaskType(taskType) {
		return nil, business.NewBadRequest("fixed_live_slot policy requires release-linked taskType")
	}
	nextRun, err := calcInitialNextRun(kind, strings.TrimSpace(req.CronExpr), req.RunAt, now)
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if current != nil {
		if req.ScheduleKind == "" {
			kind = current.ScheduleKind
		}
		if req.DispatchPolicy == "" {
			policy = current.DispatchPolicy
		}
		if req.Enabled == nil {
			enabled = current.Enabled != nil && *current.Enabled
		}
	}
	entity := &model.SchedulerJob{
		Name:            strings.TrimSpace(req.Name),
		TaskType:        taskType,
		Payload:         commondb.NewJSONB(copyAnyMap(req.Payload)),
		ScheduleKind:    kind,
		CronExpr:        strings.TrimSpace(req.CronExpr),
		RunAt:           req.RunAt,
		NextRunAt:       nextRun,
		Enabled:         boolPtr(enabled),
		DispatchPolicy:  policy,
		ExecutorGroup:   strings.TrimSpace(req.ExecutorGroup),
		LeaseTimeoutSec: normalizeLeaseTimeout(req.LeaseTimeoutSec, s.cfg.DefaultLeaseSec),
		MaxRetries:      effectiveMaxRetries(req.MaxRetries, s.cfg.DefaultMaxRetries),
		Metadata:        commondb.NewJSONB(copyStringMap(req.Metadata)),
	}
	if current != nil {
		entity.ID = current.ID
		entity.LastDispatchedSeq = current.LastDispatchedSeq
		if entity.Name == "" {
			entity.Name = current.Name
		}
		if entity.ExecutorGroup == "" {
			entity.ExecutorGroup = current.ExecutorGroup
		}
		if req.Payload == nil {
			entity.Payload = current.Payload
		}
		if req.Metadata == nil {
			entity.Metadata = current.Metadata
		}
		if req.RunAt == nil && kind == model.SchedulerScheduleKindOneTime {
			entity.RunAt = current.RunAt
			entity.NextRunAt = current.NextRunAt
		}
		if req.CronExpr == "" && kind == model.SchedulerScheduleKindCron {
			entity.CronExpr = current.CronExpr
			next, nErr := nextCronTimeUTC(entity.CronExpr, now)
			if nErr != nil {
				return nil, nErr
			}
			entity.NextRunAt = &next
		}
	}
	if entity.Name == "" {
		return nil, business.NewBadRequest("name required")
	}
	if entity.ExecutorGroup == "" {
		return nil, business.NewBadRequest("executorGroup required")
	}
	return entity, nil
}

func parseScheduleKind(raw string) (model.SchedulerScheduleKind, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "one_time", "", "onetime":
		return model.SchedulerScheduleKindOneTime, nil
	case "cron":
		return model.SchedulerScheduleKindCron, nil
	default:
		return 0, business.NewBadRequest("scheduleKind must be one_time or cron")
	}
}

func parseDispatchPolicy(raw string) (model.SchedulerDispatchPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "round_robin", "roundrobin":
		return model.SchedulerDispatchPolicyRoundRobin, nil
	case "fixed_live_slot", "fixedliveslot":
		return model.SchedulerDispatchPolicyFixedLiveSlot, nil
	default:
		return 0, business.NewBadRequest("dispatchPolicy must be round_robin or fixed_live_slot")
	}
}

func calcInitialNextRun(kind model.SchedulerScheduleKind, cronExpr string, runAt *time.Time, now time.Time) (*time.Time, error) {
	switch kind {
	case model.SchedulerScheduleKindOneTime:
		if runAt == nil {
			r := now.UTC()
			return &r, nil
		}
		r := runAt.UTC()
		return &r, nil
	case model.SchedulerScheduleKindCron:
		if strings.TrimSpace(cronExpr) == "" {
			return nil, business.NewBadRequest("cronExpr required for cron schedule")
		}
		next, err := nextCronTimeUTC(cronExpr, now.UTC())
		if err != nil {
			return nil, business.NewBadRequest(err.Error())
		}
		return &next, nil
	default:
		return nil, business.NewBadRequest("unsupported schedule kind")
	}
}

func isReleaseLinkedTaskType(taskType string) bool {
	clean := strings.ToLower(strings.TrimSpace(taskType))
	return strings.HasPrefix(clean, "release.") || strings.HasPrefix(clean, "release_")
}

func effectiveMaxRetries(v int, fallback int) int {
	if v < 0 {
		return 0
	}
	if v == 0 {
		if fallback < 0 {
			return 0
		}
		return fallback
	}
	return v
}

func normalizeLeaseTimeout(v int, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

func retryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	s := 5
	for i := 1; i < attempt && i < 6; i++ {
		s *= 2
	}
	return time.Duration(s) * time.Second
}

func effectiveLeaseTimeout(run *model.SchedulerJobRun, fallback int) int {
	if run == nil {
		return fallback
	}
	if run.LeaseTimeoutSec > 0 {
		return run.LeaseTimeoutSec
	}
	jobLease := 0
	if run.Payload != nil {
		if v, ok := run.Payload.Get()["leaseTimeoutSec"]; ok {
			switch val := v.(type) {
			case int:
				jobLease = val
			case float64:
				jobLease = int(val)
			}
		}
	}
	if jobLease > 0 {
		return jobLease
	}
	if fallback <= 0 {
		return 60
	}
	return fallback
}

func serviceIDFromPayload(payload map[string]any) (uuid.UUID, error) {
	raw, ok := payload["serviceId"]
	if !ok {
		raw = payload["serviceID"]
	}
	if raw == nil {
		return uuid.Nil, business.NewBadRequest("serviceId required in payload for fixed_live_slot")
	}
	s, ok := raw.(string)
	if !ok {
		return uuid.Nil, business.NewBadRequest("serviceId must be string")
	}
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid serviceId: %w", err)
	}
	return id, nil
}

func serviceIDFromServiceInstanceMetadata(metadata map[string]string) (uuid.UUID, error) {
	raw := strings.TrimSpace(metadata["service_id"])
	if raw == "" {
		return uuid.Nil, business.NewBadRequest("service_id required in metadata")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, business.NewBadRequest("service_id must be valid uuid")
	}
	return id, nil
}

func schedulerServiceInstanceExecutorID(serviceID string, releaseID string, slot model.Slot, containerID string) string {
	serviceID = strings.TrimSpace(serviceID)
	releaseID = strings.TrimSpace(releaseID)
	containerID = strings.TrimSpace(containerID)
	if serviceID == "" || releaseID == "" || containerID == "" {
		return ""
	}
	return fmt.Sprintf("svc:%s:rel:%s:slot:%s:ctr:%s", serviceID, releaseID, schedulerSlotToken(slot), shortContainerID(containerID))
}

func schedulerSlotToken(slot model.Slot) string {
	switch slot {
	case model.SlotBlue:
		return "blue"
	case model.SlotGreen:
		return "green"
	default:
		return "unknown"
	}
}

func shortContainerID(containerID string) string {
	containerID = strings.TrimSpace(containerID)
	if len(containerID) <= 12 {
		return containerID
	}
	return containerID[:12]
}

func serviceInstanceServiceID(executor OnlineExecutor) (uuid.UUID, bool) {
	raw := strings.TrimSpace(executor.ServiceID)
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func jobPayload(job *model.SchedulerJob) map[string]any {
	if job == nil || job.Payload == nil {
		return map[string]any{}
	}
	return copyAnyMap(job.Payload.Get())
}

func runPayload(run *model.SchedulerJobRun) map[string]any {
	if run == nil || run.Payload == nil {
		return map[string]any{}
	}
	return copyAnyMap(run.Payload.Get())
}

func toSchedulerJobOutput(job *model.SchedulerJob) dto.SchedulerJobOutput {
	payload := map[string]any{}
	if job.Payload != nil {
		payload = copyAnyMap(job.Payload.Get())
	}
	metadata := map[string]string{}
	if job.Metadata != nil {
		metadata = copyStringMap(job.Metadata.Get())
	}
	return dto.SchedulerJobOutput{
		ID:              job.ID,
		Name:            job.Name,
		TaskType:        job.TaskType,
		Payload:         payload,
		ScheduleKind:    job.ScheduleKind,
		CronExpr:        job.CronExpr,
		RunAt:           job.RunAt,
		NextRunAt:       job.NextRunAt,
		Enabled:         job.Enabled,
		DispatchPolicy:  job.DispatchPolicy,
		ExecutorGroup:   job.ExecutorGroup,
		LeaseTimeoutSec: job.LeaseTimeoutSec,
		MaxRetries:      job.MaxRetries,
		Metadata:        metadata,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
	}
}

func toSchedulerRunOutput(run *model.SchedulerJobRun) dto.SchedulerRunOutput {
	payload := map[string]any{}
	if run.Payload != nil {
		payload = copyAnyMap(run.Payload.Get())
	}
	return dto.SchedulerRunOutput{
		ID:             run.ID,
		JobID:          run.JobID,
		TaskType:       run.TaskType,
		Payload:        payload,
		Status:         run.Status,
		Attempt:        run.Attempt,
		MaxRetries:     run.MaxRetries,
		ScheduledAt:    run.ScheduledAt,
		DispatchedAt:   run.DispatchedAt,
		StartedAt:      run.StartedAt,
		CompletedAt:    run.CompletedAt,
		LeaseExpiresAt: run.LeaseExpiresAt,
		LeasedBy:       run.LeasedBy,
		NextRetryAt:    run.NextRetryAt,
		ErrorMessage:   run.ErrorMessage,
		CreatedAt:      run.CreatedAt,
		UpdatedAt:      run.UpdatedAt,
	}
}

func toSchedulerExecutorOutput(executor *model.SchedulerExecutor) dto.SchedulerExecutorOutput {
	meta := map[string]string{}
	if executor.InstanceMeta != nil {
		meta = copyStringMap(executor.InstanceMeta.Get())
	}
	return dto.SchedulerExecutorOutput{
		ID:              executor.ID,
		Group:           executor.Group,
		ChannelMode:     executor.ChannelMode,
		RelayAgentID:    executor.RelayAgentID,
		RelayRoutingKey: executor.RelayRoutingKey,
		Enabled:         executor.Enabled,
		LastSeenAt:      executor.LastSeenAt,
		LiveSlot:        executor.LiveSlot,
		InstanceMeta:    meta,
		CreatedAt:       executor.CreatedAt,
		UpdatedAt:       executor.UpdatedAt,
	}
}

func copyAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sanitizeExecutorMetadata(in map[string]string) map[string]string {
	clean := copyStringMap(in)
	for k := range clean {
		if strings.EqualFold(strings.TrimSpace(k), "relay_token") {
			delete(clean, k)
		}
	}
	return clean
}

func boolPtr(v bool) *bool {
	return &v
}

func timePtr(v time.Time) *time.Time {
	return &v
}

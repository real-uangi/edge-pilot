package taskexec

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/real-uangi/edge-pilot/internal/agent/application/containerindex"
	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"github.com/real-uangi/allingo/common/log"
)

const (
	defaultLogTailLines   = 200
	defaultLogMaxBytes    = 64 * 1024
	stepHealthCheckFailed = "health_check_failed"
	stepStartupGrace      = "startup_grace_started"
	stepHealthProbeRetry  = "health_probe_retry"
	stepContainerWaiting  = "container_reused_waiting"
)

type TaskFailureDiagnostic struct {
	ContainerID      string
	DockerHealth     string
	FailureLogs      string
	CleanupCompleted bool
}

type StartupManagedContainerScanStats struct {
	Scanned   int
	Removed   int
	Preserved int
	Failed    int
}

type Executor struct {
	cfg       *config.AgentRuntimeConfig
	docker    agentdomain.DockerRuntime
	proxy     agentdomain.ProxyRuntime
	index     *containerindex.ManagedContainerIndex
	httpProbe func(string, string, map[string]string, int, int) error
	logger    *log.StdLogger
}

type TaskExecutionError struct {
	Step       string
	Err        error
	Diagnostic *TaskFailureDiagnostic
}

func (e *TaskExecutionError) Error() string {
	return e.Err.Error()
}

func (e *TaskExecutionError) Unwrap() error {
	return e.Err
}

func NewExecutor(cfg *config.AgentRuntimeConfig, docker agentdomain.DockerRuntime, proxy agentdomain.ProxyRuntime, index *containerindex.ManagedContainerIndex) *Executor {
	return &Executor{
		cfg:       cfg,
		docker:    docker,
		proxy:     proxy,
		index:     index,
		httpProbe: probeHTTP,
		logger:    log.NewStdLogger("agent.executor"),
	}
}

func (e *Executor) Execute(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	if err := report(&grpcapi.TaskUpdate{
		TaskId: task.GetTaskId(),
		Status: grpcapi.TaskStatus_TASK_STATUS_RUNNING,
		Step:   "accepted",
	}); err != nil {
		return err
	}

	switch task.GetType() {
	case grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN:
		return e.executeDeploy(ctx, task, report)
	case grpcapi.TaskType_TASK_TYPE_SWITCH_TRAFFIC, grpcapi.TaskType_TASK_TYPE_ROLLBACK:
		return e.executeTrafficSwitch(ctx, task, report)
	case grpcapi.TaskType_TASK_TYPE_CLEANUP_OLD:
		return report(&grpcapi.TaskUpdate{
			TaskId: task.GetTaskId(),
			Status: grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
			Step:   "noop",
		})
	default:
		return fmt.Errorf("unknown task type: %s", task.GetType())
	}
}

func (e *Executor) CollectStats(ctx context.Context) ([]*grpcapi.BackendStatPoint, error) {
	return e.proxy.ShowStats(ctx)
}

func (e *Executor) executeDeploy(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	if err := e.proxy.EnsureReady(ctx); err != nil {
		return &TaskExecutionError{Step: "proxy_stack_not_ready", Err: err}
	}
	normalizeHealthConfig(task)
	imageRef := task.GetImageRepo() + ":" + task.GetImageTag()
	if err := e.docker.EnsureImage(ctx, imageRef, task); err != nil {
		return &TaskExecutionError{Step: "image_pull_failed", Err: err}
	}
	if err := report(&grpcapi.TaskUpdate{
		TaskId: task.GetTaskId(),
		Status: grpcapi.TaskStatus_TASK_STATUS_RUNNING,
		Step:   "image_pulled",
	}); err != nil {
		return err
	}
	runtime, decision, err := e.ensureDeployContainer(ctx, task)
	if err != nil {
		return err
	}
	switch decision {
	case deployDecisionReuseHealthy:
		return report(&grpcapi.TaskUpdate{
			TaskId:        task.GetTaskId(),
			Status:        grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
			Step:          "healthy",
			ContainerId:   runtime.ContainerID,
			ListenAddress: runtime.ListenAddress,
			Slot:          task.GetTargetSlot(),
			ServerName:    task.GetServerName(),
		})
	case deployDecisionWaitExisting:
		if err := report(&grpcapi.TaskUpdate{
			TaskId:        task.GetTaskId(),
			Status:        grpcapi.TaskStatus_TASK_STATUS_RUNNING,
			Step:          stepContainerWaiting,
			ContainerId:   runtime.ContainerID,
			ListenAddress: runtime.ListenAddress,
			Slot:          task.GetTargetSlot(),
			ServerName:    task.GetServerName(),
		}); err != nil {
			return err
		}
	default:
		runtime, err = e.docker.DeployContainer(ctx, task)
		if err != nil {
			return err
		}
		if err := report(&grpcapi.TaskUpdate{
			TaskId:        task.GetTaskId(),
			Status:        grpcapi.TaskStatus_TASK_STATUS_RUNNING,
			Step:          "container_started",
			ContainerId:   runtime.ContainerID,
			ListenAddress: runtime.ListenAddress,
			Slot:          task.GetTargetSlot(),
			ServerName:    task.GetServerName(),
		}); err != nil {
			return err
		}
	}

	if err := e.waitForHealth(ctx, task, runtime, report); err != nil {
		return e.wrapTaskFailure(ctx, runtime, err)
	}
	return report(&grpcapi.TaskUpdate{
		TaskId:        task.GetTaskId(),
		Status:        grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
		Step:          "healthy",
		ContainerId:   runtime.ContainerID,
		ListenAddress: runtime.ListenAddress,
		Slot:          task.GetTargetSlot(),
		ServerName:    task.GetServerName(),
	})
}

func (e *Executor) executeTrafficSwitch(ctx context.Context, task *grpcapi.TaskCommand, report func(*grpcapi.TaskUpdate) error) error {
	if err := e.proxy.EnsureReady(ctx); err != nil {
		return &TaskExecutionError{Step: "proxy_stack_not_ready", Err: err}
	}
	if removed, err := e.cleanupManagedContainers(ctx, task); err != nil {
		_ = report(&grpcapi.TaskUpdate{
			TaskId:       task.GetTaskId(),
			Status:       grpcapi.TaskStatus_TASK_STATUS_RUNNING,
			Step:         "cleanup_failed",
			ErrorMessage: err.Error(),
		})
	} else if removed > 0 {
		_ = report(&grpcapi.TaskUpdate{
			TaskId: task.GetTaskId(),
			Status: grpcapi.TaskStatus_TASK_STATUS_RUNNING,
			Step:   fmt.Sprintf("cleanup_pruned:%d", removed),
		})
	}
	return report(&grpcapi.TaskUpdate{
		TaskId:     task.GetTaskId(),
		Status:     grpcapi.TaskStatus_TASK_STATUS_SUCCEEDED,
		Step:       "traffic_switched",
		Slot:       task.GetTargetSlot(),
		ServerName: task.GetServerName(),
	})
}

type deployDecision int

const (
	deployDecisionStartNew deployDecision = iota + 1
	deployDecisionReuseHealthy
	deployDecisionWaitExisting
)

func (e *Executor) ensureDeployContainer(ctx context.Context, task *grpcapi.TaskCommand) (*agentdomain.ContainerRuntime, deployDecision, error) {
	identity := agentdomain.ManagedContainerIdentity{
		AgentID:    task.GetAgentId(),
		ServiceKey: task.GetServiceKey(),
		ReleaseID:  task.GetReleaseId(),
		Slot:       task.GetTargetSlot(),
	}
	existing, err := e.findManagedContainer(ctx, identity)
	if err != nil {
		return nil, deployDecisionStartNew, err
	}
	if existing == nil {
		if removed, err := e.cleanupStaleContainersOnSlot(ctx, task.GetAgentId(), task.GetServiceKey(), task.GetTargetSlot()); err != nil {
			e.logger.Errorf(err, "stale container cleanup on slot failed: agentId=%s serviceKey=%s slot=%s", task.GetAgentId(), task.GetServiceKey(), task.GetTargetSlot().String())
		} else if removed > 0 {
			e.logger.Infof("cleaned up %d stale containers on slot before deploying release: agentId=%s serviceKey=%s slot=%s", removed, task.GetAgentId(), task.GetServiceKey(), task.GetTargetSlot().String())
		}
		return nil, deployDecisionStartNew, nil
	}
	if !existing.Managed || existing.AgentID != task.GetAgentId() {
		name := agentdomain.ManagedContainerNameForTask(task.GetServiceKey(), task.GetReleaseId(), task.GetTargetSlot())
		return nil, deployDecisionStartNew, &TaskExecutionError{
			Step: "managed_container_conflict",
			Err:  fmt.Errorf("managed container conflict: %s exists but is not owned by agent %s", name, task.GetAgentId()),
		}
	}

	status, statusErr := e.docker.InspectContainer(ctx, existing.ContainerID)
	if statusErr != nil {
		return nil, deployDecisionStartNew, statusErr
	}
	if status != nil && status.Terminal() {
		if err := e.docker.RemoveContainer(ctx, existing.ContainerID); err != nil {
			return nil, deployDecisionStartNew, err
		}
		return nil, deployDecisionStartNew, nil
	}

	listenAddress, err := e.docker.ResolveListenAddress(ctx, existing.ContainerID, int(task.GetContainerPort()))
	if err != nil {
		listenAddress = ""
	}
	runtime := &agentdomain.ContainerRuntime{
		ContainerID:   existing.ContainerID,
		ListenAddress: listenAddress,
	}
	if existing.ReleaseID == task.GetReleaseId() {
		if listenAddress != "" {
			if err := e.verifyHealth(ctx, task, status, listenAddress); err == nil {
				return runtime, deployDecisionReuseHealthy, nil
			}
		}
		if status != nil && status.Running {
			return runtime, deployDecisionWaitExisting, nil
		}
	}
	if err := e.docker.RemoveContainer(ctx, existing.ContainerID); err != nil {
		return nil, deployDecisionStartNew, err
	}
	return nil, deployDecisionStartNew, nil
}

func (e *Executor) findManagedContainer(ctx context.Context, identity agentdomain.ManagedContainerIdentity) (*agentdomain.ManagedContainer, error) {
	if e.index != nil {
		if err := e.index.RefreshNow(ctx); err != nil {
			e.logger.Errorf(err, "managed container index refresh before deploy lookup failed: agentId=%s serviceKey=%s releaseId=%s", identity.AgentID, identity.ServiceKey, identity.ReleaseID)
		} else {
			item, err := e.index.FindByIdentity(identity)
			if err != nil {
				return nil, err
			}
			if item != nil {
				return item, nil
			}
		}
	}
	return e.docker.FindManagedContainerByIdentity(ctx, identity)
}

func (e *Executor) waitForHealth(ctx context.Context, task *grpcapi.TaskCommand, runtime *agentdomain.ContainerRuntime, report func(*grpcapi.TaskUpdate) error) error {
	deadline := time.Now().Add(time.Duration(task.GetHttpTimeoutSecond()) * time.Second)
	probeInterval := time.Duration(defaultProbeInterval(task.GetHttpProbeIntervalSecond())) * time.Second
	successThreshold := int(defaultSuccessThreshold(task.GetHttpSuccessThreshold()))
	startupGrace := time.Duration(defaultStartupGrace(task.GetStartupGraceSecond())) * time.Second

	if startupGrace > 0 {
		if err := report(&grpcapi.TaskUpdate{
			TaskId:      task.GetTaskId(),
			Status:      grpcapi.TaskStatus_TASK_STATUS_RUNNING,
			Step:        stepStartupGrace,
			ContainerId: runtime.ContainerID,
		}); err != nil {
			return err
		}
		graceDeadline := time.Now().Add(startupGrace)
		for time.Now().Before(graceDeadline) {
			status, err := e.docker.InspectContainer(ctx, runtime.ContainerID)
			if err != nil {
				return err
			}
			if status != nil && status.Terminal() {
				return fmt.Errorf("container entered terminal state during startup grace: %s", summarizeContainerStatus(status))
			}
			if err := sleepWithContext(ctx, minDuration(probeInterval, time.Until(graceDeadline))); err != nil {
				return err
			}
		}
	}

	httpProbeRequired := strings.TrimSpace(task.GetHttpHealthPath()) != ""
	var lastErr error
	consecutiveSuccess := 0
	retryReported := false
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		status, err := e.docker.InspectContainer(ctx, runtime.ContainerID)
		if err != nil {
			lastErr = err
		} else if status != nil && status.Terminal() {
			return fmt.Errorf("container entered terminal state: %s", summarizeContainerStatus(status))
		} else {
			if httpProbeRequired && strings.TrimSpace(runtime.ListenAddress) == "" {
				listenAddress, resolveErr := e.docker.ResolveListenAddress(ctx, runtime.ContainerID, int(task.GetContainerPort()))
				if resolveErr != nil {
					lastErr = resolveErr
				} else {
					runtime.ListenAddress = listenAddress
				}
			}
			if !httpProbeRequired || strings.TrimSpace(runtime.ListenAddress) != "" {
				if healthErr := e.verifyHealth(ctx, task, status, runtime.ListenAddress); healthErr == nil {
					consecutiveSuccess++
					if consecutiveSuccess >= successThreshold {
						return nil
					}
					lastErr = nil
				} else {
					lastErr = healthErr
					consecutiveSuccess = 0
					if !retryReported {
						retryReported = true
						if reportErr := report(&grpcapi.TaskUpdate{
							TaskId:       task.GetTaskId(),
							Status:       grpcapi.TaskStatus_TASK_STATUS_RUNNING,
							Step:         stepHealthProbeRetry,
							ContainerId:  runtime.ContainerID,
							ErrorMessage: healthErr.Error(),
						}); reportErr != nil {
							return reportErr
						}
					}
				}
			}
		}
		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = fmt.Errorf("health check timeout for task %s", task.GetTaskId())
			}
			return fmt.Errorf("health check timeout for task %s: %w", task.GetTaskId(), lastErr)
		}
		if err := sleepWithContext(ctx, probeInterval); err != nil {
			return err
		}
	}
}

func (e *Executor) verifyHealth(ctx context.Context, task *grpcapi.TaskCommand, status *agentdomain.ContainerStatus, listenAddress string) error {
	if task.GetDockerHealthCheck() && status != nil {
		health := strings.TrimSpace(status.Health)
		if health != "" && !strings.EqualFold(health, "healthy") {
			return fmt.Errorf("docker health not ready: %s", health)
		}
	}
	path := strings.TrimSpace(task.GetHttpHealthPath())
	if path == "" {
		return nil
	}
	expectedCode := defaultCode(task.GetHttpExpectedCode())
	timeoutSeconds := defaultProbeTimeout(task.GetHttpProbeTimeoutSecond())
	if err := e.httpProbe(
		listenAddress,
		path,
		task.GetHttpHealthHeaders(),
		expectedCode,
		timeoutSeconds,
	); err != nil {
		return fmt.Errorf("http health probe failed: target=http://%s%s expected=%d timeout=%ds: %w", listenAddress, path, expectedCode, timeoutSeconds, err)
	}
	return nil
}

func (e *Executor) wrapTaskFailure(ctx context.Context, runtime *agentdomain.ContainerRuntime, err error) error {
	execErr, ok := err.(*TaskExecutionError)
	if ok {
		if execErr.Diagnostic != nil || strings.TrimSpace(runtimeID(runtime)) == "" {
			return execErr
		}
	}
	diagnostic, cleanupErr := e.collectFailureDiagnostic(ctx, runtime)
	if cleanupErr != nil {
		err = fmt.Errorf("%w; cleanup failed: %v", err, cleanupErr)
	}
	step := stepHealthCheckFailed
	if ok && strings.TrimSpace(execErr.Step) != "" {
		step = execErr.Step
	}
	return &TaskExecutionError{
		Step:       step,
		Err:        err,
		Diagnostic: diagnostic,
	}
}

func (e *Executor) collectFailureDiagnostic(ctx context.Context, runtime *agentdomain.ContainerRuntime) (*TaskFailureDiagnostic, error) {
	containerID := runtimeID(runtime)
	if containerID == "" {
		return nil, nil
	}
	diagnostic := &TaskFailureDiagnostic{
		ContainerID: containerID,
	}
	status, statusErr := e.docker.InspectContainer(ctx, containerID)
	if statusErr == nil {
		diagnostic.DockerHealth = summarizeContainerStatus(status)
	} else {
		diagnostic.DockerHealth = statusErr.Error()
	}
	logs, logsErr := e.docker.ReadContainerLogs(ctx, containerID, defaultLogTailLines, defaultLogMaxBytes)
	if logsErr != nil {
		diagnostic.FailureLogs = "log_capture_failed: " + logsErr.Error()
	} else {
		diagnostic.FailureLogs = logs
	}
	cleanupErr := e.docker.RemoveContainer(ctx, containerID)
	if cleanupErr == nil && strings.TrimSpace(runtime.Image) != "" {
		if err := e.docker.RemoveImage(ctx, runtime.Image); err != nil {
			e.logger.Errorf(err, "failed to remove docker image after container cleanup: image=%s containerId=%s", runtime.Image, containerID)
		}
	}
	diagnostic.CleanupCompleted = cleanupErr == nil
	return diagnostic, cleanupErr
}

func (e *Executor) cleanupManagedContainers(ctx context.Context, task *grpcapi.TaskCommand) (int, error) {
	items, err := e.docker.ListManagedContainers(ctx, task.GetAgentId(), task.GetServiceKey())
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	removed := 0
	var errs []error
	for _, item := range items {
		if item == nil {
			continue
		}
		if shouldPreserveManagedContainerForSwitch(item, task) {
			continue
		}
		if err := e.docker.RemoveContainer(ctx, item.ContainerID); err != nil {
			errs = append(errs, err)
			continue
		}
		e.removeContainerImage(ctx, item.ContainerID, item.Image)
		removed++
	}
	if len(errs) > 0 {
		return removed, errors.Join(errs...)
	}
	return removed, nil
}

func (e *Executor) removeContainerImage(ctx context.Context, containerID, image string) {
	if strings.TrimSpace(image) == "" {
		return
	}
	if err := e.docker.RemoveImage(ctx, image); err != nil {
		e.logger.Errorf(err, "failed to remove docker image after container cleanup: image=%s containerId=%s", image, containerID)
	}
}

func (e *Executor) cleanupStaleContainersOnSlot(ctx context.Context, agentID, serviceKey string, slot grpcapi.Slot) (int, error) {
	var items []*agentdomain.ManagedContainer
	if e.index != nil {
		items = e.index.FindByServiceKeyAndSlot(serviceKey, slot)
	}
	if len(items) == 0 {
		all, err := e.docker.ListManagedContainers(ctx, agentID, serviceKey)
		if err != nil {
			return 0, err
		}
		for _, item := range all {
			if item != nil && item.Slot == slot {
				items = append(items, item)
			}
		}
	}
	removed := 0
	var errs []error
	for _, item := range items {
		if err := e.docker.RemoveContainer(ctx, item.ContainerID); err != nil {
			errs = append(errs, err)
			continue
		}
		e.removeContainerImage(ctx, item.ContainerID, item.Image)
		removed++
	}
	if len(errs) > 0 {
		return removed, errors.Join(errs...)
	}
	return removed, nil
}

func shouldPreserveManagedContainerForSwitch(item *agentdomain.ManagedContainer, task *grpcapi.TaskCommand) bool {
	if item == nil || task == nil {
		return false
	}
	releaseID := strings.TrimSpace(task.GetReleaseId())
	if releaseID != "" && strings.TrimSpace(item.ReleaseID) == releaseID {
		return true
	}
	if item.Slot != grpcapi.Slot_SLOT_UNSPECIFIED && item.Slot == task.GetCurrentLiveSlot() {
		return true
	}
	// Fallback for malformed tasks without release id.
	return releaseID == "" && item.Slot != grpcapi.Slot_SLOT_UNSPECIFIED && item.Slot == task.GetTargetSlot()
}

func (e *Executor) ReconcileManagedContainersOnStartup(ctx context.Context, agentID string) (StartupManagedContainerScanStats, error) {
	stats := StartupManagedContainerScanStats{}
	items, err := e.docker.ListManagedContainers(ctx, agentID, "")
	if err != nil {
		return stats, err
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		stats.Scanned++
		if !shouldRemoveOnStartupScan(item) {
			stats.Preserved++
			continue
		}
		if err := e.docker.RemoveContainer(ctx, item.ContainerID); err != nil {
			stats.Failed++
			e.logger.Errorf(err, "startup managed container cleanup failed: agentId=%s containerId=%s serviceKey=%s releaseId=%s slot=%s state=%s", agentID, item.ContainerID, item.ServiceKey, item.ReleaseID, item.Slot.String(), item.State)
			continue
		}
		e.removeContainerImage(ctx, item.ContainerID, item.Image)
		stats.Removed++
	}
	e.logger.Infof("startup managed container scan completed: agentId=%s scanned=%d removed=%d preserved=%d failed=%d", agentID, stats.Scanned, stats.Removed, stats.Preserved, stats.Failed)
	return stats, nil
}

func normalizeHealthConfig(task *grpcapi.TaskCommand) {
	if task.GetHttpExpectedCode() == 0 {
		task.HttpExpectedCode = http.StatusOK
	}
	if task.GetHttpTimeoutSecond() <= 0 {
		task.HttpTimeoutSecond = int32(modelDefaultHTTPTimeout())
	}
	if task.GetStartupGraceSecond() <= 0 {
		task.StartupGraceSecond = int32(modelDefaultStartupGrace())
	}
	if task.GetHttpProbeTimeoutSecond() <= 0 {
		task.HttpProbeTimeoutSecond = int32(modelDefaultProbeTimeout())
	}
	if task.GetHttpProbeIntervalSecond() <= 0 {
		task.HttpProbeIntervalSecond = int32(modelDefaultProbeInterval())
	}
	if task.GetHttpSuccessThreshold() <= 0 {
		task.HttpSuccessThreshold = int32(modelDefaultSuccessThreshold())
	}
}

func shouldRemoveOnStartupScan(item *agentdomain.ManagedContainer) bool {
	if item == nil {
		return false
	}
	if isTerminalContainerState(item.State) {
		return true
	}
	return !managedContainerLabelsValid(item)
}

func isTerminalContainerState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "exited", "dead", "removing":
		return true
	default:
		return false
	}
}

func managedContainerLabelsValid(item *agentdomain.ManagedContainer) bool {
	if item == nil {
		return false
	}
	if strings.TrimSpace(item.ServiceKey) == "" || strings.TrimSpace(item.ReleaseID) == "" {
		return false
	}
	switch item.Slot {
	case grpcapi.Slot_SLOT_BLUE, grpcapi.Slot_SLOT_GREEN:
		return true
	default:
		return false
	}
}

func defaultCode(value int32) int {
	if value == 0 {
		return http.StatusOK
	}
	return int(value)
}

func defaultStartupGrace(value int32) int {
	if value > 0 {
		return int(value)
	}
	return modelDefaultStartupGrace()
}

func defaultProbeTimeout(value int32) int {
	if value > 0 {
		return int(value)
	}
	return modelDefaultProbeTimeout()
}

func defaultProbeInterval(value int32) int {
	if value > 0 {
		return int(value)
	}
	return modelDefaultProbeInterval()
}

func defaultSuccessThreshold(value int32) int {
	if value > 0 {
		return int(value)
	}
	return modelDefaultSuccessThreshold()
}

func modelDefaultStartupGrace() int {
	return model.DefaultStartupGraceSecond
}

func modelDefaultProbeTimeout() int {
	return model.DefaultHTTPProbeTimeoutSecond
}

func modelDefaultProbeInterval() int {
	return model.DefaultHTTPProbeIntervalSecond
}

func modelDefaultSuccessThreshold() int {
	return model.DefaultHTTPSuccessThreshold
}

func modelDefaultHTTPTimeout() int {
	return model.DefaultHTTPTimeoutSecond
}

func probeHTTP(listenAddress string, path string, headers map[string]string, expectedCode int, timeoutSeconds int) error {
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, "http://"+listenAddress+path, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedCode {
		return fmt.Errorf("unexpected health status: %d", resp.StatusCode)
	}
	return nil
}

func summarizeContainerStatus(status *agentdomain.ContainerStatus) string {
	if status == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if strings.TrimSpace(status.State) != "" {
		parts = append(parts, "state="+status.State)
	}
	if strings.TrimSpace(status.Health) != "" {
		parts = append(parts, "health="+status.Health)
	}
	parts = append(parts, fmt.Sprintf("running=%t", status.Running))
	return strings.Join(parts, " ")
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func minDuration(left time.Duration, right time.Duration) time.Duration {
	if right <= 0 {
		return left
	}
	if left < right {
		return left
	}
	return right
}

func runtimeID(runtime *agentdomain.ContainerRuntime) string {
	if runtime == nil {
		return ""
	}
	return strings.TrimSpace(runtime.ContainerID)
}

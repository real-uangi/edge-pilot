package application

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/model"
)

func (s *Service) GetPipeline(release *dto.ReleaseOutput, tasks []dto.TaskSnapshot, audits []dto.AuditLog) (*dto.ReleasePipeline, error) {
	stages := []dto.PipelineStage{
		{ID: "dispatch", Label: "派发", Kind: "automatic"},
		{ID: "image", Label: "镜像准备", Kind: "automatic"},
		{ID: "container", Label: "启动容器", Kind: "automatic"},
		{ID: "health", Label: "健康检查", Kind: "automatic"},
		{ID: "confirmation", Label: "等待确认", Kind: "manual"},
		{ID: "traffic", Label: "切换流量", Kind: "event"},
	}
	for _, audit := range audits {
		if audit.EventType == "rollback_completed" || audit.EventType == "rolled_back" {
			stages = append(stages, dto.PipelineStage{ID: "rollback", Label: "回滚", Kind: "event"})
			break
		}
	}
	if len(stages) == 6 && release.Status == model.ReleaseStatusRolledBack {
		stages = append(stages, dto.PipelineStage{ID: "rollback", Label: "回滚", Kind: "event"})
	}

	history, err := s.repo.ListRecentSuccessfulDeployTasks(release.ServiceID, release.ID, 10)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(history))
	for i := range history {
		ids[i] = history[i].ID
	}
	historicalAttempts, err := s.repo.ListTaskAttemptsByTasks(ids)
	if err != nil {
		return nil, err
	}
	byTask := make(map[uuid.UUID][]model.TaskAttempt, len(history))
	for _, attempt := range historicalAttempts {
		byTask[attempt.TaskID] = append(byTask[attempt.TaskID], attempt)
	}
	var totals [4]int64
	for _, task := range history {
		attempts := byTask[task.ID]
		snapshot := dto.TaskSnapshot{
			ID: task.ID, Type: task.Type, Status: task.Status, CreatedAt: task.CreatedAt,
			CompletedAt: task.CompletedAt, LastStep: task.LastStep,
			Attempts: make([]dto.TaskAttempt, len(attempts)),
		}
		for i, attempt := range attempts {
			startedAt := attempt.StartedAt
			if startedAt == nil {
				startedAt = attempt.CompletedAt
			}
			if startedAt == nil {
				startedAt = &attempts[i].CreatedAt
			}
			snapshot.Attempts[i] = dto.TaskAttempt{
				ID: attempt.ID, Status: attempt.Status, Message: attempt.Message,
				StartedAt: startedAt, CompletedAt: attempt.CompletedAt,
			}
		}
		run := buildPipelineRun(snapshot, stages, false)
		if run.ReplayCount != 0 {
			continue
		}
		for i := range totals {
			cell := run.Cells[i]
			if cell.Status == "completed" && cell.DurationMS != nil && *cell.DurationMS > 0 {
				totals[i] += *cell.DurationMS
				stages[i].SampleCount++
			}
		}
	}
	for i := range totals {
		if stages[i].SampleCount > 0 {
			mean := totals[i] / int64(stages[i].SampleCount)
			stages[i].EstimatedDurationMS = &mean
		}
	}

	deploys := make([]dto.TaskSnapshot, 0, len(tasks))
	for _, task := range tasks {
		if task.Type == model.TaskTypeDeployGreen {
			deploys = append(deploys, task)
		}
	}
	sort.SliceStable(deploys, func(i, j int) bool { return deploys[i].CreatedAt.Before(deploys[j].CreatedAt) })
	orderedAudits := append([]dto.AuditLog(nil), audits...)
	sort.SliceStable(orderedAudits, func(i, j int) bool { return orderedAudits[i].CreatedAt.Before(orderedAudits[j].CreatedAt) })
	pipeline := &dto.ReleasePipeline{Stages: stages, Runs: make([]dto.PipelineRun, len(deploys))}
	for i, task := range deploys {
		isCurrent := release.CurrentTaskID != nil && *release.CurrentTaskID == task.ID && i == len(deploys)-1
		run := buildPipelineRun(task, stages, isCurrent && !release.Status.IsTerminal())
		run.Sequence = i + 1
		var until *time.Time
		if i+1 < len(deploys) {
			until = &deploys[i+1].CreatedAt
		}
		applyPipelineAudits(&run, orderedAudits, until, release, isCurrent)
		pipeline.Runs[len(deploys)-1-i] = run
	}
	return pipeline, nil
}

func buildPipelineRun(task dto.TaskSnapshot, stages []dto.PipelineStage, isCurrent bool) dto.PipelineRun {
	run := dto.PipelineRun{
		TaskID: task.ID, Status: task.Status, StartedAt: task.CreatedAt, CompletedAt: task.CompletedAt,
		Cells: make([]dto.PipelineCell, len(stages)),
	}
	for i, stage := range stages {
		run.Cells[i] = dto.PipelineCell{StageID: stage.ID, Status: "pending", Events: []dto.PipelineEvent{}}
	}
	active := -1
	var since time.Time
	closeActive := func(at time.Time, status string, completeBoundary bool) {
		if active < 0 {
			return
		}
		cell := &run.Cells[active]
		cell.RunningSince = nil
		if !completeBoundary || at.Before(since) {
			cell.Status = "unrecorded"
			cell.DurationMS = nil
			return
		}
		duration := at.Sub(since).Milliseconds()
		if cell.DurationMS != nil {
			duration += *cell.DurationMS
		}
		cell.DurationMS = &duration
		cell.Status = status
	}
	finish := func(at *time.Time, status model.TaskStatus) int {
		failedStage := pipelineFailureStage(task.LastStep)
		if status == model.TaskStatusSucceeded {
			if active == 3 && at != nil {
				closeActive(*at, "completed", true)
			} else if active >= 0 {
				run.Cells[active].Status = "unrecorded"
				run.Cells[active].DurationMS = nil
			}
			for i := 0; i < 4; i++ {
				if run.Cells[i].Status == "pending" {
					run.Cells[i].Status = "unrecorded"
				}
			}
			active = -1
			return 3
		}
		if failedStage < 0 {
			failedStage = active
		}
		if active >= 0 {
			if at != nil {
				closeActive(*at, "unrecorded", active == failedStage)
			} else {
				run.Cells[active].Status = "unrecorded"
				run.Cells[active].DurationMS = nil
			}
		}
		if failedStage >= 0 {
			run.Cells[failedStage].Status = "failed"
			if status == model.TaskStatusTimedOut {
				run.Cells[failedStage].Status = "timed_out"
			}
		}
		active = -1
		return failedStage
	}
	finished := false
	for _, attempt := range task.Attempts {
		at := attempt.StartedAt
		if at == nil {
			at = attempt.CompletedAt
		}
		if at == nil {
			continue
		}
		event := dto.PipelineEvent{ID: attempt.ID, Kind: "task", Code: attempt.Message, Message: attempt.Message, At: *at}
		if attempt.Message == "replayed_after_reconnect" {
			run.ReplayCount++
		}
		if attempt.Message == "healthy" || attempt.Status.IsTerminal() {
			status := attempt.Status
			if attempt.Message == "healthy" {
				status = model.TaskStatusSucceeded
			}
			index := finish(at, status)
			if index >= 0 {
				run.Cells[index].Events = append(run.Cells[index].Events, event)
			}
			finished = true
			continue
		}
		index := pipelineStageForStep(attempt.Message)
		if index < 0 {
			index = active
		} else if index != active {
			if active >= 0 {
				closeActive(*at, "completed", index == active+1 || index < active)
				if index < active {
					run.Cells[active].Status = "unrecorded"
				}
			}
			for i := 0; i < index; i++ {
				if run.Cells[i].Status == "pending" {
					run.Cells[i].Status = "unrecorded"
				}
			}
			active = index
			since = *at
			if run.Cells[index].DurationMS == nil {
				zero := int64(0)
				run.Cells[index].DurationMS = &zero
			}
		}
		if index >= 0 {
			run.Cells[index].Events = append(run.Cells[index].Events, event)
		}
	}
	if task.Status.IsTerminal() {
		if !finished || active >= 0 {
			finish(task.CompletedAt, task.Status)
		}
	} else if active >= 0 {
		cell := &run.Cells[active]
		if isCurrent {
			cell.Status = "running"
			if task.Status == model.TaskStatusPending {
				cell.Status = "waiting"
			}
			cell.RunningSince = &since
		} else {
			cell.Status = "unrecorded"
			cell.DurationMS = nil
		}
	}
	return run
}

func pipelineStageForStep(step string) int {
	switch step {
	case "dispatched", "replayed_after_reconnect":
		return 0
	case "accepted":
		return 1
	case "image_pulled":
		return 2
	case "container_started", "container_reused_waiting", "startup_grace_started", "health_probe_retry":
		return 3
	default:
		return -1
	}
}

func pipelineFailureStage(step string) int {
	switch step {
	case "dispatch_failed":
		return 0
	case "proxy_stack_not_ready", "image_pull_failed":
		return 1
	case "managed_container_conflict":
		return 2
	case "health_check_failed":
		return 3
	default:
		return -1
	}
}

func applyPipelineAudits(run *dto.PipelineRun, audits []dto.AuditLog, until *time.Time, release *dto.ReleaseOutput, isCurrent bool) {
	confirmation := &run.Cells[4]
	traffic := &run.Cells[5]
	var readyAt *time.Time
	var endedAt *time.Time
	for _, audit := range audits {
		if audit.CreatedAt.Before(run.StartedAt) || (until != nil && !audit.CreatedAt.Before(*until)) {
			continue
		}
		event := dto.PipelineEvent{ID: audit.ID, Kind: "audit", Code: audit.EventType, Message: audit.Message, At: audit.CreatedAt}
		switch audit.EventType {
		case "ready_to_switch":
			if readyAt == nil {
				at := audit.CreatedAt
				readyAt = &at
			}
			confirmation.Events = append(confirmation.Events, event)
		case "traffic_percent_updated":
			traffic.Events = append(traffic.Events, event)
			traffic.Status = "waiting"
		case "switch_confirmed", "traffic_switched":
			at := audit.CreatedAt
			endedAt = &at
			traffic.Status = "completed"
			traffic.Events = append(traffic.Events, event)
		case "rollback_completed", "rolled_back":
			at := audit.CreatedAt
			endedAt = &at
			traffic.Status = "skipped"
			rollback := &run.Cells[6]
			rollback.Status = "completed"
			rollback.Events = append(rollback.Events, event)
		case "release_superseded":
			at := audit.CreatedAt
			endedAt = &at
			traffic.Status = "skipped"
			confirmation.Events = append(confirmation.Events, event)
		}
	}
	if readyAt == nil {
		if endedAt != nil || (isCurrent && run.Status == model.TaskStatusSucceeded) {
			confirmation.Status = "unrecorded"
		}
		return
	}
	if endedAt == nil {
		if until != nil {
			endedAt = until
			traffic.Status = "unrecorded"
		} else if isCurrent && release.Status.IsTerminal() {
			endedAt = release.CompletedAt
			traffic.Status = "skipped"
			if release.SwitchConfirmed != nil && *release.SwitchConfirmed {
				traffic.Status = "completed"
			}
			if release.Status == model.ReleaseStatusRolledBack && len(run.Cells) > 6 {
				run.Cells[6].Status = "completed"
			}
		}
	}
	if endedAt != nil && !endedAt.Before(*readyAt) {
		duration := endedAt.Sub(*readyAt).Milliseconds()
		confirmation.DurationMS = &duration
		confirmation.Status = "completed"
	} else if isCurrent && !release.Status.IsTerminal() && run.Status == model.TaskStatusSucceeded {
		zero := int64(0)
		confirmation.DurationMS = &zero
		confirmation.RunningSince = readyAt
		confirmation.Status = "waiting"
	} else {
		confirmation.Status = "unrecorded"
	}
}

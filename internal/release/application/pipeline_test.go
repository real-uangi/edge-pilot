package application

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/model"
)

func pipelineTestTask(start time.Time, spacing time.Duration, status model.TaskStatus, steps ...string) dto.TaskSnapshot {
	task := dto.TaskSnapshot{ID: uuid.New(), Type: model.TaskTypeDeployGreen, Status: status, CreatedAt: start}
	for i, step := range steps {
		at := start.Add(time.Duration(i) * spacing)
		attemptStatus := model.TaskStatusRunning
		if step == "dispatched" || step == "replayed_after_reconnect" {
			attemptStatus = model.TaskStatusDispatched
		}
		attempt := dto.TaskAttempt{ID: uuid.New(), Message: step, Status: attemptStatus, StartedAt: &at}
		if i == len(steps)-1 && status.IsTerminal() {
			attempt.Status = status
			attempt.CompletedAt = &at
			task.CompletedAt = &at
		}
		task.Attempts = append(task.Attempts, attempt)
		task.LastStep = step
	}
	return task
}

func pipelineTestRelease(task dto.TaskSnapshot) *dto.ReleaseOutput {
	return &dto.ReleaseOutput{ID: uuid.New(), ServiceID: uuid.New(), CurrentTaskID: &task.ID, Status: model.ReleaseStatusDeploying}
}

func TestPipelineKeepsRetriesSeparateAndOnlyCurrentStageActive(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	failed := pipelineTestTask(base, time.Second, model.TaskStatusFailed, "dispatched", "accepted", "image_pulled", "container_started", "health_check_failed")
	current := pipelineTestTask(base.Add(time.Minute), time.Second, model.TaskStatusRunning, "dispatched", "accepted", "image_pulled")
	release := pipelineTestRelease(current)
	tasks := []dto.TaskSnapshot{current, failed}
	before, _ := json.Marshal(tasks)
	pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(release, tasks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pipeline.Runs) != 2 || pipeline.Runs[0].TaskID != current.ID || pipeline.Runs[0].Sequence != 2 || pipeline.Runs[1].Sequence != 1 {
		t.Fatalf("incorrect execution rows: %+v", pipeline.Runs)
	}
	for _, run := range pipeline.Runs {
		for i, cell := range run.Cells {
			wantActive := run.TaskID == current.ID && i == 2
			if (cell.RunningSince != nil) != wantActive {
				t.Errorf("task %s stage %s activity = %v, want %v", run.TaskID, cell.StageID, cell.RunningSince, wantActive)
			}
		}
	}
	if cell := pipeline.Runs[1].Cells[3]; cell.Status != "failed" || cell.DurationMS == nil || *cell.DurationMS != 1000 {
		t.Fatalf("failed health stage = %+v", cell)
	}
	after, _ := json.Marshal(tasks)
	if string(before) != string(after) {
		t.Fatal("pipeline projection changed task records")
	}
}

func TestPipelineReplaysAccumulateSegmentsWithoutAddingRows(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	task := pipelineTestTask(base, time.Second, model.TaskStatusSucceeded,
		"dispatched", "accepted", "image_pulled", "container_started", "health_probe_retry",
		"replayed_after_reconnect", "accepted", "image_pulled", "container_started", "healthy")
	pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(pipelineTestRelease(task), []dto.TaskSnapshot{task}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pipeline.Runs) != 1 || pipeline.Runs[0].ReplayCount != 1 {
		t.Fatalf("replay created an execution row: %+v", pipeline.Runs)
	}
	for i, expected := range []int64{2000, 2000, 2000, 3000} {
		cell := pipeline.Runs[0].Cells[i]
		if cell.Status != "completed" || cell.DurationMS == nil || *cell.DurationMS != expected || cell.RunningSince != nil {
			t.Errorf("stage %s = %+v, want completed %dms", cell.StageID, cell, expected)
		}
	}
}

func TestPipelinePendingAndUnknownRecoveryDoNotAnimate(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		task dto.TaskSnapshot
	}{
		{"deferred dispatch", pipelineTestTask(base, time.Second, model.TaskStatusPending, "dispatched")},
		{"unknown recovery", pipelineTestTask(base, time.Second, model.TaskStatusRunning, "recovered_running")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(pipelineTestRelease(tc.task), []dto.TaskSnapshot{tc.task}, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, cell := range pipeline.Runs[0].Cells {
				if cell.Status == "running" {
					t.Fatalf("unexpected running stage: %+v", cell)
				}
			}
		})
	}
}

func TestPipelineDoesNotInventMissingStageDurations(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	task := pipelineTestTask(base, time.Second, model.TaskStatusSucceeded, "dispatched", "accepted", "image_pulled", "healthy")
	pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(pipelineTestRelease(task), []dto.TaskSnapshot{task}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{2, 3} {
		cell := pipeline.Runs[0].Cells[index]
		if cell.Status != "unrecorded" || cell.DurationMS != nil || cell.RunningSince != nil {
			t.Fatalf("missing stage was invented: %+v", cell)
		}
	}
	if cell := pipeline.Runs[0].Cells[4]; cell.Status != "unrecorded" || cell.RunningSince != nil {
		t.Fatalf("missing confirmation boundary was invented: %+v", cell)
	}
}

func TestPipelineTerminalTasksStopWithoutFinalAttempt(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	for _, status := range []model.TaskStatus{model.TaskStatusSucceeded, model.TaskStatusFailed, model.TaskStatusTimedOut} {
		t.Run(string(rune('0'+status)), func(t *testing.T) {
			task := pipelineTestTask(base, time.Second, model.TaskStatusRunning, "dispatched", "accepted", "image_pulled", "container_started")
			task.Status = status
			end := base.Add(5 * time.Second)
			task.CompletedAt = &end
			task.LastStep = "healthy"
			if status == model.TaskStatusFailed {
				task.LastStep = "health_check_failed"
			}
			if status == model.TaskStatusTimedOut {
				task.LastStep = "task_timed_out"
			}
			pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(pipelineTestRelease(task), []dto.TaskSnapshot{task}, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, cell := range pipeline.Runs[0].Cells {
				if cell.RunningSince != nil {
					t.Fatalf("terminal task still counts time: %+v", cell)
				}
			}
			cell := pipeline.Runs[0].Cells[3]
			if cell.DurationMS == nil || *cell.DurationMS != 2000 {
				t.Fatalf("terminal health interval = %+v, want 2000ms", cell)
			}
		})
	}
}

func TestPipelineExplicitFailureAndUnknownFailure(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		step  string
		index int
	}{
		{"dispatch_failed", 0}, {"proxy_stack_not_ready", 1}, {"image_pull_failed", 1},
		{"managed_container_conflict", 2}, {"health_check_failed", 3}, {"execution_failed", -1},
	} {
		t.Run(tc.step, func(t *testing.T) {
			task := pipelineTestTask(base, time.Second, model.TaskStatusFailed, "error details")
			task.LastStep = tc.step
			pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(pipelineTestRelease(task), []dto.TaskSnapshot{task}, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i, cell := range pipeline.Runs[0].Cells {
				if (cell.Status == "failed") != (i == tc.index) || cell.RunningSince != nil {
					t.Errorf("stage %s = %+v, failure index %d", cell.StageID, cell, tc.index)
				}
			}
		})
	}
}

func TestPipelineTerminalTaskCannotBeReactivatedByLaterStep(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	task := pipelineTestTask(base, time.Second, model.TaskStatusSucceeded, "dispatched", "accepted", "image_pulled", "container_started", "healthy")
	late := base.Add(5 * time.Second)
	task.Attempts = append(task.Attempts, dto.TaskAttempt{ID: uuid.New(), Status: model.TaskStatusRunning, Message: "accepted", StartedAt: &late})
	pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(pipelineTestRelease(task), []dto.TaskSnapshot{task}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range pipeline.Runs[0].Cells {
		if cell.RunningSince != nil || cell.Status == "running" {
			t.Fatalf("terminal task was reactivated: %+v", cell)
		}
	}
}

func TestPipelineConfirmationUsesAuditsAndKeepsInstantResultsUntimed(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	task := pipelineTestTask(base, time.Second, model.TaskStatusSucceeded, "dispatched", "accepted", "image_pulled", "container_started", "healthy")
	ready := base.Add(5 * time.Second)
	end := base.Add(12 * time.Second)
	for _, tc := range []struct {
		name           string
		event          string
		releaseStatus  model.ReleaseStatus
		wantTraffic    string
		wantStageCount int
	}{
		{"waiting", "", model.ReleaseStatusReadyToSwitch, "pending", 6},
		{"confirmed", "switch_confirmed", model.ReleaseStatusCompleted, "completed", 6},
		{"rollback", "rollback_completed", model.ReleaseStatusRolledBack, "skipped", 7},
		{"superseded", "release_superseded", model.ReleaseStatusCompleted, "skipped", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release := pipelineTestRelease(task)
			release.Status = tc.releaseStatus
			audits := []dto.AuditLog{{ID: uuid.New(), EventType: "ready_to_switch", CreatedAt: ready}}
			if tc.event != "" {
				audits = append(audits, dto.AuditLog{ID: uuid.New(), EventType: tc.event, CreatedAt: end})
				release.CompletedAt = &end
			}
			pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(release, []dto.TaskSnapshot{task}, audits)
			if err != nil {
				t.Fatal(err)
			}
			if len(pipeline.Stages) != tc.wantStageCount {
				t.Fatalf("stage count = %d, want %d", len(pipeline.Stages), tc.wantStageCount)
			}
			confirmation := pipeline.Runs[0].Cells[4]
			traffic := pipeline.Runs[0].Cells[5]
			if traffic.Status != tc.wantTraffic || traffic.DurationMS != nil || traffic.RunningSince != nil {
				t.Fatalf("traffic result was timed or has wrong status: %+v", traffic)
			}
			if tc.event == "" {
				if confirmation.Status != "waiting" || confirmation.RunningSince == nil || !confirmation.RunningSince.Equal(ready) {
					t.Fatalf("confirmation is not waiting: %+v", confirmation)
				}
			} else if confirmation.Status != "completed" || confirmation.DurationMS == nil || *confirmation.DurationMS != 7000 || confirmation.RunningSince != nil {
				t.Fatalf("confirmation did not stop: %+v", confirmation)
			}
			if tc.event == "rollback_completed" {
				rollback := pipeline.Runs[0].Cells[6]
				if rollback.Status != "completed" || rollback.DurationMS != nil {
					t.Fatalf("rollback result = %+v", rollback)
				}
			}
		})
	}
}

func TestPipelineConfirmationDoesNotRunAfterReleaseEndsWithoutFinalAudit(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	task := pipelineTestTask(base, time.Second, model.TaskStatusSucceeded, "dispatched", "accepted", "image_pulled", "container_started", "healthy")
	release := pipelineTestRelease(task)
	release.Status = model.ReleaseStatusCompleted
	end := base.Add(time.Minute)
	release.CompletedAt = &end
	pipeline, err := (&Service{repo: newFakeReleaseRepo()}).GetPipeline(release, []dto.TaskSnapshot{task}, []dto.AuditLog{{EventType: "ready_to_switch", CreatedAt: base.Add(5 * time.Second)}})
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range pipeline.Runs[0].Cells {
		if cell.RunningSince != nil {
			t.Fatalf("completed release still counts time: %+v", cell)
		}
	}
}

func TestPipelineEstimatesUseOnlyValidSameServiceSuccesses(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	repo := newFakeReleaseRepo()
	current := pipelineTestTask(base.Add(time.Hour), time.Second, model.TaskStatusRunning, "dispatched")
	release := pipelineTestRelease(current)
	steps := []string{"dispatched", "accepted", "image_pulled", "container_started", "healthy"}
	addHistory := func(task dto.TaskSnapshot, serviceID, releaseID uuid.UUID) {
		repo.tasks[task.ID] = &model.Task{ID: task.ID, ServiceID: serviceID, ReleaseID: releaseID,
			Model: commondb.Model{CreatedAt: task.CreatedAt}, Type: task.Type, Status: task.Status, CompletedAt: task.CompletedAt, LastStep: task.LastStep}
		for _, attempt := range task.Attempts {
			repo.taskAttempts = append(repo.taskAttempts, &model.TaskAttempt{
				ID: attempt.ID, TaskID: task.ID, Model: commondb.Model{CreatedAt: *attempt.StartedAt},
				Status: attempt.Status, Message: attempt.Message, StartedAt: attempt.StartedAt, CompletedAt: attempt.CompletedAt,
			})
		}
	}
	addHistory(pipelineTestTask(base, time.Second, model.TaskStatusSucceeded, steps...), release.ServiceID, uuid.New())
	addHistory(pipelineTestTask(base.Add(time.Minute), 3*time.Second, model.TaskStatusSucceeded, steps...), release.ServiceID, uuid.New())
	addHistory(pipelineTestTask(base.Add(2*time.Minute), 10*time.Second, model.TaskStatusSucceeded,
		"dispatched", "replayed_after_reconnect", "accepted", "image_pulled", "container_started", "healthy"), release.ServiceID, uuid.New())
	addHistory(pipelineTestTask(base.Add(3*time.Minute), 20*time.Second, model.TaskStatusSucceeded, steps...), uuid.New(), uuid.New())
	addHistory(pipelineTestTask(base.Add(4*time.Minute), 30*time.Second, model.TaskStatusSucceeded, steps...), release.ServiceID, release.ID)
	addHistory(pipelineTestTask(base.Add(5*time.Minute), 40*time.Second, model.TaskStatusFailed, steps...), release.ServiceID, uuid.New())
	// This successful deployment lacks container/health boundaries and contributes only dispatch and image.
	addHistory(pipelineTestTask(base.Add(6*time.Minute), 2*time.Second, model.TaskStatusSucceeded,
		"dispatched", "accepted", "image_pulled", "healthy"), release.ServiceID, uuid.New())
	pipeline, err := (&Service{repo: repo}).GetPipeline(release, []dto.TaskSnapshot{current}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, stage := range pipeline.Stages {
		if i < 4 {
			count := 2
			if i < 2 {
				count = 3
			}
			if stage.SampleCount != count || stage.EstimatedDurationMS == nil || *stage.EstimatedDurationMS != 2000 {
				t.Errorf("estimate for %s = %+v, want %d samples averaging 2000ms", stage.ID, stage, count)
			}
		} else if stage.EstimatedDurationMS != nil || stage.SampleCount != 0 {
			t.Errorf("manual/instant stage has an estimate: %+v", stage)
		}
	}
}

func TestPipelineEstimatesAreLimitedToTenRecentSuccesses(t *testing.T) {
	base := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	current := pipelineTestTask(base.Add(24*time.Hour), time.Second, model.TaskStatusRunning, "dispatched")
	release := pipelineTestRelease(current)
	repo := newFakeReleaseRepo()
	for i := 0; i < 11; i++ {
		spacing := time.Second
		if i == 0 {
			spacing = 100 * time.Second
		}
		task := pipelineTestTask(base.Add(time.Duration(i)*time.Hour), spacing, model.TaskStatusSucceeded,
			"dispatched", "accepted", "image_pulled", "container_started", "healthy")
		repo.tasks[task.ID] = &model.Task{ID: task.ID, ServiceID: release.ServiceID, ReleaseID: uuid.New(),
			Type: task.Type, Status: task.Status, CompletedAt: task.CompletedAt, LastStep: task.LastStep}
		for _, attempt := range task.Attempts {
			repo.taskAttempts = append(repo.taskAttempts, &model.TaskAttempt{ID: attempt.ID, TaskID: task.ID,
				Status: attempt.Status, Message: attempt.Message, StartedAt: attempt.StartedAt, CompletedAt: attempt.CompletedAt,
				Model: commondb.Model{CreatedAt: *attempt.StartedAt}})
		}
	}
	pipeline, err := (&Service{repo: repo}).GetPipeline(release, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range pipeline.Stages[:4] {
		if stage.SampleCount != 10 || stage.EstimatedDurationMS == nil || *stage.EstimatedDurationMS != 1000 {
			t.Fatalf("old outlier affected %s: %+v", stage.ID, stage)
		}
	}
	if !reflect.DeepEqual(pipeline.Runs, []dto.PipelineRun{}) {
		t.Fatalf("no execution should have produced no rows: %+v", pipeline.Runs)
	}
}

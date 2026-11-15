package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/real-uangi/edge-pilot/internal/shared/model"
)

type ReleasePipeline struct {
	Stages []PipelineStage `json:"stages"`
	Runs   []PipelineRun   `json:"runs"`
}

type PipelineStage struct {
	ID                  string `json:"id"`
	Label               string `json:"label"`
	Kind                string `json:"kind"`
	EstimatedDurationMS *int64 `json:"estimatedDurationMs"`
	SampleCount         int    `json:"sampleCount"`
}

type PipelineRun struct {
	TaskID      uuid.UUID        `json:"taskId"`
	Sequence    int              `json:"sequence"`
	Status      model.TaskStatus `json:"status"`
	StartedAt   time.Time        `json:"startedAt"`
	CompletedAt *time.Time       `json:"completedAt"`
	ReplayCount int              `json:"replayCount"`
	Cells       []PipelineCell   `json:"cells"`
}

type PipelineCell struct {
	StageID string `json:"stageId"`
	Status  string `json:"status"`
	// DurationMS contains closed time segments; add time since RunningSince while active.
	DurationMS   *int64          `json:"durationMs"`
	RunningSince *time.Time      `json:"runningSince"`
	Events       []PipelineEvent `json:"events"`
}

type PipelineEvent struct {
	ID      uuid.UUID `json:"id"`
	Kind    string    `json:"kind"`
	Code    string    `json:"code"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

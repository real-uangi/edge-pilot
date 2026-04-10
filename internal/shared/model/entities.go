package model

import (
	"time"

	"github.com/google/uuid"
	commondb "github.com/real-uangi/allingo/common/db"
)

type Slot int

const (
	SlotBlue Slot = iota + 1
	SlotGreen
)

type ReleaseStatus int

const (
	ReleaseStatusQueued ReleaseStatus = iota + 1
	ReleaseStatusDispatching
	ReleaseStatusDeploying
	ReleaseStatusReadyToSwitch
	ReleaseStatusSwitched
	ReleaseStatusCompleted
	ReleaseStatusFailed
	ReleaseStatusRolledBack
	ReleaseStatusSkipped
)

func (s ReleaseStatus) IsActive() bool {
	switch s {
	case ReleaseStatusDispatching,
		ReleaseStatusDeploying,
		ReleaseStatusReadyToSwitch,
		ReleaseStatusSwitched:
		return true
	default:
		return false
	}
}

func (s ReleaseStatus) IsQueued() bool {
	return s == ReleaseStatusQueued
}

type TaskType int

const (
	TaskTypeDeployGreen TaskType = iota + 1
	TaskTypeSwitchTraffic
	TaskTypeRollback
	TaskTypeCleanupOld
)

type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota + 1
	TaskStatusDispatched
	TaskStatusRunning
	TaskStatusSucceeded
	TaskStatusFailed
	TaskStatusTimedOut
)

type Service struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	commondb.Model
	ServiceKey        string                             `json:"serviceKey" gorm:"size:128;uniqueIndex;not null"`
	Name              string                             `json:"name" gorm:"size:255;not null"`
	AgentID           string                             `json:"agentId" gorm:"size:128;index;not null"`
	ImageRepo         string                             `json:"imageRepo" gorm:"size:512;not null"`
	ContainerPort     int                                `json:"containerPort"`
	CurrentLiveSlot   Slot                               `json:"currentLiveSlot"`
	DockerHealthCheck *bool                              `json:"dockerHealthCheck" gorm:"not null"`
	HTTPHealthPath    string                             `json:"httpHealthPath" gorm:"size:255"`
	HTTPExpectedCode  int                                `json:"httpExpectedCode"`
	HTTPTimeoutSecond int                                `json:"httpTimeoutSecond"`
	RouteHost         string                             `json:"routeHost" gorm:"size:255;index;not null"`
	RoutePathPrefix   string                             `json:"routePathPrefix" gorm:"size:255;index;not null"`
	Env               *commondb.JSONB[map[string]string] `json:"env" gorm:"type:jsonb"`
	Command           *commondb.JSONB[[]string]          `json:"command" gorm:"type:jsonb"`
	Entrypoint        *commondb.JSONB[[]string]          `json:"entrypoint" gorm:"type:jsonb"`
	Volumes           *commondb.JSONB[[]VolumeMount]     `json:"volumes" gorm:"type:jsonb"`
	PublishedPorts    *commondb.JSONB[[]PublishedPort]   `json:"publishedPorts" gorm:"type:jsonb"`
	Enabled           *bool                              `json:"enabled" gorm:"not null"`
}

func (Service) TableName() string {
	return "ep_service"
}

type RegistryCredential struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	commondb.Model
	RegistryHost     string `json:"registryHost" gorm:"size:255;uniqueIndex;not null"`
	Username         string `json:"username" gorm:"size:255;not null"`
	SecretCiphertext string `json:"secretCiphertext" gorm:"type:text;not null"`
	SecretKeyVersion string `json:"secretKeyVersion" gorm:"size:64;not null"`
}

func (RegistryCredential) TableName() string {
	return "ep_registry_credential"
}

type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly"`
}

type PublishedPort struct {
	HostPort      int `json:"hostPort"`
	ContainerPort int `json:"containerPort"`
}

type Release struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	commondb.Model
	ServiceID        uuid.UUID     `json:"serviceId" gorm:"type:uuid;index;not null"`
	AgentID          string        `json:"agentId" gorm:"size:128;index;not null"`
	ImageRepo        string        `json:"imageRepo" gorm:"size:512;not null"`
	ImageTag         string        `json:"imageTag" gorm:"size:255;not null"`
	CommitSHA        string        `json:"commitSha" gorm:"size:128"`
	TriggeredBy      string        `json:"triggeredBy" gorm:"size:255"`
	TraceID          string        `json:"traceId" gorm:"size:255;index"`
	Status           ReleaseStatus `json:"status" gorm:"index;not null"`
	TargetSlot       Slot          `json:"targetSlot"`
	PreviousLiveSlot Slot          `json:"previousLiveSlot"`
	CurrentTaskID    *uuid.UUID    `json:"currentTaskId" gorm:"type:uuid"`
	SwitchConfirmed  *bool         `json:"switchConfirmed" gorm:"not null"`
	CompletedAt      *time.Time    `json:"completedAt" gorm:"type:timestamptz"`
}

func (Release) TableName() string {
	return "ep_release"
}

type Task struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	commondb.Model
	ReleaseID    uuid.UUID                    `json:"releaseId" gorm:"type:uuid;index;not null"`
	ServiceID    uuid.UUID                    `json:"serviceId" gorm:"type:uuid;index;not null"`
	AgentID      string                       `json:"agentId" gorm:"size:128;index;not null"`
	Type         TaskType                     `json:"type" gorm:"index;not null"`
	Status       TaskStatus                   `json:"status" gorm:"index;not null"`
	Payload      *commondb.JSONB[TaskPayload] `json:"payload" gorm:"type:jsonb"`
	LastError    string                       `json:"lastError" gorm:"type:text"`
	DispatchedAt *time.Time                   `json:"dispatchedAt" gorm:"type:timestamptz"`
	StartedAt    *time.Time                   `json:"startedAt" gorm:"type:timestamptz"`
	CompletedAt  *time.Time                   `json:"completedAt" gorm:"type:timestamptz"`
}

func (Task) TableName() string {
	return "ep_task"
}

type TaskPayload struct {
	ServiceID         uuid.UUID         `json:"serviceId"`
	ServiceKey        string            `json:"serviceKey"`
	ImageRepo         string            `json:"imageRepo"`
	ImageTag          string            `json:"imageTag"`
	RegistryHost      string            `json:"registryHost,omitempty"`
	RegistryUsername  string            `json:"registryUsername,omitempty"`
	RegistrySecret    string            `json:"registrySecret,omitempty"`
	CommitSHA         string            `json:"commitSha"`
	TraceID           string            `json:"traceId"`
	TargetSlot        Slot              `json:"targetSlot"`
	CurrentLiveSlot   Slot              `json:"currentLiveSlot"`
	ContainerPort     int               `json:"containerPort"`
	DockerHealthCheck bool              `json:"dockerHealthCheck"`
	HTTPHealthPath    string            `json:"httpHealthPath"`
	HTTPExpectedCode  int               `json:"httpExpectedCode"`
	HTTPTimeoutSecond int               `json:"httpTimeoutSecond"`
	BackendName       string            `json:"backendName"`
	ServerName        string            `json:"serverName"`
	PreviousServer    string            `json:"previousServer"`
	Env               map[string]string `json:"env,omitempty"`
	Command           []string          `json:"command,omitempty"`
	Entrypoint        []string          `json:"entrypoint,omitempty"`
	Volumes           []VolumeMount     `json:"volumes,omitempty"`
	PublishedPorts    []PublishedPort   `json:"publishedPorts,omitempty"`
}

type TaskAttempt struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	commondb.Model
	TaskID      uuid.UUID  `json:"taskId" gorm:"type:uuid;index;not null"`
	AgentID     string     `json:"agentId" gorm:"size:128;index;not null"`
	Status      TaskStatus `json:"status" gorm:"index;not null"`
	Message     string     `json:"message" gorm:"type:text"`
	StartedAt   *time.Time `json:"startedAt" gorm:"type:timestamptz"`
	CompletedAt *time.Time `json:"completedAt" gorm:"type:timestamptz"`
}

func (TaskAttempt) TableName() string {
	return "ep_task_attempt"
}

type AgentNode struct {
	ID string `json:"id" gorm:"size:128;primaryKey"`
	commondb.Model
	TokenHash       string                    `json:"tokenHash" gorm:"size:128;not null"`
	Enabled         *bool                     `json:"enabled" gorm:"not null"`
	Hostname        string                    `json:"hostname" gorm:"size:255"`
	Version         string                    `json:"version" gorm:"size:128"`
	Online          *bool                     `json:"online" gorm:"not null"`
	LastConnectedAt *time.Time                `json:"lastConnectedAt" gorm:"type:timestamptz"`
	LastHeartbeatAt *time.Time                `json:"lastHeartbeatAt" gorm:"type:timestamptz"`
	TokenRotatedAt  *time.Time                `json:"tokenRotatedAt" gorm:"type:timestamptz"`
	LastError       string                    `json:"lastError" gorm:"type:text"`
	Capabilities    *commondb.JSONB[[]string] `json:"capabilities" gorm:"type:jsonb"`
}

func (AgentNode) TableName() string {
	return "ep_agent_node"
}

type RuntimeInstance struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	commondb.Model
	ServiceID        uuid.UUID `json:"serviceId" gorm:"type:uuid;index;not null"`
	ReleaseID        uuid.UUID `json:"releaseId" gorm:"type:uuid;index;not null"`
	Slot             Slot      `json:"slot" gorm:"index;not null"`
	ContainerID      string    `json:"containerId" gorm:"size:255"`
	ImageTag         string    `json:"imageTag" gorm:"size:255"`
	ListenAddress    string    `json:"listenAddress" gorm:"size:255"`
	HostPort         int       `json:"hostPort"`
	ServerName       string    `json:"serverName" gorm:"size:255"`
	Healthy          *bool     `json:"healthy" gorm:"not null"`
	AcceptingTraffic *bool     `json:"acceptingTraffic" gorm:"not null"`
	Active           *bool     `json:"active" gorm:"not null"`
}

func (RuntimeInstance) TableName() string {
	return "ep_runtime_instance"
}

type AuditLog struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	commondb.Model
	AggregateType string                             `json:"aggregateType" gorm:"size:128;index;not null"`
	AggregateID   string                             `json:"aggregateId" gorm:"size:128;index;not null"`
	EventType     string                             `json:"eventType" gorm:"size:128;index;not null"`
	Message       string                             `json:"message" gorm:"type:text"`
	TraceID       string                             `json:"traceId" gorm:"size:255;index"`
	Metadata      *commondb.JSONB[map[string]string] `json:"metadata" gorm:"type:jsonb"`
}

func (AuditLog) TableName() string {
	return "ep_audit_log"
}

type BackendStatSnapshot struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	commondb.Model
	ServiceID     uuid.UUID `json:"serviceId" gorm:"type:uuid;index;not null"`
	BackendName   string    `json:"backendName" gorm:"size:255;index;not null"`
	ServerName    string    `json:"serverName" gorm:"size:255;index;not null"`
	Scur          int64     `json:"scur"`
	Rate          int64     `json:"rate"`
	ErrorRequests int64     `json:"errorRequests"`
}

func (BackendStatSnapshot) TableName() string {
	return "ep_backend_stat_snapshot"
}

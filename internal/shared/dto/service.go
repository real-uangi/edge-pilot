package dto

import (
	"edge-pilot/internal/shared/model"
	"time"

	"github.com/google/uuid"
)

type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly"`
}

type UpsertServiceRequest struct {
	Name               string            `json:"name" binding:"required"`
	ServiceKey         string            `json:"serviceKey" binding:"required"`
	AgentID            string            `json:"agentId" binding:"required"`
	ImageRepo          string            `json:"imageRepo" binding:"required"`
	ContainerPort      int               `json:"containerPort" binding:"required"`
	BlueHostPort       int               `json:"blueHostPort" binding:"required"`
	GreenHostPort      int               `json:"greenHostPort" binding:"required"`
	DockerHealthCheck  *bool             `json:"dockerHealthCheck"`
	HTTPHealthPath     string            `json:"httpHealthPath"`
	HTTPExpectedCode   int               `json:"httpExpectedCode"`
	HTTPTimeoutSecond  int               `json:"httpTimeoutSecond"`
	HAProxyBackend     string            `json:"haproxyBackend" binding:"required"`
	HAProxyBlueServer  string            `json:"haproxyBlueServer" binding:"required"`
	HAProxyGreenServer string            `json:"haproxyGreenServer" binding:"required"`
	Env                map[string]string `json:"env"`
	Command            []string          `json:"command"`
	Entrypoint         []string          `json:"entrypoint"`
	Volumes            []VolumeMount     `json:"volumes"`
	Enabled            *bool             `json:"enabled"`
}

type ServiceOutput struct {
	ID                 uuid.UUID         `json:"id"`
	Name               string            `json:"name"`
	ServiceKey         string            `json:"serviceKey"`
	AgentID            string            `json:"agentId"`
	ImageRepo          string            `json:"imageRepo"`
	ContainerPort      int               `json:"containerPort"`
	BlueHostPort       int               `json:"blueHostPort"`
	GreenHostPort      int               `json:"greenHostPort"`
	CurrentLiveSlot    model.Slot        `json:"currentLiveSlot"`
	DockerHealthCheck  *bool             `json:"dockerHealthCheck"`
	HTTPHealthPath     string            `json:"httpHealthPath"`
	HTTPExpectedCode   int               `json:"httpExpectedCode"`
	HTTPTimeoutSecond  int               `json:"httpTimeoutSecond"`
	HAProxyBackend     string            `json:"haproxyBackend"`
	HAProxyBlueServer  string            `json:"haproxyBlueServer"`
	HAProxyGreenServer string            `json:"haproxyGreenServer"`
	Env                map[string]string `json:"env"`
	Command            []string          `json:"command"`
	Entrypoint         []string          `json:"entrypoint"`
	Volumes            []VolumeMount     `json:"volumes"`
	Enabled            *bool             `json:"enabled"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

type ServiceDeploymentSpec struct {
	ID                 uuid.UUID
	Name               string
	ServiceKey         string
	AgentID            string
	ImageRepo          string
	ContainerPort      int
	BlueHostPort       int
	GreenHostPort      int
	CurrentLiveSlot    model.Slot
	DockerHealthCheck  bool
	HTTPHealthPath     string
	HTTPExpectedCode   int
	HTTPTimeoutSecond  int
	HAProxyBackend     string
	HAProxyBlueServer  string
	HAProxyGreenServer string
	Env                map[string]string
	Command            []string
	Entrypoint         []string
	Volumes            []VolumeMount
	Enabled            bool
}

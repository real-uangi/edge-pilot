package dto

import (
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"github.com/google/uuid"
)

type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly"`
}

type PublishedPort struct {
	HostPort      int `json:"hostPort"`
	ContainerPort int `json:"containerPort"`
}

type UpsertServiceRequest struct {
	Name                    string            `json:"name" binding:"required"`
	ServiceKey              string            `json:"serviceKey" binding:"required,max=24"`
	AgentID                 string            `json:"agentId" binding:"required"`
	ImageRepo               string            `json:"imageRepo" binding:"required"`
	ContainerPort           int               `json:"containerPort" binding:"required"`
	CPULimitCores           float64           `json:"cpuLimitCores"`
	MemoryLimitMB           int64             `json:"memoryLimitMB"`
	DockerHealthCheck       *bool             `json:"dockerHealthCheck"`
	HTTPHealthPath          string            `json:"httpHealthPath"`
	HTTPHealthHeaders       map[string]string `json:"httpHealthHeaders"`
	HTTPExpectedCode        int               `json:"httpExpectedCode"`
	HTTPTimeoutSecond       int               `json:"httpTimeoutSecond"`
	StartupGraceSecond      int               `json:"startupGraceSecond"`
	HTTPProbeTimeoutSecond  int               `json:"httpProbeTimeoutSecond"`
	HTTPProbeIntervalSecond int               `json:"httpProbeIntervalSecond"`
	HTTPSuccessThreshold    int               `json:"httpSuccessThreshold"`
	SchedulerSDKPort        int               `json:"schedulerSdkPort"`
	SchedulerExecutorGroup  string            `json:"schedulerExecutorGroup"`
	RouteHost               string            `json:"routeHost"`
	RouteHosts              []string          `json:"routeHosts"`
	RoutePathPrefix         string            `json:"routePathPrefix"`
	Env                     map[string]string `json:"env"`
	Command                 []string          `json:"command"`
	Entrypoint              []string          `json:"entrypoint"`
	Volumes                 []VolumeMount     `json:"volumes"`
	NetworkAliases          []string          `json:"networkAliases"`
	PublishedPorts          []PublishedPort   `json:"publishedPorts"`
	Enabled                 *bool             `json:"enabled"`
}

type ServiceOutput struct {
	ID                      uuid.UUID         `json:"id"`
	Name                    string            `json:"name"`
	ServiceKey              string            `json:"serviceKey"`
	AgentID                 string            `json:"agentId"`
	ImageRepo               string            `json:"imageRepo"`
	ContainerPort           int               `json:"containerPort"`
	CPULimitCores           float64           `json:"cpuLimitCores"`
	MemoryLimitMB           int64             `json:"memoryLimitMB"`
	CurrentLiveSlot         model.Slot        `json:"currentLiveSlot"`
	DockerHealthCheck       *bool             `json:"dockerHealthCheck"`
	HTTPHealthPath          string            `json:"httpHealthPath"`
	HTTPHealthHeaders       map[string]string `json:"httpHealthHeaders"`
	HTTPExpectedCode        int               `json:"httpExpectedCode"`
	HTTPTimeoutSecond       int               `json:"httpTimeoutSecond"`
	StartupGraceSecond      int               `json:"startupGraceSecond"`
	HTTPProbeTimeoutSecond  int               `json:"httpProbeTimeoutSecond"`
	HTTPProbeIntervalSecond int               `json:"httpProbeIntervalSecond"`
	HTTPSuccessThreshold    int               `json:"httpSuccessThreshold"`
	SchedulerSDKPort        int               `json:"schedulerSdkPort"`
	SchedulerExecutorGroup  string            `json:"schedulerExecutorGroup"`
	RouteHost               string            `json:"routeHost"`
	RouteHosts              []string          `json:"routeHosts"`
	RoutePathPrefix         string            `json:"routePathPrefix"`
	Env                     map[string]string `json:"env"`
	Command                 []string          `json:"command"`
	Entrypoint              []string          `json:"entrypoint"`
	Volumes                 []VolumeMount     `json:"volumes"`
	NetworkAliases          []string          `json:"networkAliases"`
	PublishedPorts          []PublishedPort   `json:"publishedPorts"`
	Enabled                 *bool             `json:"enabled"`
	CreatedAt               time.Time         `json:"createdAt"`
	UpdatedAt               time.Time         `json:"updatedAt"`
}

type ServiceDeploymentSpec struct {
	ID                      uuid.UUID
	Name                    string
	ServiceKey              string
	AgentID                 string
	ImageRepo               string
	ContainerPort           int
	CPULimitCores           float64
	MemoryLimitMB           int64
	CurrentLiveSlot         model.Slot
	DockerHealthCheck       bool
	HTTPHealthPath          string
	HTTPHealthHeaders       map[string]string
	HTTPExpectedCode        int
	HTTPTimeoutSecond       int
	StartupGraceSecond      int
	HTTPProbeTimeoutSecond  int
	HTTPProbeIntervalSecond int
	HTTPSuccessThreshold    int
	SchedulerSDKPort        int
	SchedulerExecutorGroup  string
	RouteHost               string
	RouteHosts              []string
	RoutePathPrefix         string
	Env                     map[string]string
	EnvEncrypted            bool
	Command                 []string
	Entrypoint              []string
	Volumes                 []VolumeMount
	NetworkAliases          []string
	PublishedPorts          []PublishedPort
	Enabled                 bool
}

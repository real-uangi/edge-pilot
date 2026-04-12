package dto

import (
	"edge-pilot/internal/shared/model"
	"time"

	"github.com/google/uuid"
)

type AgentOverview struct {
	ID              string     `json:"id"`
	Enabled         *bool      `json:"enabled"`
	Hostname        string     `json:"hostname"`
	IP              string     `json:"ip"`
	Version         string     `json:"version"`
	Online          *bool      `json:"online"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt"`
}

type RuntimeInstanceOutput struct {
	ID               uuid.UUID  `json:"id"`
	ServiceID        uuid.UUID  `json:"serviceId"`
	ReleaseID        uuid.UUID  `json:"releaseId"`
	Slot             model.Slot `json:"slot"`
	ContainerID      string     `json:"containerId"`
	ImageTag         string     `json:"imageTag"`
	ListenAddress    string     `json:"listenAddress"`
	HostPort         int        `json:"hostPort"`
	ServerName       string     `json:"serverName"`
	Healthy          *bool      `json:"healthy"`
	AcceptingTraffic *bool      `json:"acceptingTraffic"`
	Active           *bool      `json:"active"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type BackendStatOutput struct {
	ServiceID     uuid.UUID `json:"serviceId"`
	BackendName   string    `json:"backendName"`
	ServerName    string    `json:"serverName"`
	Scur          int64     `json:"scur"`
	Rate          int64     `json:"rate"`
	ErrorRequests int64     `json:"errorRequests"`
	CreatedAt     time.Time `json:"createdAt"`
}

type ObservabilityOutput struct {
	ServiceID        uuid.UUID               `json:"serviceId"`
	RuntimeInstances []RuntimeInstanceOutput `json:"runtimeInstances"`
	BackendStats     []BackendStatOutput     `json:"backendStats"`
}

type OverviewOutput struct {
	Agents          []AgentOverview `json:"agents"`
	Services        []ServiceOutput `json:"services"`
	RecentReleases  []ReleaseOutput `json:"recentReleases"`
	ActiveInstances int             `json:"activeInstances"`
}

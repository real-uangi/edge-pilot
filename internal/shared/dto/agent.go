package dto

import "time"

type AgentOutput struct {
	ID              string     `json:"id"`
	Enabled         *bool      `json:"enabled"`
	Hostname        string     `json:"hostname"`
	IP              string     `json:"ip"`
	Version         string     `json:"version"`
	Online          *bool      `json:"online"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt"`
	LastConnectedAt *time.Time `json:"lastConnectedAt"`
	LastError       string     `json:"lastError"`
	TokenRotatedAt  *time.Time `json:"tokenRotatedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type AgentCredentialOutput struct {
	ID              string     `json:"id"`
	Token           string     `json:"token"`
	Enabled         *bool      `json:"enabled"`
	Hostname        string     `json:"hostname"`
	IP              string     `json:"ip"`
	Version         string     `json:"version"`
	Online          *bool      `json:"online"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt"`
	LastConnectedAt *time.Time `json:"lastConnectedAt"`
	LastError       string     `json:"lastError"`
	TokenRotatedAt  *time.Time `json:"tokenRotatedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

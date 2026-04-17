package dto

import "time"

type AgentHAProxyConfigOutput struct {
	AgentID   string    `json:"agentId"`
	Config    string    `json:"config"`
	FetchedAt time.Time `json:"fetchedAt"`
}

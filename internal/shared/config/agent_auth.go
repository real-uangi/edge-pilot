package config

import (
	"os"
	"strings"
)

type AgentAuthConfig struct {
	SharedToken string
	TokenByID   map[string]string
}

func LoadAgentAuthConfig() *AgentAuthConfig {
	cfg := &AgentAuthConfig{
		SharedToken: os.Getenv("AGENT_SHARED_TOKEN"),
		TokenByID:   make(map[string]string),
	}
	raw := os.Getenv("AGENT_TOKENS")
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		cfg.TokenByID[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return cfg
}

func (c *AgentAuthConfig) Validate(agentID string, token string) bool {
	if agentID == "" || token == "" {
		return false
	}
	if expected, ok := c.TokenByID[agentID]; ok {
		return expected == token
	}
	if c.SharedToken == "" {
		return false
	}
	return c.SharedToken == token
}

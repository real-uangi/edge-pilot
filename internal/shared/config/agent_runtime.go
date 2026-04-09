package config

import (
	"os"
	"strings"
)

type AgentRuntimeConfig struct {
	AgentID            string
	AgentToken         string
	ControlPlaneAddr   string
	AgentVersion       string
	Hostname           string
	DockerSocketPath   string
	HAProxyRuntimePath string
	HTTPProbeTimeoutS  int
}

func LoadAgentRuntimeConfig() *AgentRuntimeConfig {
	hostname, _ := os.Hostname()
	cfg := &AgentRuntimeConfig{
		AgentID:            defaultString(os.Getenv("AGENT_ID"), hostname),
		AgentToken:         os.Getenv("AGENT_TOKEN"),
		ControlPlaneAddr:   defaultString(os.Getenv("CONTROL_PLANE_GRPC_ADDR"), "127.0.0.1:9090"),
		AgentVersion:       defaultString(os.Getenv("AGENT_VERSION"), "dev"),
		Hostname:           hostname,
		DockerSocketPath:   defaultString(os.Getenv("DOCKER_SOCKET_PATH"), "/var/run/docker.sock"),
		HAProxyRuntimePath: defaultString(os.Getenv("HAPROXY_RUNTIME_SOCKET"), "/var/run/haproxy/admin.sock"),
		HTTPProbeTimeoutS:  defaultInt(os.Getenv("HTTP_PROBE_TIMEOUT_SECONDS"), 5),
	}
	return cfg
}

func defaultString(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func defaultInt(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	var v int
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return fallback
		}
		v = v*10 + int(ch-'0')
	}
	if v <= 0 {
		return fallback
	}
	return v
}

package config

import (
	"edge-pilot/internal/shared/buildinfo"
	"testing"
)

func TestLoadAgentRuntimeConfigUsesBuildVersionByDefault(t *testing.T) {
	t.Setenv("AGENT_ID", "11111111-1111-1111-1111-111111111111")
	t.Setenv("AGENT_TOKEN", "token")
	t.Setenv("CONTROL_PLANE_GRPC_ADDR", "")
	t.Setenv("DOCKER_SOCKET_PATH", "")
	t.Setenv("HTTP_PROBE_TIMEOUT_SECONDS", "")

	originalVersion := buildinfo.Version
	buildinfo.Version = "v1.2.3"
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
	})

	cfg, err := LoadAgentRuntimeConfig()
	if err != nil {
		t.Fatalf("LoadAgentRuntimeConfig() error = %v", err)
	}
	if cfg.AgentVersion != "v1.2.3" {
		t.Fatalf("expected build version fallback, got %q", cfg.AgentVersion)
	}
}

func TestLoadAgentRuntimeConfigRequiresIssuedCredentials(t *testing.T) {
	t.Setenv("CONTROL_PLANE_GRPC_ADDR", "")

	t.Setenv("AGENT_ID", "")
	t.Setenv("AGENT_TOKEN", "token")
	if _, err := LoadAgentRuntimeConfig(); err == nil {
		t.Fatalf("expected missing AGENT_ID to fail")
	}

	t.Setenv("AGENT_ID", "not-a-uuid")
	t.Setenv("AGENT_TOKEN", "token")
	if _, err := LoadAgentRuntimeConfig(); err == nil {
		t.Fatalf("expected invalid AGENT_ID to fail")
	}

	t.Setenv("AGENT_ID", "11111111-1111-1111-1111-111111111111")
	t.Setenv("AGENT_TOKEN", "")
	if _, err := LoadAgentRuntimeConfig(); err == nil {
		t.Fatalf("expected missing AGENT_TOKEN to fail")
	}
}

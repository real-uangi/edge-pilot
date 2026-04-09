package config

import (
	"edge-pilot/internal/shared/buildinfo"
	"testing"
)

func TestLoadAgentRuntimeConfigUsesBuildVersionByDefault(t *testing.T) {
	t.Setenv("AGENT_ID", "")
	t.Setenv("AGENT_TOKEN", "")
	t.Setenv("CONTROL_PLANE_GRPC_ADDR", "")
	t.Setenv("DOCKER_SOCKET_PATH", "")
	t.Setenv("HAPROXY_RUNTIME_SOCKET", "")
	t.Setenv("HTTP_PROBE_TIMEOUT_SECONDS", "")

	originalVersion := buildinfo.Version
	buildinfo.Version = "v1.2.3"
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
	})

	cfg := LoadAgentRuntimeConfig()
	if cfg.AgentVersion != "v1.2.3" {
		t.Fatalf("expected build version fallback, got %q", cfg.AgentVersion)
	}
}

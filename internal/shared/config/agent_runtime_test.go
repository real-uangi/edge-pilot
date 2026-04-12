package config

import (
	"edge-pilot/internal/shared/buildinfo"
	"errors"
	"net"
	"testing"
	"time"
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

func TestDetectReportedIPReturnsLocalAddress(t *testing.T) {
	ip := detectReportedIP("10.0.0.1:9090", func(network string, address string) (net.Conn, error) {
		return &fakeNetConn{
			localAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.10"), Port: 34567},
		}, nil
	})
	if ip != "192.168.1.10" {
		t.Fatalf("expected detected ip, got %q", ip)
	}
}

func TestDetectReportedIPRejectsInvalidTarget(t *testing.T) {
	ip := detectReportedIP("not-a-valid-addr", func(network string, address string) (net.Conn, error) {
		t.Fatal("dial should not be called for invalid target")
		return nil, nil
	})
	if ip != "" {
		t.Fatalf("expected empty ip for invalid target, got %q", ip)
	}
}

func TestDetectReportedIPReturnsEmptyOnDialFailure(t *testing.T) {
	ip := detectReportedIP("10.0.0.1:9090", func(network string, address string) (net.Conn, error) {
		return nil, errors.New("dial failed")
	})
	if ip != "" {
		t.Fatalf("expected empty ip on dial failure, got %q", ip)
	}
}

type fakeNetConn struct {
	localAddr net.Addr
}

func (f *fakeNetConn) Read([]byte) (int, error)         { return 0, nil }
func (f *fakeNetConn) Write([]byte) (int, error)        { return 0, nil }
func (f *fakeNetConn) Close() error                     { return nil }
func (f *fakeNetConn) LocalAddr() net.Addr              { return f.localAddr }
func (f *fakeNetConn) RemoteAddr() net.Addr             { return &net.UDPAddr{} }
func (f *fakeNetConn) SetDeadline(time.Time) error      { return nil }
func (f *fakeNetConn) SetReadDeadline(time.Time) error  { return nil }
func (f *fakeNetConn) SetWriteDeadline(time.Time) error { return nil }

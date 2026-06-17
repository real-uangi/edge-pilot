package agent

import (
	"context"
	"testing"

	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
)

func TestHandleHAProxyConfigRequestSuccess(t *testing.T) {
	client := &Client{
		cfg:   &config.AgentRuntimeConfig{AgentID: "agent-a"},
		proxy: &fakeProxyRuntimeForConfig{configText: "global\n  daemon\n"},
	}
	outbound := make(chan *grpcapi.AgentMessage, 1)

	client.handleHAProxyConfigRequest(context.Background(), &grpcapi.HAProxyConfigRequest{
		RequestId: "req-1",
		AgentId:   "agent-a",
	}, outbound)

	msg := <-outbound
	resp := msg.GetHaproxyConfigResponse()
	if resp == nil {
		t.Fatal("expected haproxy config response")
	}
	if resp.GetRequestId() != "req-1" {
		t.Fatalf("unexpected request id %q", resp.GetRequestId())
	}
	if resp.GetConfig() != "global\n  daemon\n" {
		t.Fatalf("unexpected config %q", resp.GetConfig())
	}
	if resp.GetErrorMessage() != "" {
		t.Fatalf("unexpected error message %q", resp.GetErrorMessage())
	}
}

func TestHandleHAProxyConfigRequestError(t *testing.T) {
	client := &Client{
		cfg:   &config.AgentRuntimeConfig{AgentID: "agent-a"},
		proxy: &fakeProxyRuntimeForConfig{err: agentdomain.ErrProxyNotReady},
	}
	outbound := make(chan *grpcapi.AgentMessage, 1)

	client.handleHAProxyConfigRequest(context.Background(), &grpcapi.HAProxyConfigRequest{
		RequestId: "req-2",
		AgentId:   "agent-a",
	}, outbound)

	msg := <-outbound
	resp := msg.GetHaproxyConfigResponse()
	if resp == nil {
		t.Fatal("expected haproxy config response")
	}
	if resp.GetRequestId() != "req-2" {
		t.Fatalf("unexpected request id %q", resp.GetRequestId())
	}
	if resp.GetErrorMessage() == "" {
		t.Fatal("expected error message")
	}
}

type fakeProxyRuntimeForConfig struct {
	configText string
	err        error
}

func (f *fakeProxyRuntimeForConfig) EnsureReady(context.Context) error { return nil }
func (f *fakeProxyRuntimeForConfig) ApplySnapshot(context.Context, *grpcapi.ProxyConfigSnapshot) error {
	return nil
}
func (f *fakeProxyRuntimeForConfig) SetServerAddress(context.Context, string, string, string, int) error {
	return nil
}
func (f *fakeProxyRuntimeForConfig) EnableServer(context.Context, string, string) error { return nil }
func (f *fakeProxyRuntimeForConfig) DisableServer(context.Context, string, string) error {
	return nil
}
func (f *fakeProxyRuntimeForConfig) ShowStats(context.Context) ([]*grpcapi.BackendStatPoint, error) {
	return nil, nil
}
func (f *fakeProxyRuntimeForConfig) ShowConfig(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.configText, nil
}

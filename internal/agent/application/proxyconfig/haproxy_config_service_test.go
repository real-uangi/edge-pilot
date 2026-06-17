package proxyconfig

import (
	"context"
	"testing"
	"time"

	registryapp "github.com/real-uangi/edge-pilot/internal/agent/application/registry"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/model"
)

func TestHAProxyConfigServiceGetHAProxyConfigSuccess(t *testing.T) {
	auth := config.LoadAgentAuthConfig()
	online := true
	enabled := true
	repo := &fakeHAProxyConfigAgentRepo{
		node: &model.AgentNode{
			ID:      "agent-a",
			Online:  &online,
			Enabled: &enabled,
		},
	}
	registry := registryapp.NewRegistryService(auth, repo)
	service := NewHAProxyConfigService(registry, fakeHAProxyConfigRequester{
		configText: "global\n  daemon\n",
	})

	output, err := service.GetHAProxyConfig("agent-a")
	if err != nil {
		t.Fatalf("GetHAProxyConfig() error = %v", err)
	}
	if output.AgentID != "agent-a" {
		t.Fatalf("unexpected agent id %q", output.AgentID)
	}
	if output.Config != "global\n  daemon\n" {
		t.Fatalf("unexpected config %q", output.Config)
	}
}

func TestHAProxyConfigServiceGetHAProxyConfigOffline(t *testing.T) {
	auth := config.LoadAgentAuthConfig()
	online := false
	enabled := true
	repo := &fakeHAProxyConfigAgentRepo{
		node: &model.AgentNode{
			ID:      "agent-a",
			Online:  &online,
			Enabled: &enabled,
		},
	}
	registry := registryapp.NewRegistryService(auth, repo)
	service := NewHAProxyConfigService(registry, fakeHAProxyConfigRequester{})

	if _, err := service.GetHAProxyConfig("agent-a"); err == nil {
		t.Fatal("expected offline error")
	}
}

func TestHAProxyConfigServiceGetHAProxyConfigTimeout(t *testing.T) {
	auth := config.LoadAgentAuthConfig()
	online := true
	enabled := true
	repo := &fakeHAProxyConfigAgentRepo{
		node: &model.AgentNode{
			ID:      "agent-a",
			Online:  &online,
			Enabled: &enabled,
		},
	}
	registry := registryapp.NewRegistryService(auth, repo)
	service := NewHAProxyConfigService(registry, fakeHAProxyConfigRequester{
		err: ErrHAProxyConfigTimeout,
	})

	if _, err := service.GetHAProxyConfig("agent-a"); err == nil {
		t.Fatal("expected timeout error")
	}
}

type fakeHAProxyConfigRequester struct {
	configText string
	err        error
}

func (f fakeHAProxyConfigRequester) RequestHAProxyConfig(context.Context, string) (string, error) {
	return f.configText, f.err
}

type fakeHAProxyConfigAgentRepo struct {
	node *model.AgentNode
}

func (f *fakeHAProxyConfigAgentRepo) Save(*model.AgentNode) error { return nil }
func (f *fakeHAProxyConfigAgentRepo) Get(string) (*model.AgentNode, error) {
	return f.node, nil
}
func (f *fakeHAProxyConfigAgentRepo) Delete(string) error              { return nil }
func (f *fakeHAProxyConfigAgentRepo) List() ([]model.AgentNode, error) { return nil, nil }
func (f *fakeHAProxyConfigAgentRepo) ListEnabled() ([]model.AgentNode, error) {
	return nil, nil
}
func (f *fakeHAProxyConfigAgentRepo) MarkOffline(string, string) error { return nil }
func (f *fakeHAProxyConfigAgentRepo) MarkOfflineStale(time.Time) ([]string, error) {
	return nil, nil
}

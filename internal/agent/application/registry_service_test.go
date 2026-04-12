package application

import (
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/model"
	"testing"
	"time"
)

func TestRegistryServiceCreateAndResetToken(t *testing.T) {
	repo := &fakeRegistryRepo{nodes: map[string]*model.AgentNode{}}
	svc := NewRegistryService(config.LoadAgentAuthConfig(), repo)

	created, err := svc.CreateAgent()
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if created.ID == "" || created.Token == "" {
		t.Fatalf("expected issued credentials, got %#v", created)
	}
	node := repo.nodes[created.ID]
	if node == nil || node.TokenHash == "" {
		t.Fatalf("expected token hash to be stored in repo")
	}
	if node.TokenHash == created.Token {
		t.Fatalf("expected repo to store hash instead of plaintext token")
	}
	if err := svc.Authenticate(created.ID, created.Token); err != nil {
		t.Fatalf("Authenticate() with issued token error = %v", err)
	}

	reset, err := svc.ResetToken(created.ID)
	if err != nil {
		t.Fatalf("ResetToken() error = %v", err)
	}
	if reset.Token == created.Token {
		t.Fatalf("expected reset token to differ from original token")
	}
	if err := svc.Authenticate(created.ID, created.Token); err == nil {
		t.Fatalf("expected old token to be rejected after reset")
	}
	if err := svc.Authenticate(created.ID, reset.Token); err != nil {
		t.Fatalf("expected reset token to authenticate, got %v", err)
	}
}

func TestRegistryServiceDisableRejectsAuthenticationAndScheduling(t *testing.T) {
	repo := &fakeRegistryRepo{nodes: map[string]*model.AgentNode{}}
	svc := NewRegistryService(config.LoadAgentAuthConfig(), repo)
	created, err := svc.CreateAgent()
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	disabled, err := svc.Disable(created.ID)
	if err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if disabled.Enabled == nil || *disabled.Enabled {
		t.Fatalf("expected agent to be disabled")
	}
	if err := svc.Authenticate(created.ID, created.Token); err == nil {
		t.Fatalf("expected disabled agent authentication to fail")
	}
	if online, err := svc.IsOnline(created.ID); err != nil || online {
		t.Fatalf("expected disabled agent to be non-schedulable, online=%v err=%v", online, err)
	}
}

func TestRegistryServiceMarkConnectedPersistsReportedIP(t *testing.T) {
	repo := &fakeRegistryRepo{nodes: map[string]*model.AgentNode{}}
	svc := NewRegistryService(config.LoadAgentAuthConfig(), repo)
	created, err := svc.CreateAgent()
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	if err := svc.MarkConnectedWithIP(created.ID, "host-a", "10.0.0.8", "v1.2.3", []string{"docker"}); err != nil {
		t.Fatalf("MarkConnectedWithIP() error = %v", err)
	}

	agent, err := svc.GetAgent(created.ID)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if agent.IP != "10.0.0.8" {
		t.Fatalf("expected reported ip to be returned, got %#v", agent)
	}

	list, err := svc.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].IP != "10.0.0.8" {
		t.Fatalf("expected overview ip to be returned, got %#v", list)
	}
}

func TestRegistryServiceMarkConnectedKeepsExistingIPWhenReportedIPIsEmpty(t *testing.T) {
	repo := &fakeRegistryRepo{nodes: map[string]*model.AgentNode{}}
	svc := NewRegistryService(config.LoadAgentAuthConfig(), repo)
	created, err := svc.CreateAgent()
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}

	if err := svc.MarkConnectedWithIP(created.ID, "host-a", "10.0.0.8", "v1.2.3", []string{"docker"}); err != nil {
		t.Fatalf("MarkConnectedWithIP() error = %v", err)
	}
	if err := svc.MarkConnectedWithIP(created.ID, "host-a", "", "v1.2.4", []string{"docker"}); err != nil {
		t.Fatalf("MarkConnectedWithIP() with empty ip error = %v", err)
	}

	agent, err := svc.GetAgent(created.ID)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if agent.IP != "10.0.0.8" {
		t.Fatalf("expected existing ip to be preserved, got %#v", agent)
	}
}

type fakeRegistryRepo struct {
	nodes map[string]*model.AgentNode
}

func (r *fakeRegistryRepo) Save(node *model.AgentNode) error {
	copyNode := *node
	if copyNode.CreatedAt.IsZero() {
		copyNode.CreatedAt = time.Now()
	}
	copyNode.UpdatedAt = time.Now()
	r.nodes[node.ID] = &copyNode
	return nil
}

func (r *fakeRegistryRepo) Get(id string) (*model.AgentNode, error) {
	node := r.nodes[id]
	if node == nil {
		return nil, nil
	}
	copyNode := *node
	return &copyNode, nil
}

func (r *fakeRegistryRepo) List() ([]model.AgentNode, error) {
	out := make([]model.AgentNode, 0, len(r.nodes))
	for _, item := range r.nodes {
		out = append(out, *item)
	}
	return out, nil
}

func (r *fakeRegistryRepo) ListEnabled() ([]model.AgentNode, error) {
	out := make([]model.AgentNode, 0, len(r.nodes))
	for _, item := range r.nodes {
		if item.Enabled != nil && *item.Enabled {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (r *fakeRegistryRepo) MarkOffline(id string, reason string) error {
	node := r.nodes[id]
	if node == nil {
		return nil
	}
	offline := false
	node.Online = &offline
	node.LastError = reason
	return nil
}

func (r *fakeRegistryRepo) MarkOfflineStale(before time.Time) ([]string, error) {
	return nil, nil
}

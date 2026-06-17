package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/real-uangi/edge-pilot/internal/shared/dto"

	"github.com/gin-gonic/gin"
)

func TestAgentRoutesDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/admin")
	agents := &fakeAgentAdminActions{}
	configs := &fakeAgentHAProxyConfigActions{}
	registerAdminAgentRoutes(admin, agents, configs)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/agents/11111111-1111-1111-1111-111111111111", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if agents.deletedID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected delete to receive parsed uuid, got %q", agents.deletedID)
	}
	if !strings.Contains(recorder.Body.String(), `"deleted":true`) {
		t.Fatalf("expected delete response to confirm deletion, got %s", recorder.Body.String())
	}
}

func TestAgentRoutesDeleteRejectInvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/admin")
	registerAdminAgentRoutes(admin, &fakeAgentAdminActions{}, &fakeAgentHAProxyConfigActions{})

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/agents/not-a-uuid", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAgentRoutesHAProxyConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/admin")
	actions := &fakeAgentHAProxyConfigActions{
		output: &dto.AgentHAProxyConfigOutput{
			AgentID: "11111111-1111-1111-1111-111111111111",
			Config:  "global\n  daemon\n",
		},
	}
	registerAdminAgentRoutes(admin, &fakeAgentAdminActions{}, actions)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/agents/11111111-1111-1111-1111-111111111111/haproxy-config", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if actions.agentID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected agent id to be passed, got %q", actions.agentID)
	}
	if !strings.Contains(recorder.Body.String(), "\"config\":\"global\\n  daemon\\n\"") {
		t.Fatalf("expected config payload, got %s", recorder.Body.String())
	}
}

type fakeAgentAdminActions struct {
	deletedID string
}

type fakeAgentHAProxyConfigActions struct {
	agentID string
	output  *dto.AgentHAProxyConfigOutput
	err     error
}

func (f *fakeAgentAdminActions) CreateAgent() (*dto.AgentCredentialOutput, error) {
	return &dto.AgentCredentialOutput{}, nil
}

func (f *fakeAgentAdminActions) ListAgents() ([]dto.AgentOutput, error) {
	return nil, nil
}

func (f *fakeAgentAdminActions) GetAgent(string) (*dto.AgentOutput, error) {
	return &dto.AgentOutput{}, nil
}

func (f *fakeAgentAdminActions) ResetToken(string) (*dto.AgentCredentialOutput, error) {
	return &dto.AgentCredentialOutput{}, nil
}

func (f *fakeAgentAdminActions) Enable(string) (*dto.AgentOutput, error) {
	return &dto.AgentOutput{}, nil
}

func (f *fakeAgentAdminActions) Disable(string) (*dto.AgentOutput, error) {
	return &dto.AgentOutput{}, nil
}

func (f *fakeAgentAdminActions) Delete(id string) error {
	f.deletedID = id
	return nil
}

func (f *fakeAgentHAProxyConfigActions) GetHAProxyConfig(agentID string) (*dto.AgentHAProxyConfigOutput, error) {
	f.agentID = agentID
	return f.output, f.err
}

package routes

import (
	"edge-pilot/internal/shared/dto"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestObservabilityRoutesSystemPerformance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/admin")
	svc := &fakeObservabilityAdminActions{
		systemOutput: &dto.SystemPerformanceOverviewOutput{
			ControlPlaneLatest: &dto.PerformancePointOutput{
				CPUPercent:       32.5,
				MemoryUsedBytes:  123,
				MemoryLimitBytes: 456,
				Source:           "cgroup",
				CollectedAt:      time.Now(),
			},
		},
	}
	registerObservabilityRoutes(admin, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/system/performance", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "\"controlPlaneLatest\"") {
		t.Fatalf("expected system performance payload, got %s", recorder.Body.String())
	}
}

func TestObservabilityRoutesAgentHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/admin")
	svc := &fakeObservabilityAdminActions{
		historyOutput: &dto.AgentPerformanceHistoryOutput{
			History: []dto.PerformancePointOutput{
				{CPUPercent: 19.3, Source: "cgroup", CollectedAt: time.Now()},
			},
		},
	}
	registerObservabilityRoutes(admin, svc)
	agentID := "11111111-1111-1111-1111-111111111111"

	req := httptest.NewRequest(http.MethodGet, "/api/admin/system/performance/agents/"+agentID+"/history", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if svc.historyAgentID != agentID {
		t.Fatalf("expected agent id to be parsed, got %q", svc.historyAgentID)
	}
}

func TestObservabilityRoutesAgentHistoryRejectInvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/admin")
	registerObservabilityRoutes(admin, &fakeObservabilityAdminActions{})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/system/performance/agents/not-a-uuid/history", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

type fakeObservabilityAdminActions struct {
	systemOutput   *dto.SystemPerformanceOverviewOutput
	historyOutput  *dto.AgentPerformanceHistoryOutput
	historyAgentID string
}

func (f *fakeObservabilityAdminActions) GetOverview() (*dto.OverviewOutput, error) {
	return &dto.OverviewOutput{}, nil
}

func (f *fakeObservabilityAdminActions) GetServiceObservability(uuid.UUID) (*dto.ObservabilityOutput, error) {
	return &dto.ObservabilityOutput{}, nil
}

func (f *fakeObservabilityAdminActions) GetSystemPerformanceOverview() (*dto.SystemPerformanceOverviewOutput, error) {
	if f.systemOutput == nil {
		return &dto.SystemPerformanceOverviewOutput{}, nil
	}
	return f.systemOutput, nil
}

func (f *fakeObservabilityAdminActions) GetAgentPerformanceHistory(agentID string) (*dto.AgentPerformanceHistoryOutput, error) {
	f.historyAgentID = agentID
	if f.historyOutput == nil {
		return &dto.AgentPerformanceHistoryOutput{AgentID: agentID}, nil
	}
	out := *f.historyOutput
	out.AgentID = agentID
	return &out, nil
}

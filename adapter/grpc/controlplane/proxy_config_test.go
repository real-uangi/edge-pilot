package controlplane

import (
	"edge-pilot/internal/shared/model"
	"testing"

	"github.com/google/uuid"
)

func TestBuildProxyConfigSnapshotCarriesSortedRoutesAndLiveSlot(t *testing.T) {
	enabled := true
	services := []model.Service{
		{
			ID:              uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			ServiceKey:      "svc-root",
			AgentID:         "agent-a",
			RouteHost:       "api.example.com",
			RoutePathPrefix: "/",
			CurrentLiveSlot: model.SlotBlue,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
		{
			ID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ServiceKey:      "svc-api",
			AgentID:         "agent-a",
			RouteHost:       "api.example.com",
			RoutePathPrefix: "/v1/internal",
			CurrentLiveSlot: model.SlotGreen,
			ContainerPort:   8080,
			Enabled:         &enabled,
		},
	}

	snapshot := buildProxyConfigSnapshot("agent-a", services)
	if snapshot.GetFrontendName() == "" || snapshot.GetDefaultBackend() == "" {
		t.Fatalf("expected managed frontend metadata")
	}
	if len(snapshot.GetServices()) != 2 {
		t.Fatalf("expected two services, got %d", len(snapshot.GetServices()))
	}
	if snapshot.GetServices()[0].GetServiceKey() != "svc-api" {
		t.Fatalf("expected longest path first, got %#v", snapshot.GetServices())
	}
	if snapshot.GetServices()[0].GetCurrentLiveSlot() != toProtoSlot(model.SlotGreen) {
		t.Fatalf("expected current live slot to be preserved")
	}
}

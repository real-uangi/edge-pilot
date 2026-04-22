package domain

import (
	"edge-pilot/internal/shared/grpcapi"
	"testing"
)

func TestManagedContainerNameForRelease(t *testing.T) {
	name := ManagedContainerNameForRelease("svc/a_demo", "rel_1/2 test")
	if name != "ep-svc-a-demo-rel-1-2-test" {
		t.Fatalf("expected sanitized release container name, got %q", name)
	}
}

func TestManagedContainerNameForTaskFallsBackToSlotWhenReleaseMissing(t *testing.T) {
	name := ManagedContainerNameForTask("svc-a", "", grpcapi.Slot_SLOT_GREEN)
	if name != "ep-svc-a-green" {
		t.Fatalf("expected slot fallback name, got %q", name)
	}
}

func TestManagedContainerNameForTaskPrefersReleaseID(t *testing.T) {
	name := ManagedContainerNameForTask("svc-a", "release-123", grpcapi.Slot_SLOT_GREEN)
	if name != "ep-svc-a-release-123" {
		t.Fatalf("expected release-based name, got %q", name)
	}
}

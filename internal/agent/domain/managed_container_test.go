package domain

import (
	"strings"
	"testing"

	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
)

func TestManagedContainerNameForRelease(t *testing.T) {
	name := ManagedContainerNameForRelease("svc/a_demo", "rel_1/2 test")
	if !strings.HasPrefix(name, "ep-svc-a-demo-") {
		t.Fatalf("expected sanitized release container prefix, got %q", name)
	}
	if len(name) != len("ep-svc-a-demo-")+10 {
		t.Fatalf("expected fixed hash suffix length, got %q", name)
	}
}

func TestManagedContainerNameForReleaseStable(t *testing.T) {
	nameA := ManagedContainerNameForRelease("svc.api", "rel.1")
	nameB := ManagedContainerNameForRelease("svc.api", "rel.1")
	if nameA != nameB {
		t.Fatalf("expected deterministic name, got %q and %q", nameA, nameB)
	}
	if !strings.HasPrefix(nameA, "ep-svc-api-") {
		t.Fatalf("expected dots to be replaced with dashes, got %q", nameA)
	}
}

func TestManagedContainerNameForTaskFallsBackToSlotWhenReleaseMissing(t *testing.T) {
	name := ManagedContainerNameForTask("svc-a", "", grpcapi.Slot_SLOT_GREEN)
	if !strings.HasPrefix(name, "ep-svc-a-") {
		t.Fatalf("expected slot fallback name, got %q", name)
	}
}

func TestManagedContainerNameForTaskPrefersReleaseID(t *testing.T) {
	name := ManagedContainerNameForTask("svc-a", "release-123", grpcapi.Slot_SLOT_GREEN)
	if !strings.HasPrefix(name, "ep-svc-a-") {
		t.Fatalf("expected release-based name, got %q", name)
	}
}

func TestManagedContainerNameForTaskDiffersByRelease(t *testing.T) {
	nameA := ManagedContainerNameForTask("svc-a", "release-1", grpcapi.Slot_SLOT_GREEN)
	nameB := ManagedContainerNameForTask("svc-a", "release-2", grpcapi.Slot_SLOT_GREEN)
	if nameA == nameB {
		t.Fatalf("expected distinct names for different releases, got %q", nameA)
	}
}

package domain

import (
	"edge-pilot/internal/shared/grpcapi"
	"fmt"
	"strings"
)

const (
	ManagedLabelKey        = "ep.managed"
	ManagedLabelValue      = "true"
	ManagedLabelAgentID    = "ep.agent_id"
	ManagedLabelServiceID  = "ep.service_id"
	ManagedLabelServiceKey = "ep.service_key"
	ManagedLabelSlot       = "ep.slot"
	ManagedLabelReleaseID  = "ep.release_id"
)

type ManagedContainer struct {
	ContainerRuntime
	Name       string
	Managed    bool
	AgentID    string
	ServiceID  string
	ServiceKey string
	ReleaseID  string
	Slot       grpcapi.Slot
	State      string
}

func ManagedContainerName(serviceKey string, slot grpcapi.Slot) string {
	return fmt.Sprintf("ep-%s-%s", sanitizeContainerToken(serviceKey, "service"), managedSlotName(slot))
}

func ManagedContainerNameForRelease(serviceKey string, releaseID string) string {
	serviceToken := sanitizeContainerToken(serviceKey, "service")
	releaseToken := sanitizeContainerToken(releaseID, "")
	if releaseToken == "" {
		return ""
	}
	return fmt.Sprintf("ep-%s-%s", serviceToken, releaseToken)
}

func ManagedContainerNameForTask(serviceKey string, releaseID string, slot grpcapi.Slot) string {
	if name := ManagedContainerNameForRelease(serviceKey, releaseID); name != "" {
		return name
	}
	return ManagedContainerName(serviceKey, slot)
}

func ManagedSlotValue(slot grpcapi.Slot) string {
	return managedSlotName(slot)
}

func sanitizeContainerToken(value string, fallback string) string {
	replacer := strings.NewReplacer("/", "-", "_", "-", " ", "-", ".", "-")
	name := replacer.Replace(strings.TrimSpace(value))
	name = strings.Trim(name, "-")
	if name == "" {
		return fallback
	}
	return name
}

func managedSlotName(slot grpcapi.Slot) string {
	switch slot {
	case grpcapi.Slot_SLOT_BLUE:
		return "blue"
	case grpcapi.Slot_SLOT_GREEN:
		return "green"
	default:
		return "unknown"
	}
}

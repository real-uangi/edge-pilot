package domain

import (
	"crypto/sha1"
	"edge-pilot/internal/shared/grpcapi"
	"encoding/hex"
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

type ContainerDetails struct {
	ContainerID  string
	Name         string
	State        string
	Image        string
	Running      bool
	Health       string
	RestartCount int32
	Labels       map[string]string
	Env          map[string]string
	Command      []string
	Entrypoint   []string
	IPAddress    string
	Volumes      []*VolumeMount
	Ports        []*PublishedPort
	CPULimit     float64
	MemoryLimit  int64
	CreatedAt    int64
}

type VolumeMount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type PublishedPort struct {
	ContainerPort int32
	HostPort      int32
}

type ManagedContainer struct {
	ContainerRuntime
	Name       string
	Image      string
	CreatedAt  int64
	Managed    bool
	AgentID    string
	ServiceID  string
	ServiceKey string
	ReleaseID  string
	Slot       grpcapi.Slot
	State      string
}

func ManagedContainerName(serviceKey string, slot grpcapi.Slot) string {
	serviceToken := sanitizeContainerToken(serviceKey, "service")
	return fmt.Sprintf("ep-%s-%s", serviceToken, shortIdentityHash(serviceToken, "", slot))
}

func ManagedContainerNameForRelease(serviceKey string, releaseID string) string {
	serviceToken := sanitizeContainerToken(serviceKey, "service")
	releaseToken := sanitizeContainerToken(releaseID, "")
	if releaseToken == "" {
		return ""
	}
	return fmt.Sprintf("ep-%s-%s", serviceToken, shortIdentityHash(serviceToken, releaseToken, grpcapi.Slot_SLOT_UNSPECIFIED))
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

func shortIdentityHash(serviceKey string, releaseID string, slot grpcapi.Slot) string {
	sum := sha1.Sum([]byte(strings.Join([]string{serviceKey, releaseID, managedSlotName(slot)}, "|")))
	return hex.EncodeToString(sum[:])[:10]
}

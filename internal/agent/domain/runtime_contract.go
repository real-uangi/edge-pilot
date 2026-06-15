package domain

import (
	"context"
	"errors"
	"io"
	"strings"

	"edge-pilot/internal/shared/grpcapi"
)

var ErrProxyNotReady = errors.New("proxy stack not ready")

type DockerRuntime interface {
	DeployContainer(context.Context, *grpcapi.TaskCommand) (*ContainerRuntime, error)
	InspectContainer(context.Context, string) (*ContainerStatus, error)
	GetContainerDetails(context.Context, string) (*ContainerDetails, error)
	FindContainerByName(context.Context, string) (*ManagedContainer, error)
	FindManagedContainerByIdentity(context.Context, ManagedContainerIdentity) (*ManagedContainer, error)
	ResolveListenAddress(context.Context, string, int) (string, error)
	ReadContainerLogs(context.Context, string, int, int) (string, error)
	StreamContainerLogs(context.Context, string, int, bool, bool, bool) (io.ReadCloser, error)
	RemoveContainer(context.Context, string) error
	RemoveImage(context.Context, string) error
	ListManagedContainers(context.Context, string, string) ([]*ManagedContainer, error)
}

type ManagedContainerIdentity struct {
	AgentID    string
	ServiceKey string
	ReleaseID  string
	Slot       grpcapi.Slot
}

type ProxyRuntime interface {
	EnsureReady(context.Context) error
	ApplySnapshot(context.Context, *grpcapi.ProxyConfigSnapshot) error
	SetServerAddress(context.Context, string, string, string, int) error
	EnableServer(context.Context, string, string) error
	DisableServer(context.Context, string, string) error
	ShowStats(context.Context) ([]*grpcapi.BackendStatPoint, error)
	ShowConfig(context.Context) (string, error)
}

type ContainerRuntime struct {
	ContainerID   string
	ListenAddress string
	Image         string
}

type ContainerStatus struct {
	State   string
	Health  string
	Running bool
}

func (s *ContainerStatus) Terminal() bool {
	if s == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(s.State)) {
	case "exited", "dead", "removing":
		return true
	default:
		return false
	}
}

package containerindex

import (
	"context"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/real-uangi/allingo/common/log"
)

type SnapshotStats struct {
	TotalManaged      int
	IdentityConflicts int
	LastRefreshAt     time.Time
	LastDuration      time.Duration
	LastError         string
}

type ManagedContainerIndex struct {
	cfg    *config.AgentRuntimeConfig
	docker agentdomain.DockerRuntime
	logger *log.StdLogger

	mu            sync.RWMutex
	containers    []*agentdomain.ManagedContainer
	identityIndex map[string][]*agentdomain.ManagedContainer
	stats         SnapshotStats
}

func NewManagedContainerIndex(cfg *config.AgentRuntimeConfig, docker agentdomain.DockerRuntime) *ManagedContainerIndex {
	return &ManagedContainerIndex{
		cfg:           cfg,
		docker:        docker,
		logger:        log.NewStdLogger("agent.container-index"),
		identityIndex: make(map[string][]*agentdomain.ManagedContainer),
	}
}

func (i *ManagedContainerIndex) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := i.RefreshNow(ctx); err != nil {
				i.logger.Errorf(err, "managed container index refresh failed: agentId=%s", i.cfg.AgentID)
			}
		}
	}
}

func (i *ManagedContainerIndex) RefreshNow(ctx context.Context) error {
	start := time.Now()
	items, err := i.docker.ListManagedContainers(ctx, i.cfg.AgentID, "")
	duration := time.Since(start)
	if err != nil {
		i.mu.Lock()
		i.stats.LastDuration = duration
		i.stats.LastRefreshAt = time.Now()
		i.stats.LastError = err.Error()
		i.mu.Unlock()
		return err
	}
	index := make(map[string][]*agentdomain.ManagedContainer)
	conflicts := 0
	for _, item := range items {
		if item == nil {
			continue
		}
		releaseKey := identityKey(item.ServiceKey, item.ReleaseID, item.Slot)
		if releaseKey != "" {
			index[releaseKey] = append(index[releaseKey], item)
			if len(index[releaseKey]) == 2 {
				conflicts++
			}
		}
	}
	i.mu.Lock()
	i.containers = cloneContainers(items)
	i.identityIndex = index
	i.stats = SnapshotStats{
		TotalManaged:      len(items),
		IdentityConflicts: conflicts,
		LastRefreshAt:     time.Now(),
		LastDuration:      duration,
	}
	i.mu.Unlock()
	return nil
}

func (i *ManagedContainerIndex) FindByIdentity(identity agentdomain.ManagedContainerIdentity) (*agentdomain.ManagedContainer, error) {
	key := identityKey(identity.ServiceKey, identity.ReleaseID, identity.Slot)
	if key == "" {
		return nil, nil
	}
	i.mu.RLock()
	items := i.identityIndex[key]
	i.mu.RUnlock()
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("managed container conflict: found %d containers for serviceKey=%s releaseId=%s slot=%s", len(items), identity.ServiceKey, identity.ReleaseID, identity.Slot.String())
	}
	return cloneContainer(items[0]), nil
}

func (i *ManagedContainerIndex) List() []*agentdomain.ManagedContainer {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return cloneContainers(i.containers)
}

func (i *ManagedContainerIndex) Stats() SnapshotStats {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.stats
}

func identityKey(serviceKey string, releaseID string, slot grpcapi.Slot) string {
	serviceKey = strings.TrimSpace(serviceKey)
	if serviceKey == "" {
		return ""
	}
	releaseID = strings.TrimSpace(releaseID)
	if releaseID != "" {
		return "release|" + serviceKey + "|" + releaseID
	}
	slotValue := strings.TrimSpace(agentdomain.ManagedSlotValue(slot))
	if slotValue == "" || slotValue == "unknown" {
		return ""
	}
	return "slot|" + serviceKey + "|" + slotValue
}

func cloneContainers(items []*agentdomain.ManagedContainer) []*agentdomain.ManagedContainer {
	out := make([]*agentdomain.ManagedContainer, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		copyItem := *item
		out = append(out, &copyItem)
	}
	return out
}

func cloneContainer(item *agentdomain.ManagedContainer) *agentdomain.ManagedContainer {
	if item == nil {
		return nil
	}
	copyItem := *item
	return &copyItem
}

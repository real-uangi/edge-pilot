package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	servicecatalogapp "github.com/real-uangi/edge-pilot/internal/servicecatalog/application"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"github.com/real-uangi/allingo/common/env"
	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
	"google.golang.org/protobuf/proto"
)

var containerIDPattern = regexp.MustCompile(`[0-9a-f]{12,64}`)

const managedProxyResolversName = "ep_dns"
const managedProxyInitAddrFallback = "last,libc,none"
const managedProxyDefaultsName = "unnamed_defaults_1"
const normalizeBackendName = "ep_normalize"

type managedProxyRuntimeAPI interface {
	SetServerAddress(context.Context, string, string, string, int) error
	EnableServer(context.Context, string, string) error
	DisableServer(context.Context, string, string) error
	ShowStats(context.Context) ([]*grpcapi.BackendStatPoint, error)
	run(context.Context, string) (string, error)
}

type managedProxyDataPlaneAPI interface {
	ConfigurationVersion(context.Context) (string, error)
	ShowRawConfig(context.Context) (string, error)
	ShowRawConfigInTransaction(context.Context, string) (string, error)
	StartTransaction(context.Context, string) (string, error)
	CommitTransaction(context.Context, string) error
	AbortTransaction(context.Context, string) error
	ReplaceFrontendInTransaction(context.Context, string, frontendSection) error
	EnsureBackendInTransaction(context.Context, string, backendSection) error
	EnsureServerInTransaction(context.Context, string, string, backendServer) error
	ListBackends(context.Context) ([]string, error)
	DeleteBackendInTransaction(context.Context, string, string) error
	ListFrontends(context.Context) ([]string, error)
	DeleteFrontendInTransaction(context.Context, string, string) error
}

type ManagedProxyRuntime struct {
	cfg       *config.AgentRuntimeConfig
	docker    *DockerClient
	runtime   managedProxyRuntimeAPI
	dataplane managedProxyDataPlaneAPI
	logger    *log.StdLogger

	mu                 sync.Mutex
	desired            *grpcapi.ProxyConfigSnapshot
	desiredHash        string
	appliedHash        string
	prepared           bool
	ready              bool
	attachedToNetwork  bool
	selfContainerID    string
	prebindInitialized bool
	preboundTCPPorts   []int
	lastPrepareError   string
	lastApplyErrorText string
}

func NewManagedProxyRuntime(cfg *config.AgentRuntimeConfig, docker *DockerClient) *ManagedProxyRuntime {
	runtime := &ManagedProxyRuntime{
		cfg:    cfg,
		docker: docker,
		logger: log.NewStdLogger("agent.proxy-stack"),
	}
	runtime.runtime = newHAProxyRuntimeClient(runtime.runtimeAddress)
	runtime.dataplane = newDataPlaneAPIClient(runtime.dataplaneBaseURL, func() string {
		return runtime.cfg.DataPlaneAPIUsername
	}, func() string {
		return runtime.cfg.DataPlaneAPIPassword
	})
	return runtime
}

func StartManagedProxyRuntime(lc fx.Lifecycle, runtime *ManagedProxyRuntime) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			runtime.logger.Infof("checking docker socket connectivity: agentId=%s", runtime.cfg.AgentID)
			if err := runtime.docker.Ping(startCtx); err != nil {
				runtime.logger.Errorf(err, "docker endpoint is not accessible: agentId=%s endpoint=%s", runtime.cfg.AgentID, runtime.docker.endpoint.display())
				return err
			}
			runtime.logger.Infof("docker endpoint is accessible: agentId=%s endpoint=%s", runtime.cfg.AgentID, runtime.docker.endpoint.display())
			prepareCtx, cancelPrepare := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancelPrepare()
			if err := runtime.Prepare(prepareCtx); err != nil {
				runtime.logger.Errorf(err, "proxy stack startup prepare failed: agentId=%s", runtime.cfg.AgentID)
				return err
			}
			go runtime.runSelfHeal(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func (m *ManagedProxyRuntime) Prepare(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.prepareLocked(ctx, true)
	return err
}

func (m *ManagedProxyRuntime) runSelfHeal(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.cfg.ProxySelfHealIntervalS) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			changed, err := m.prepareLocked(ctx, false)
			if err == nil && m.desired != nil && (changed || !m.ready || m.desiredHash != m.appliedHash) {
				err = m.reconcileLocked(ctx, m.desired)
				if err == nil {
					m.ready = true
					m.appliedHash = m.desiredHash
					m.lastApplyErrorText = ""
				} else {
					m.ready = false
					m.lastApplyErrorText = err.Error()
				}
			} else if err != nil {
				m.ready = false
				m.lastApplyErrorText = err.Error()
			}
			m.mu.Unlock()
			if err != nil {
				m.logger.Errorf(err, "proxy stack self-heal failed")
			}
		}
	}
}

func (m *ManagedProxyRuntime) ApplySnapshot(ctx context.Context, snapshot *grpcapi.ProxyConfigSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	desired := cloneSnapshot(snapshot)
	if err := m.validatePreboundTCPPortsLocked(desired); err != nil {
		m.ready = false
		m.lastApplyErrorText = err.Error()
		return err
	}
	m.desired = desired
	m.desiredHash = snapshotHash(m.desired)
	m.logger.Infof("received proxy snapshot: agentId=%s services=%d frontend=%s", m.cfg.AgentID, len(snapshot.GetServices()), snapshot.GetFrontendName())
	if _, err := m.prepareLocked(ctx, false); err != nil {
		m.ready = false
		m.lastApplyErrorText = err.Error()
		return err
	}
	if !m.prepared {
		m.ready = false
		if m.lastPrepareError == "" {
			m.lastPrepareError = "proxy stack is not prepared"
		}
		return fmt.Errorf("%w: %s", agentdomain.ErrProxyNotReady, m.lastPrepareError)
	}
	if err := m.reconcileLocked(ctx, m.desired); err != nil {
		m.ready = false
		m.lastApplyErrorText = err.Error()
		return err
	}
	m.ready = true
	m.appliedHash = m.desiredHash
	m.lastApplyErrorText = ""
	return nil
}

func (m *ManagedProxyRuntime) PreboundTCPPorts() []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]int(nil), m.preboundTCPPorts...)
}

func (m *ManagedProxyRuntime) EnsureReady(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureReadyLocked(ctx)
}

func (m *ManagedProxyRuntime) ShowStats(ctx context.Context) ([]*grpcapi.BackendStatPoint, error) {
	m.mu.Lock()
	prepared := m.prepared
	ready := m.ready
	prepareErr := m.lastPrepareError
	lastErr := m.lastApplyErrorText
	m.mu.Unlock()
	if !prepared {
		if strings.TrimSpace(prepareErr) == "" {
			prepareErr = "proxy stack is still preparing"
		}
		return nil, fmt.Errorf("%w: %s", agentdomain.ErrProxyNotReady, prepareErr)
	}
	if !ready {
		if strings.TrimSpace(lastErr) == "" {
			lastErr = "proxy stack is still bootstrapping"
		}
		return nil, fmt.Errorf("%w: %s", agentdomain.ErrProxyNotReady, lastErr)
	}
	return m.runtime.ShowStats(ctx)
}

func (m *ManagedProxyRuntime) ShowConfig(ctx context.Context) (string, error) {
	m.mu.Lock()
	prepared := m.prepared
	prepareErr := m.lastPrepareError
	m.mu.Unlock()
	if !prepared {
		if strings.TrimSpace(prepareErr) == "" {
			prepareErr = "proxy stack is still preparing"
		}
		return "", fmt.Errorf("%w: %s", agentdomain.ErrProxyNotReady, prepareErr)
	}
	return m.dataplane.ShowRawConfig(ctx)
}

func (m *ManagedProxyRuntime) SetServerAddress(ctx context.Context, backend string, server string, address string, port int) error {
	return m.runtime.SetServerAddress(ctx, backend, server, address, port)
}

func (m *ManagedProxyRuntime) EnableServer(ctx context.Context, backend string, server string) error {
	return m.runtime.EnableServer(ctx, backend, server)
}

func (m *ManagedProxyRuntime) DisableServer(ctx context.Context, backend string, server string) error {
	return m.runtime.DisableServer(ctx, backend, server)
}

func (m *ManagedProxyRuntime) ensureReadyLocked(ctx context.Context) error {
	if _, err := m.prepareLocked(ctx, false); err != nil {
		m.ready = false
		m.lastApplyErrorText = err.Error()
		return err
	}
	if m.desired == nil {
		m.ready = false
		m.lastApplyErrorText = "proxy config snapshot not received"
		return fmt.Errorf("proxy config snapshot not received")
	}
	if err := m.reconcileLocked(ctx, m.desired); err != nil {
		m.ready = false
		m.lastApplyErrorText = err.Error()
		return err
	}
	m.ready = true
	m.appliedHash = m.desiredHash
	m.lastApplyErrorText = ""
	return nil
}

func (m *ManagedProxyRuntime) prepareLocked(ctx context.Context, startup bool) (bool, error) {
	if startup {
		m.logger.Infof("pulling proxy stack images: haproxyImage=%s helperImage=%s", m.cfg.HAProxyImage, m.cfg.ProxyHelperImage)
	} else if !m.ready {
		m.logger.Infof("ensuring managed proxy stack: container=%s network=%s", m.cfg.ProxyContainerName, m.cfg.ProxyNetworkName)
	}
	changed := false
	if err := m.pullProxyImagesLocked(ctx); err != nil {
		m.prepared = false
		m.lastPrepareError = err.Error()
		return false, err
	}
	if startup {
		m.logger.Infof("preparing proxy network and config volume: container=%s network=%s volume=%s", m.cfg.ProxyContainerName, m.cfg.ProxyNetworkName, m.cfg.HAProxyConfigVolume)
	}
	if err := m.docker.ensureNetwork(ctx, m.cfg.ProxyNetworkName, m.cfg.ProxyNetworkSubnet); err != nil {
		m.prepared = false
		m.lastPrepareError = err.Error()
		return false, err
	}
	if err := m.ensureReservedProxyIPLocked(ctx); err != nil {
		m.prepared = false
		m.lastPrepareError = err.Error()
		return false, err
	}

	proxyInspect, err := m.docker.inspectManagedContainer(ctx, m.cfg.ProxyContainerName)
	if err != nil {
		return false, err
	}
	if !m.prebindInitialized {
		preboundTCPPorts, probeErr := m.docker.probeAvailableTCPHostPorts(
			ctx,
			m.cfg.ProxyHelperImage,
			tcpProxyPrebindPorts(),
			boundTCPHostPorts(proxyInspect),
		)
		if probeErr != nil {
			m.prepared = false
			m.lastPrepareError = probeErr.Error()
			return false, probeErr
		}
		m.preboundTCPPorts = preboundTCPPorts
		m.prebindInitialized = true
		m.logger.Infof(
			"tcp prebind pool selected: available=%d total=%d ports=%v",
			len(preboundTCPPorts),
			model.TCPProxyPrebindEndPort-model.TCPProxyPrebindStartPort+1,
			preboundTCPPorts,
		)
	}
	if proxyInspect == nil {
		if err := m.docker.recreateVolume(ctx, m.cfg.HAProxyConfigVolume); err != nil {
			m.prepared = false
			m.lastPrepareError = err.Error()
			return false, err
		}
		if err := m.bootstrapBaseFiles(ctx); err != nil {
			m.prepared = false
			m.lastPrepareError = err.Error()
			return false, err
		}
		changed = true
	} else {
		if err := m.docker.ensureVolume(ctx, m.cfg.HAProxyConfigVolume); err != nil {
			m.prepared = false
			m.lastPrepareError = err.Error()
			return false, err
		}
	}
	if proxyInspect == nil || proxyInspectNeedsBootstrapRefresh(proxyInspect, m.bootstrapFilesHash()) || !proxyInspect.State.Running {
		if err := m.bootstrapBaseFiles(ctx); err != nil {
			m.prepared = false
			m.lastPrepareError = err.Error()
			return false, err
		}
		changed = true
	}

	if startup {
		m.logger.Infof("starting proxy container: container=%s", m.cfg.ProxyContainerName)
	}
	containerChanged, err := m.docker.ensureManagedContainer(ctx, m.proxySpec())
	if err != nil {
		m.prepared = false
		m.lastPrepareError = err.Error()
		return false, err
	}
	if containerChanged {
		changed = true
	}
	if err := m.ensureSelfConnectedLocked(ctx); err != nil {
		m.prepared = false
		m.lastPrepareError = err.Error()
		return false, err
	}
	if startup {
		m.logger.Infof("waiting for proxy runtime api: addr=%s", m.runtimeAddress())
	}
	if err := retry(ctx, 12, time.Second, func() error {
		_, err := m.runtime.run(ctx, "show info")
		return err
	}); err != nil {
		m.prepared = false
		m.lastPrepareError = err.Error()
		return false, err
	}
	if startup {
		m.logger.Infof("waiting for dataplane api: baseURL=%s", m.dataplaneBaseURL())
	}
	if err := retry(ctx, 12, time.Second, func() error {
		_, err := m.dataplane.ConfigurationVersion(ctx)
		return err
	}); err != nil {
		m.prepared = false
		m.lastPrepareError = err.Error()
		return false, err
	}
	m.prepared = true
	m.lastPrepareError = ""
	return changed, nil
}

func (m *ManagedProxyRuntime) pullProxyImagesLocked(ctx context.Context) error {
	if err := m.docker.EnsureImage(ctx, m.cfg.HAProxyImage, nil); err != nil {
		return err
	}
	if strings.TrimSpace(m.cfg.ProxyHelperImage) == "" || m.cfg.ProxyHelperImage == m.cfg.HAProxyImage {
		return nil
	}
	return m.docker.EnsureImage(ctx, m.cfg.ProxyHelperImage, nil)
}

func (m *ManagedProxyRuntime) bootstrapBaseFiles(ctx context.Context) error {
	m.logger.Infof("bootstrapping proxy config files: container=%s", m.cfg.ProxyContainerName)
	return m.docker.writeVolumeFiles(ctx, m.cfg.ProxyHelperImage, m.cfg.HAProxyConfigVolume, m.bootstrapFiles())
}

func (m *ManagedProxyRuntime) reconcileLocked(ctx context.Context, snapshot *grpcapi.ProxyConfigSnapshot) error {
	m.logger.Infof("reconciling proxy snapshot: agentId=%s services=%d", m.cfg.AgentID, len(snapshot.GetServices()))
	version, err := m.dataplane.ConfigurationVersion(ctx)
	if err != nil {
		return err
	}
	transactionID, err := m.dataplane.StartTransaction(ctx, version)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		abortCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if abortErr := m.dataplane.AbortTransaction(abortCtx, transactionID); abortErr != nil {
			m.logger.Errorf(abortErr, "aborting dataplane transaction failed: transactionId=%s", transactionID)
		}
	}()
	frontend := m.frontendSection(snapshot)
	failureContext := dataplaneFailureContext{
		AgentID:                m.cfg.AgentID,
		TransactionID:          transactionID,
		Version:                version,
		Frontend:               frontend,
		IntendedFrontendConfig: renderIntendedFrontendConfig(frontend),
		DefaultBackend:         snapshot.GetDefaultBackend(),
		ServiceCount:           len(snapshot.GetServices()),
	}
	for _, service := range snapshot.GetServices() {
		for _, target := range []serviceBackendTarget{
			serviceLiveTarget(service),
			serviceCandidateTarget(service),
		} {
			if strings.TrimSpace(target.BackendName) == "" {
				continue
			}
			backend := backendSection{
				Name: target.BackendName,
				Mode: "http",
				From: managedProxyDefaultsName,
				Balance: &backendBalance{
					Algorithm: "roundrobin",
				},
				HTTPResponseRules: serviceBackendResponseRules(
					service.GetServiceKey(),
					service.GetRoutePathPrefix(),
					target.ReleaseID,
					strings.TrimSpace(service.GetLiveReleaseId()),
					strings.TrimSpace(service.GetCandidateReleaseId()),
					releaseRoleForTarget(target, service),
				),
			}
			failureContext.Backends = append(failureContext.Backends, backend)
			if err := m.dataplane.EnsureBackendInTransaction(ctx, transactionID, backend); err != nil {
				m.logDataplaneFailure(err, "dataplane ensure backend failed", failureContext)
				return err
			}
			server := backendServer{
				Name:      "srv",
				Address:   agentdomain.ManagedContainerNameForTask(service.GetServiceKey(), target.ReleaseID, target.Slot),
				Port:      int(service.GetContainerPort()),
				Check:     "enabled",
				Resolvers: managedProxyResolversName,
				InitAddr:  managedProxyInitAddrFallback,
			}
			failureContext.Servers = append(failureContext.Servers, dataplaneBackendServer{
				Backend: backend.Name,
				Server:  server,
			})
			if err := m.dataplane.EnsureServerInTransaction(ctx, backend.Name, transactionID, server); err != nil {
				m.logDataplaneFailure(err, "dataplane ensure server failed", failureContext)
				return err
			}
		}
	}
	tcpProxies := buildTCPProxyConfigs(snapshot)
	for _, proxy := range tcpProxies {
		for _, target := range []tcpProxyBackendTarget{
			{BackendName: proxy.LiveBackendName, ReleaseID: proxy.LiveReleaseID, Slot: proxy.LiveSlot},
			{BackendName: proxy.CandidateBackendName, ReleaseID: proxy.CandidateReleaseID, Slot: proxy.CandidateSlot},
		} {
			if strings.TrimSpace(target.BackendName) == "" || strings.TrimSpace(target.ReleaseID) == "" {
				continue
			}
			backend := tcpProxyBackendSection(target.BackendName, proxy.IdleTimeoutSecond)
			failureContext.Backends = append(failureContext.Backends, backend)
			if err := m.dataplane.EnsureBackendInTransaction(ctx, transactionID, backend); err != nil {
				m.logDataplaneFailure(err, "dataplane ensure tcp backend failed", failureContext)
				return err
			}
			server := backendServer{
				Name:      "srv",
				Address:   agentdomain.ManagedContainerNameForTask(proxy.ServiceKey, target.ReleaseID, target.Slot),
				Port:      proxy.ContainerPort,
				Check:     "enabled",
				Resolvers: managedProxyResolversName,
				InitAddr:  managedProxyInitAddrFallback,
			}
			failureContext.Servers = append(failureContext.Servers, dataplaneBackendServer{Backend: backend.Name, Server: server})
			if err := m.dataplane.EnsureServerInTransaction(ctx, backend.Name, transactionID, server); err != nil {
				m.logDataplaneFailure(err, "dataplane ensure tcp server failed", failureContext)
				return err
			}
		}
		unavailableBackend := tcpProxyBackendSection(proxy.UnavailableBackendName, proxy.IdleTimeoutSecond)
		failureContext.Backends = append(failureContext.Backends, unavailableBackend)
		if err := m.dataplane.EnsureBackendInTransaction(ctx, transactionID, unavailableBackend); err != nil {
			m.logDataplaneFailure(err, "dataplane ensure tcp unavailable backend failed", failureContext)
			return err
		}
		frontend := tcpProxyFrontendSection(proxy)
		if err := m.dataplane.ReplaceFrontendInTransaction(ctx, transactionID, frontend); err != nil {
			m.logDataplaneFailure(err, "dataplane replace tcp frontend failed", failureContext)
			return err
		}
	}
	normalizeBackend := backendSection{
		Name:             normalizeBackendName,
		Mode:             "http",
		HTTPRequestRules: normalizeBackendRequestRules(snapshot.GetServices()),
	}
	failureContext.Backends = append(failureContext.Backends, normalizeBackend)
	if err := m.dataplane.EnsureBackendInTransaction(ctx, transactionID, normalizeBackend); err != nil {
		m.logDataplaneFailure(err, "dataplane ensure normalize backend failed", failureContext)
		return err
	}
	if err := m.dataplane.ReplaceFrontendInTransaction(ctx, transactionID, frontend); err != nil {
		m.logDataplaneFailure(err, "dataplane replace frontend failed", failureContext)
		return err
	}
	existingFrontends, err := m.dataplane.ListFrontends(ctx)
	if err != nil {
		return err
	}
	desiredFrontends := map[string]struct{}{snapshot.GetFrontendName(): {}}
	for _, proxy := range tcpProxies {
		desiredFrontends[proxy.FrontendName] = struct{}{}
	}
	for _, name := range existingFrontends {
		if !isManagedTCPFrontend(name) {
			continue
		}
		if _, ok := desiredFrontends[name]; ok {
			continue
		}
		if err := m.dataplane.DeleteFrontendInTransaction(ctx, name, transactionID); err != nil {
			m.logDataplaneFailure(err, "dataplane delete stale tcp frontend failed", failureContext)
			return err
		}
	}
	existing, err := m.dataplane.ListBackends(ctx)
	if err != nil {
		return err
	}
	desiredBackends := map[string]struct{}{
		snapshot.GetDefaultBackend(): {},
		normalizeBackendName:         {},
	}
	for _, service := range snapshot.GetServices() {
		for _, name := range []string{
			strings.TrimSpace(service.GetLiveBackendName()),
			strings.TrimSpace(service.GetCandidateBackendName()),
		} {
			if name == "" {
				continue
			}
			desiredBackends[name] = struct{}{}
		}
	}
	for _, proxy := range tcpProxies {
		desiredBackends[proxy.UnavailableBackendName] = struct{}{}
		for _, backendName := range []string{proxy.LiveBackendName, proxy.CandidateBackendName} {
			if strings.TrimSpace(backendName) != "" {
				desiredBackends[backendName] = struct{}{}
			}
		}
	}
	failureContext.DesiredBackends = sortedBackendNames(desiredBackends)
	for _, name := range existing {
		if _, ok := desiredBackends[name]; ok {
			continue
		}
		failureContext.StaleBackends = append(failureContext.StaleBackends, name)
		m.logger.Infof("removing stale backend from dataplane: backend=%s", name)
		if err := m.dataplane.DeleteBackendInTransaction(ctx, name, transactionID); err != nil {
			m.logDataplaneFailure(err, "dataplane delete backend failed", failureContext)
			return err
		}
	}
	if err := m.dataplane.CommitTransaction(ctx, transactionID); err != nil {
		m.logDataplaneFailure(err, "dataplane commit transaction failed", failureContext)
		return err
	}
	committed = true
	m.cleanupStaleManagedContainers(ctx, snapshot)
	return nil
}

func (m *ManagedProxyRuntime) cleanupStaleManagedContainers(ctx context.Context, snapshot *grpcapi.ProxyConfigSnapshot) {
	if m.docker == nil || snapshot == nil {
		return
	}
	items, err := m.docker.ListManagedContainers(ctx, strings.TrimSpace(m.cfg.AgentID), "")
	if err != nil {
		m.logger.Errorf(err, "list managed containers for stale cleanup failed: agentId=%s", m.cfg.AgentID)
		return
	}
	staleContainerIDs, imageByContainerID := selectStaleManagedContainerIDs(items, snapshot)
	for _, containerID := range staleContainerIDs {
		m.logger.Infof("removing stale managed container after snapshot reconcile: agentId=%s containerId=%s", m.cfg.AgentID, containerID)
		if err := m.docker.RemoveContainer(ctx, containerID); err != nil {
			m.logger.Errorf(err, "remove stale managed container failed: agentId=%s containerId=%s", m.cfg.AgentID, containerID)
			continue
		}
		if image := imageByContainerID[containerID]; image != "" {
			if err := m.docker.RemoveImage(ctx, image); err != nil {
				m.logger.Errorf(err, "failed to remove docker image after stale managed container cleanup: agentId=%s containerId=%s image=%s", m.cfg.AgentID, containerID, image)
			}
		}
	}
}

func selectStaleManagedContainerIDs(items []*agentdomain.ManagedContainer, snapshot *grpcapi.ProxyConfigSnapshot) ([]string, map[string]string) {
	if len(items) == 0 {
		return nil, nil
	}
	desiredServiceKeys := make(map[string]struct{}, len(snapshot.GetServices()))
	for _, service := range snapshot.GetServices() {
		serviceKey := strings.TrimSpace(service.GetServiceKey())
		if serviceKey == "" {
			continue
		}
		desiredServiceKeys[serviceKey] = struct{}{}
	}
	staleContainerIDs := make([]string, 0, len(items))
	imageByContainerID := make(map[string]string, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		serviceKey := strings.TrimSpace(item.ServiceKey)
		if serviceKey == "" {
			continue
		}
		if _, ok := desiredServiceKeys[serviceKey]; ok {
			continue
		}
		containerID := strings.TrimSpace(item.ContainerID)
		if containerID == "" {
			continue
		}
		staleContainerIDs = append(staleContainerIDs, containerID)
		imageByContainerID[containerID] = strings.TrimSpace(item.Image)
	}
	return staleContainerIDs, imageByContainerID
}

type dataplaneFailureContext struct {
	AgentID                   string                   `json:"agentId"`
	TransactionID             string                   `json:"transactionId"`
	Version                   string                   `json:"version"`
	Frontend                  frontendSection          `json:"frontend"`
	IntendedFrontendConfig    string                   `json:"intendedFrontendConfig,omitempty"`
	TransactionRawConfig      string                   `json:"transactionRawConfig,omitempty"`
	TransactionRawConfigError string                   `json:"transactionRawConfigError,omitempty"`
	DefaultBackend            string                   `json:"defaultBackend"`
	ServiceCount              int                      `json:"serviceCount"`
	Backends                  []backendSection         `json:"backends,omitempty"`
	Servers                   []dataplaneBackendServer `json:"servers,omitempty"`
	DesiredBackends           []string                 `json:"desiredBackends,omitempty"`
	StaleBackends             []string                 `json:"staleBackends,omitempty"`
}

type dataplaneBackendServer struct {
	Backend string        `json:"backend"`
	Server  backendServer `json:"server"`
}

func (m *ManagedProxyRuntime) logDataplaneFailure(err error, message string, failureContext dataplaneFailureContext) {
	m.attachTransactionRawConfig(&failureContext)
	m.logger.Errorf(
		err,
		"%s: agentId=%s transactionId=%s version=%s frontend=%s defaultBackend=%s services=%d details=%s",
		message,
		failureContext.AgentID,
		failureContext.TransactionID,
		failureContext.Version,
		failureContext.Frontend.Name,
		failureContext.DefaultBackend,
		failureContext.ServiceCount,
		formatDataplaneFailureContext(failureContext),
	)
}

func (m *ManagedProxyRuntime) attachTransactionRawConfig(failureContext *dataplaneFailureContext) {
	if failureContext == nil || strings.TrimSpace(failureContext.TransactionID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rawConfig, err := m.dataplane.ShowRawConfigInTransaction(ctx, failureContext.TransactionID)
	if err != nil {
		failureContext.TransactionRawConfigError = err.Error()
		return
	}
	failureContext.TransactionRawConfig = rawConfig
}

func formatDataplaneFailureContext(failureContext dataplaneFailureContext) string {
	encoded, err := json.Marshal(failureContext)
	if err != nil {
		return fmt.Sprintf(`{"marshalError":%q}`, err.Error())
	}
	return string(encoded)
}

func renderIntendedFrontendConfig(frontend frontendSection) string {
	lines := make([]string, 0, len(frontend.ACLList)+len(frontend.BackendSwitchingRuleList)+len(frontend.HTTPRequestRules)+len(frontend.HTTPAfterResponseRules)+8)
	lines = append(lines, "frontend "+strings.TrimSpace(frontend.Name))
	if strings.TrimSpace(frontend.Mode) != "" {
		lines = append(lines, "  mode "+strings.TrimSpace(frontend.Mode))
	}
	bindNames := mapKeys(frontend.Binds)
	for _, name := range bindNames {
		bind := frontend.Binds[name]
		lines = append(lines, fmt.Sprintf("  bind %s:%d", strings.TrimSpace(bind.Address), bind.Port))
	}
	for _, acl := range frontend.ACLList {
		lines = append(lines, fmt.Sprintf("  acl %s %s %s", strings.TrimSpace(acl.Name), strings.TrimSpace(acl.Criterion), strings.TrimSpace(acl.Value)))
	}
	for _, rule := range frontend.BackendSwitchingRuleList {
		switchLine := fmt.Sprintf("  use_backend %s", strings.TrimSpace(rule.Name))
		condTest := strings.TrimSpace(rule.CondTest)
		if condTest != "" {
			switchLine += " " + strings.TrimSpace(rule.Cond) + " " + condTest
		}
		lines = append(lines, strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(switchLine, "  ", " "), "  ", " ")))
	}
	for _, rule := range frontend.HTTPRequestRules {
		action := strings.TrimSpace(rule.Type)
		if action == "" {
			action = strings.TrimSpace(rule.Action)
		}
		actionLine := ""
		switch action {
		case "set-header", "add-header":
			actionLine = fmt.Sprintf("  http-request %s %s %s", action, strings.TrimSpace(rule.Header), strings.TrimSpace(rule.Format))
		case "set-var":
			actionLine = fmt.Sprintf("  http-request set-var(%s.%s) %s", strings.TrimSpace(rule.VarScope), strings.TrimSpace(rule.VarName), strings.TrimSpace(rule.VarExpr))
		case "return":
			status := rule.ReturnStatusCode
			if status <= 0 {
				status = rule.Status
			}
			actionLine = fmt.Sprintf("  http-request return status %d", status)
			contentType := strings.TrimSpace(firstNonEmpty(rule.ReturnContentType, rule.ContentType))
			if contentType != "" {
				actionLine += " content-type " + contentType
			}
			contentFormat := strings.TrimSpace(rule.ReturnContentFormat)
			content := strings.TrimSpace(firstNonEmpty(rule.ReturnContent, rule.String))
			if contentFormat != "" && content != "" {
				actionLine += " " + contentFormat + " " + content
			}
			for _, header := range rule.ReturnHeaders {
				name := strings.TrimSpace(header.Name)
				format := strings.TrimSpace(header.Format)
				if name == "" || format == "" {
					continue
				}
				actionLine += " hdr " + name + " " + format
			}
		default:
			continue
		}
		cond := strings.TrimSpace(rule.Cond)
		condTest := strings.TrimSpace(rule.CondTest)
		if condTest != "" {
			if cond != "" {
				actionLine += " " + cond
			}
			actionLine += " " + condTest
		}
		lines = append(lines, strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(actionLine, "  ", " "), "  ", " ")))
	}
	for _, rule := range frontend.HTTPAfterResponseRules {
		action := strings.TrimSpace(rule.Type)
		if action == "" {
			action = strings.TrimSpace(rule.Action)
		}
		actionLine := fmt.Sprintf("  http-after-response %s %s %s", action, strings.TrimSpace(rule.Header), strings.TrimSpace(rule.Format))
		cond := strings.TrimSpace(rule.Cond)
		condTest := strings.TrimSpace(rule.CondTest)
		if condTest != "" {
			if cond != "" {
				actionLine += " " + cond
			}
			actionLine += " " + condTest
		}
		lines = append(lines, strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(actionLine, "  ", " "), "  ", " ")))
	}
	return strings.Join(lines, "\n")
}

func sortedBackendNames(values map[string]struct{}) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *ManagedProxyRuntime) ensureReservedProxyIPLocked(ctx context.Context) error {
	ip := net.ParseIP(strings.TrimSpace(m.cfg.ProxyIPAddress))
	if ip == nil {
		return fmt.Errorf("invalid proxy ip address: %s", m.cfg.ProxyIPAddress)
	}
	_, network, err := net.ParseCIDR(strings.TrimSpace(m.cfg.ProxyNetworkSubnet))
	if err != nil {
		return fmt.Errorf("invalid proxy network subnet: %w", err)
	}
	if !network.Contains(ip) {
		return fmt.Errorf("proxy ip %s is outside subnet %s", m.cfg.ProxyIPAddress, m.cfg.ProxyNetworkSubnet)
	}
	if ip.Equal(network.IP) || ip.Equal(lastIPv4(network)) {
		return fmt.Errorf("proxy ip %s cannot use network or broadcast address", m.cfg.ProxyIPAddress)
	}
	inspect, err := m.docker.inspectNetwork(ctx, m.cfg.ProxyNetworkName)
	if err != nil {
		return err
	}
	if inspect == nil {
		return fmt.Errorf("proxy network %s not found", m.cfg.ProxyNetworkName)
	}
	if len(inspect.IPAM.Config) > 0 && strings.TrimSpace(inspect.IPAM.Config[0].Subnet) != "" && strings.TrimSpace(inspect.IPAM.Config[0].Subnet) != strings.TrimSpace(m.cfg.ProxyNetworkSubnet) {
		return fmt.Errorf("proxy network subnet mismatch: expected %s got %s", m.cfg.ProxyNetworkSubnet, inspect.IPAM.Config[0].Subnet)
	}
	for _, item := range inspect.Containers {
		candidate := strings.TrimSpace(strings.Split(item.IPv4Address, "/")[0])
		if candidate == "" || candidate != m.cfg.ProxyIPAddress {
			continue
		}
		if item.Name == m.cfg.ProxyContainerName {
			return nil
		}
		return fmt.Errorf("proxy ip %s is already occupied by container %s", m.cfg.ProxyIPAddress, item.Name)
	}
	return nil
}

func proxyRouteHosts(service *grpcapi.ProxyServiceConfig) []string {
	if service == nil {
		return nil
	}
	hosts := make([]string, 0, len(service.GetRouteHosts())+1)
	seen := make(map[string]struct{}, len(service.GetRouteHosts())+1)
	add := func(value string) {
		host := servicecatalogapp.NormalizeRouteHost(value)
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}
	add(service.GetRouteHost())
	for _, host := range service.GetRouteHosts() {
		add(host)
	}
	return hosts
}

func proxyRouteHostKey(service *grpcapi.ProxyServiceConfig) string {
	hosts := proxyRouteHosts(service)
	if len(hosts) == 0 {
		return ""
	}
	return hosts[0]
}

func (m *ManagedProxyRuntime) frontendSection(snapshot *grpcapi.ProxyConfigSnapshot) frontendSection {
	services := append([]*grpcapi.ProxyServiceConfig(nil), snapshot.GetServices()...)
	sort.Slice(services, func(i, j int) bool {
		if len(services[i].GetRoutePathPrefix()) != len(services[j].GetRoutePathPrefix()) {
			return len(services[i].GetRoutePathPrefix()) > len(services[j].GetRoutePathPrefix())
		}
		leftHost := proxyRouteHostKey(services[i])
		rightHost := proxyRouteHostKey(services[j])
		if leftHost != rightHost {
			return leftHost < rightHost
		}
		return services[i].GetServiceKey() < services[j].GetServiceKey()
	})
	acls := make([]frontendACL, 0, len(services)*9+1)
	rules := make([]frontendSwitchRule, 0, len(services)*8)
	requestRules := staticAssetRequestRules()
	addACL := func(name string, criterion string, value string) {
		acls = append(acls, frontendACL{
			Name:      name,
			Criterion: criterion,
			Value:     value,
			Index:     len(acls),
		})
	}
	addRule := func(name string, condTest string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		rules = append(rules, frontendSwitchRule{
			Name:     name,
			Cond:     "if",
			CondTest: strings.TrimSpace(condTest),
			Index:    len(rules),
		})
	}
	addACL(staticAssetPathACLName, "path", "-m reg -i "+staticAssetPathRegex)
	for _, service := range services {
		hostACL := aclName(service.GetServiceId(), "host")
		pathACL := aclName(service.GetServiceId(), "path")
		normalizePathACL := aclName(service.GetServiceId(), "normalize_path")
		betaPathACL := aclName(service.GetServiceId(), "beta_path")
		queryLiveACL := aclName(service.GetServiceId(), "query_live")
		queryCandidateACL := aclName(service.GetServiceId(), "query_candidate")
		cookieLiveACL := aclName(service.GetServiceId(), "cookie_live")
		cookieCandidateACL := aclName(service.GetServiceId(), "cookie_candidate")
		splitACL := aclName(service.GetServiceId(), "split_candidate")
		cookieName := servicecatalogapp.StickyCookieName(service.GetServiceKey())
		liveRelease := strings.TrimSpace(service.GetLiveReleaseId())
		candidateRelease := strings.TrimSpace(service.GetCandidateReleaseId())
		liveBackend := strings.TrimSpace(service.GetLiveBackendName())
		candidateBackend := strings.TrimSpace(service.GetCandidateBackendName())
		trafficPercent := clampTrafficPercent(int(service.GetCandidateTrafficPercent()))
		normalizePath := servicecatalogapp.BuildStickyNormalizePath(service.GetRoutePathPrefix())
		betaPath := servicecatalogapp.BuildStickyBetaPath(service.GetRoutePathPrefix())

		addACL(hostACL, "hdr(host)", exactMatchValue(strings.Join(proxyRouteHosts(service), " ")))
		addACL(pathACL, "path_beg", service.GetRoutePathPrefix())
		addACL(normalizePathACL, "path", exactPathMatchValue(normalizePath))
		addACL(betaPathACL, "path", exactPathMatchValue(betaPath))
		addACL(queryLiveACL, "url_param("+servicecatalogapp.PreviewReleaseIDQueryParam+")", exactMatchValue(liveRelease))
		addACL(queryCandidateACL, "url_param("+servicecatalogapp.PreviewReleaseIDQueryParam+")", exactMatchValue(candidateRelease))
		addACL(cookieLiveACL, "cook("+cookieName+")", exactMatchValue(liveRelease))
		addACL(cookieCandidateACL, "cook("+cookieName+")", exactMatchValue(candidateRelease))

		baseMatch := hostACL + " " + pathACL
		normalizeMatch := baseMatch + " " + normalizePathACL
		if liveRelease != "" {
			addRule(normalizeBackendName, normalizeMatch)
		}
		betaMatch := baseMatch + " " + betaPathACL
		addRule(normalizeBackendName, betaMatch)
		baseNoOverrideParts := []string{
			baseMatch,
			"!" + normalizePathACL,
			"!" + betaPathACL,
			"!" + queryLiveACL,
			"!" + queryCandidateACL,
			"!" + cookieLiveACL,
		}

		addRule(candidateBackend, baseMatch+" "+queryCandidateACL)
		addRule(liveBackend, baseMatch+" "+queryLiveACL)
		if trafficPercent > 0 {
			addRule(candidateBackend, baseMatch+" !"+queryLiveACL+" !"+queryCandidateACL+" "+cookieCandidateACL)
			baseNoOverrideParts = append(baseNoOverrideParts, "!"+cookieCandidateACL)
		}
		addRule(liveBackend, baseMatch+" !"+queryLiveACL+" !"+queryCandidateACL+" "+cookieLiveACL)
		baseNoOverride := strings.Join(baseNoOverrideParts, " ")

		if liveBackend == "" && candidateBackend == "" {
			continue
		}
		if liveBackend == "" {
			addRule(candidateBackend, baseNoOverride)
			continue
		}
		if candidateBackend == "" {
			addRule(liveBackend, baseNoOverride)
			continue
		}
		if trafficPercent <= 0 {
			if candidateRelease != "" {
				addRule(candidateBackend, baseMatch+" !"+queryLiveACL+" !"+queryCandidateACL+" "+cookieCandidateACL)
			}
			addRule(liveBackend, baseNoOverride)
			continue
		}
		if trafficPercent >= 100 {
			addRule(candidateBackend, baseNoOverride)
			continue
		}
		addACL(splitACL, "rand(100)", fmt.Sprintf("lt %d", trafficPercent))
		addRule(candidateBackend, baseNoOverride+" "+splitACL)
		addRule(liveBackend, baseNoOverride+" !"+splitACL)
	}
	return frontendSection{
		Name:           snapshot.GetFrontendName(),
		Mode:           "http",
		DefaultBackend: snapshot.GetDefaultBackend(),
		Binds: map[string]frontendBind{
			"public": {
				Name:    "public",
				Address: "0.0.0.0",
				Port:    int(snapshot.GetBindPort()),
			},
		},
		ACLList:                  acls,
		BackendSwitchingRuleList: rules,
		HTTPRequestRules:         requestRules,
	}
}

type tcpProxyConfig struct {
	ServiceKey              string
	FrontendName            string
	UnavailableBackendName  string
	ListenPort              int
	ContainerPort           int
	IdleTimeoutSecond       int
	LiveReleaseID           string
	LiveBackendName         string
	LiveSlot                grpcapi.Slot
	CandidateReleaseID      string
	CandidateBackendName    string
	CandidateSlot           grpcapi.Slot
	CandidateTrafficPercent int
}

type tcpProxyBackendTarget struct {
	BackendName string
	ReleaseID   string
	Slot        grpcapi.Slot
}

func buildTCPProxyConfigs(snapshot *grpcapi.ProxyConfigSnapshot) []tcpProxyConfig {
	if snapshot == nil {
		return nil
	}
	configs := make([]tcpProxyConfig, 0)
	for _, service := range snapshot.GetServices() {
		if service == nil {
			continue
		}
		for _, port := range service.GetTcpProxyPorts() {
			if port == nil || port.GetListenPort() <= 0 || port.GetContainerPort() <= 0 {
				continue
			}
			idleTimeoutSecond := int(port.GetIdleTimeoutSecond())
			if idleTimeoutSecond <= 0 {
				idleTimeoutSecond = model.DefaultTCPProxyIdleTimeoutSec
			}
			configs = append(configs, tcpProxyConfig{
				ServiceKey:              service.GetServiceKey(),
				FrontendName:            servicecatalogapp.TCPFrontendName(int(port.GetListenPort())),
				UnavailableBackendName:  servicecatalogapp.TCPUnavailableBackendName(int(port.GetListenPort())),
				ListenPort:              int(port.GetListenPort()),
				ContainerPort:           int(port.GetContainerPort()),
				IdleTimeoutSecond:       idleTimeoutSecond,
				LiveReleaseID:           strings.TrimSpace(service.GetLiveReleaseId()),
				LiveBackendName:         strings.TrimSpace(port.GetLiveBackendName()),
				LiveSlot:                normalizedSlot(service.GetCurrentLiveSlot()),
				CandidateReleaseID:      strings.TrimSpace(service.GetCandidateReleaseId()),
				CandidateBackendName:    strings.TrimSpace(port.GetCandidateBackendName()),
				CandidateSlot:           oppositeSlot(normalizedSlot(service.GetCurrentLiveSlot())),
				CandidateTrafficPercent: clampTrafficPercent(int(service.GetCandidateTrafficPercent())),
			})
		}
	}
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].ListenPort < configs[j].ListenPort
	})
	return configs
}

func tcpProxyBackendSection(name string, idleTimeoutSecond int) backendSection {
	return backendSection{
		Name:          name,
		Mode:          "tcp",
		From:          managedProxyDefaultsName,
		Balance:       &backendBalance{Algorithm: "roundrobin"},
		ServerTimeout: timeoutMilliseconds(idleTimeoutSecond),
	}
}

func tcpProxyFrontendSection(proxy tcpProxyConfig) frontendSection {
	defaultBackend := proxy.UnavailableBackendName
	if proxy.LiveBackendName != "" {
		defaultBackend = proxy.LiveBackendName
	} else if proxy.CandidateBackendName != "" {
		defaultBackend = proxy.CandidateBackendName
	}
	frontend := frontendSection{
		Name:           proxy.FrontendName,
		Mode:           "tcp",
		DefaultBackend: defaultBackend,
		Binds: map[string]frontendBind{
			"public": {
				Name:    "public",
				Address: "0.0.0.0",
				Port:    proxy.ListenPort,
			},
		},
		ClientTimeout: timeoutMilliseconds(proxy.IdleTimeoutSecond),
	}
	if proxy.LiveBackendName == "" || proxy.CandidateBackendName == "" {
		return frontend
	}
	percent := clampTrafficPercent(proxy.CandidateTrafficPercent)
	if percent <= 0 {
		return frontend
	}
	if percent >= 100 {
		frontend.DefaultBackend = proxy.CandidateBackendName
		return frontend
	}
	const splitACL = "tcp_split_candidate"
	frontend.ACLList = []frontendACL{{
		Name:      splitACL,
		Criterion: "rand(100)",
		Value:     fmt.Sprintf("lt %d", percent),
		Index:     0,
	}}
	frontend.BackendSwitchingRuleList = []frontendSwitchRule{{
		Name:     proxy.CandidateBackendName,
		Cond:     "if",
		CondTest: splitACL,
		Index:    0,
	}}
	return frontend
}

func timeoutMilliseconds(seconds int) *int64 {
	if seconds <= 0 {
		seconds = model.DefaultTCPProxyIdleTimeoutSec
	}
	value := int64(seconds) * 1000
	return &value
}

func isManagedTCPFrontend(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "ep_tcp_")
}

func (m *ManagedProxyRuntime) proxySpec() managedContainerSpec {
	//兼容远程docker
	haproxyApiListenAddr := env.GetOrDefault("HAPROXY_API_LISTEN_ADDR", "127.0.0.1")
	exposed := map[string]map[string]string{
		portKey(servicecatalogapp.SharedFrontendBindPort): {},
		portKey(m.cfg.HAProxyRuntimePort):                 {},
		portKey(m.cfg.DataPlaneAPIPort):                   {},
	}
	portBinds := map[string][]dockerPortBinding{
		portKey(servicecatalogapp.SharedFrontendBindPort): {
			{HostIP: "0.0.0.0", HostPort: strconv.Itoa(servicecatalogapp.SharedFrontendBindPort)},
		},
		portKey(m.cfg.HAProxyRuntimePort): {
			{HostIP: haproxyApiListenAddr, HostPort: strconv.Itoa(m.cfg.HAProxyRuntimePort)},
		},
		portKey(m.cfg.DataPlaneAPIPort): {
			{HostIP: haproxyApiListenAddr, HostPort: strconv.Itoa(m.cfg.DataPlaneAPIPort)},
		},
	}
	for _, port := range m.preboundTCPPorts {
		key := portKey(port)
		exposed[key] = map[string]string{}
		portBinds[key] = []dockerPortBinding{{HostIP: "0.0.0.0", HostPort: strconv.Itoa(port)}}
	}
	for _, proxy := range buildTCPProxyConfigs(m.desired) {
		if isTCPProxyPrebindPort(proxy.ListenPort) {
			continue
		}
		key := portKey(proxy.ListenPort)
		exposed[key] = map[string]string{}
		portBinds[key] = []dockerPortBinding{{HostIP: "0.0.0.0", HostPort: strconv.Itoa(proxy.ListenPort)}}
	}
	return managedContainerSpec{
		Name:  m.cfg.ProxyContainerName,
		Image: m.cfg.HAProxyImage,
		Labels: map[string]string{
			proxyStackLabelKey:          "true",
			proxyStackRoleLabelKey:      "proxy",
			proxyStackAgentLabel:        m.cfg.AgentID,
			proxyStackBootstrapLabelKey: m.bootstrapFilesHash(),
		},
		Binds: []string{
			m.cfg.HAProxyConfigVolume + ":/usr/local/etc/haproxy",
		},
		Tmpfs: map[string]string{
			"/run": "exec,mode=755,size=16m",
		},
		Exposed:   exposed,
		PortBinds: portBinds,
		Network:   m.cfg.ProxyNetworkName,
		IPAddress: m.cfg.ProxyIPAddress,
		RestartPolicy: dockerRestartPolicy{
			Name:              "on-failure",
			MaximumRetryCount: 3,
		},
	}
}

func (m *ManagedProxyRuntime) validatePreboundTCPPortsLocked(snapshot *grpcapi.ProxyConfigSnapshot) error {
	if snapshot == nil {
		return nil
	}
	available := make(map[int]struct{}, len(m.preboundTCPPorts))
	for _, port := range m.preboundTCPPorts {
		available[port] = struct{}{}
	}
	for _, proxy := range buildTCPProxyConfigs(snapshot) {
		if !isTCPProxyPrebindPort(proxy.ListenPort) {
			continue
		}
		if _, ok := available[proxy.ListenPort]; !ok {
			return fmt.Errorf("tcp prebind port %d is unavailable on this agent", proxy.ListenPort)
		}
	}
	return nil
}

func tcpProxyPrebindPorts() []int {
	ports := make([]int, 0, model.TCPProxyPrebindEndPort-model.TCPProxyPrebindStartPort+1)
	for port := model.TCPProxyPrebindStartPort; port <= model.TCPProxyPrebindEndPort; port++ {
		ports = append(ports, port)
	}
	return ports
}

func isTCPProxyPrebindPort(port int) bool {
	return port >= model.TCPProxyPrebindStartPort && port <= model.TCPProxyPrebindEndPort
}

func (m *ManagedProxyRuntime) ensureSelfConnectedLocked(ctx context.Context) error {
	containerID, err := m.detectSelfContainerID(ctx)
	if err != nil {
		return err
	}
	if containerID == "" {
		m.attachedToNetwork = false
		m.selfContainerID = ""
		return nil
	}
	if err := m.docker.ensureContainerConnectedToNetwork(ctx, containerID, m.cfg.ProxyNetworkName); err != nil {
		return err
	}
	m.logger.Infof("agent container attached to proxy network: agentId=%s containerId=%s network=%s", m.cfg.AgentID, containerID, m.cfg.ProxyNetworkName)
	m.attachedToNetwork = true
	m.selfContainerID = containerID
	return nil
}

func (m *ManagedProxyRuntime) detectSelfContainerID(ctx context.Context) (string, error) {
	candidates := make([]string, 0, 4)
	if raw, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		for _, match := range containerIDPattern.FindAllString(string(raw), -1) {
			candidates = append(candidates, match)
		}
	}
	if m.cfg.Hostname != "" {
		candidates = append(candidates, m.cfg.Hostname)
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		inspect, err := m.docker.inspectManagedContainer(ctx, candidate)
		if err != nil {
			return "", err
		}
		if inspect != nil {
			return inspect.ID, nil
		}
	}
	return "", nil
}

func (m *ManagedProxyRuntime) runtimeAddress() string {
	// 兼容远程docker
	host := env.GetOrDefault("HAPROXY_API_ADDR", "127.0.0.1")
	if m.attachedToNetwork {
		host = m.cfg.ProxyIPAddress
	}
	return net.JoinHostPort(host, strconv.Itoa(m.cfg.HAProxyRuntimePort))
}

func (m *ManagedProxyRuntime) dataplaneBaseURL() string {
	// 兼容远程docker
	host := env.GetOrDefault("HAPROXY_API_ADDR", "127.0.0.1")
	if m.attachedToNetwork {
		host = m.cfg.ProxyIPAddress
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(m.cfg.DataPlaneAPIPort))
}

func snapshotHash(snapshot *grpcapi.ProxyConfigSnapshot) string {
	if snapshot == nil {
		return ""
	}
	raw, err := proto.Marshal(snapshot)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", raw)
}

func (m *ManagedProxyRuntime) bootstrapFiles() map[string]string {
	return map[string]string{
		"haproxy.cfg":      m.baseHAProxyConfig(),
		"dataplaneapi.yml": m.dataPlaneConfig(),
	}
}

func (m *ManagedProxyRuntime) bootstrapFilesHash() string {
	files := m.bootstrapFiles()
	keys := mapKeys(files)
	sum := sha256.New()
	for _, name := range keys {
		_, _ = sum.Write([]byte(name))
		_, _ = sum.Write([]byte{'\n'})
		_, _ = sum.Write([]byte(files[name]))
		_, _ = sum.Write([]byte{'\n'})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func proxyInspectNeedsBootstrapRefresh(inspect *dockerContainerInspect, expectedHash string) bool {
	if inspect == nil {
		return true
	}
	return strings.TrimSpace(inspect.Config.Labels[proxyStackBootstrapLabelKey]) != strings.TrimSpace(expectedHash)
}

func (m *ManagedProxyRuntime) baseHAProxyConfig() string {
	return fmt.Sprintf(`global
  log stdout format raw local0
  nosplice
  user root
  group root
  stats socket ipv4@0.0.0.0:%d level admin expose-fd listeners

userlist dataplaneapi
  user %s insecure-password %s

defaults %s
  log global
  mode http
  option httplog
  timeout connect 5s
  timeout client 30s
  timeout server 30s

resolvers %s
  parse-resolv-conf
  accepted_payload_size 8192
  resolve_retries 3
  timeout resolve 1s
  timeout retry 1s
  hold other 30s
  hold refused 30s
  hold nx 30s
  hold timeout 30s
  hold valid 10s
  hold obsolete 30s

frontend %s
  bind *:%d
  mode http
  default_backend %s

backend %s
  mode http
  http-request return status 503 content-type text/plain string no-route
`, m.cfg.HAProxyRuntimePort, m.cfg.DataPlaneAPIUsername, m.cfg.DataPlaneAPIPassword, managedProxyDefaultsName, managedProxyResolversName, servicecatalogapp.SharedFrontendName, servicecatalogapp.SharedFrontendBindPort, servicecatalogapp.SharedDefaultBackend, servicecatalogapp.SharedDefaultBackend)
}

func (m *ManagedProxyRuntime) dataPlaneConfig() string {
	return fmt.Sprintf(`dataplaneapi:
  host: 0.0.0.0
  port: %d
  userlist:
    userlist: dataplaneapi
  transaction:
    transaction_dir: /tmp/haproxy
  resources:
    maps_dir: /tmp/haproxy/maps
    ssl_certs_dir: /tmp/haproxy/ssl
haproxy:
  config_file: /usr/local/etc/haproxy/haproxy.cfg
  haproxy_bin: /usr/local/sbin/haproxy
  master_worker_mode: true
  master_runtime: /var/run/haproxy-master.sock
  reload:
    reload_strategy: s6
    reload_delay: 1
log_targets:
  - log_to: stdout
    log_level: info
    log_types:
      - app
      - access
`, m.cfg.DataPlaneAPIPort)
}

func cloneSnapshot(snapshot *grpcapi.ProxyConfigSnapshot) *grpcapi.ProxyConfigSnapshot {
	if snapshot == nil {
		return nil
	}
	out := &grpcapi.ProxyConfigSnapshot{
		AgentId:        snapshot.GetAgentId(),
		FrontendName:   snapshot.GetFrontendName(),
		DefaultBackend: snapshot.GetDefaultBackend(),
		BindPort:       snapshot.GetBindPort(),
		Services:       make([]*grpcapi.ProxyServiceConfig, 0, len(snapshot.GetServices())),
	}
	for _, item := range snapshot.GetServices() {
		out.Services = append(out.Services, &grpcapi.ProxyServiceConfig{
			ServiceId:               item.GetServiceId(),
			ServiceKey:              item.GetServiceKey(),
			RouteHost:               item.GetRouteHost(),
			RouteHosts:              append([]string(nil), item.GetRouteHosts()...),
			RoutePathPrefix:         item.GetRoutePathPrefix(),
			BackendName:             item.GetBackendName(),
			ContainerPort:           item.GetContainerPort(),
			CurrentLiveSlot:         item.GetCurrentLiveSlot(),
			LiveReleaseId:           item.GetLiveReleaseId(),
			LiveBackendName:         item.GetLiveBackendName(),
			CandidateReleaseId:      item.GetCandidateReleaseId(),
			CandidateBackendName:    item.GetCandidateBackendName(),
			CandidateTrafficPercent: item.GetCandidateTrafficPercent(),
			TcpProxyPorts:           cloneTCPProxyPorts(item.GetTcpProxyPorts()),
		})
	}
	return out
}

func cloneTCPProxyPorts(items []*grpcapi.TCPProxyPortConfig) []*grpcapi.TCPProxyPortConfig {
	out := make([]*grpcapi.TCPProxyPortConfig, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, &grpcapi.TCPProxyPortConfig{
			ListenPort:           item.GetListenPort(),
			ContainerPort:        item.GetContainerPort(),
			IdleTimeoutSecond:    item.GetIdleTimeoutSecond(),
			LiveBackendName:      item.GetLiveBackendName(),
			CandidateBackendName: item.GetCandidateBackendName(),
		})
	}
	return out
}

type serviceBackendTarget struct {
	BackendName string
	ReleaseID   string
	Slot        grpcapi.Slot
}

const (
	staticAssetPathACLName = "ep_static_asset_path"
	staticAssetTxnVarName  = "ep_static_asset"
	staticAssetPathRegex   = `\.(avif|bmp|css|eot|gif|ico|jpeg|jpg|js|m4a|map|mjs|mp3|mp4|ogg|otf|png|svg|ttf|wasm|wav|webm|webmanifest|webp|woff|woff2)$`
)

func staticAssetRequestRules() []httpRequestRule {
	return []httpRequestRule{
		{
			Type:     "set-var",
			VarScope: "txn",
			VarName:  staticAssetTxnVarName,
			VarExpr:  "bool(false)",
			Index:    0,
		},
		{
			Type:     "set-var",
			VarScope: "txn",
			VarName:  staticAssetTxnVarName,
			VarExpr:  "bool(true)",
			Cond:     "if",
			CondTest: staticAssetPathACLName,
			Index:    1,
		},
	}
}

func staticAssetVarCondTest() string {
	return "{ var(txn." + staticAssetTxnVarName + ") -m bool }"
}

func serviceBackendResponseRules(serviceKey string, routePathPrefix string, currentReleaseID string, liveReleaseID string, betaReleaseID string, releaseRole string) []httpResponseRule {
	cookieName := servicecatalogapp.StickyCookieName(serviceKey)
	rules := []httpResponseRule{
		{
			Type:     "add-header",
			Action:   "add-header",
			Header:   "Set-Cookie",
			Format:   servicecatalogapp.BuildStickyCookie(cookieName, currentReleaseID, routePathPrefix),
			Cond:     "unless",
			CondTest: staticAssetVarCondTest(),
		},
		{
			Type:     "del-header",
			Action:   "del-header",
			Header:   "Set-Cookie",
			Cond:     "if",
			CondTest: staticAssetVarCondTest(),
		},
		{
			Type:   "set-header",
			Action: "set-header",
			Header: servicecatalogapp.CurrentReleaseIDHeaderName,
			Format: currentReleaseID,
		},
		{
			Type:   "set-header",
			Action: "set-header",
			Header: servicecatalogapp.LiveReleaseIDHeaderName,
			Format: liveReleaseID,
		},
		{
			Type:   "set-header",
			Action: "set-header",
			Header: servicecatalogapp.BetaReleaseIDHeaderName,
			Format: strings.TrimSpace(betaReleaseID),
		},
		{
			Type:   "set-header",
			Action: "set-header",
			Header: servicecatalogapp.ReleaseRoleHeaderName,
			Format: strings.TrimSpace(releaseRole),
		},
	}
	return filterHTTPResponseRules(rules)
}

func normalizeBackendRequestRules(services []*grpcapi.ProxyServiceConfig) []httpRequestRule {
	rules := []httpRequestRule{
		{
			Type:     "set-var",
			VarScope: "txn",
			VarName:  "ep_normalize_path",
			VarExpr:  "path",
		},
	}
	for _, svc := range services {
		liveRelease := strings.TrimSpace(svc.GetLiveReleaseId())
		if liveRelease == "" {
			continue
		}
		cookieName := servicecatalogapp.StickyCookieName(svc.GetServiceKey())
		normalizePath := servicecatalogapp.BuildStickyNormalizePath(svc.GetRoutePathPrefix())
		condTest := fmt.Sprintf("{ req.hdr(host) -i %s } { var(txn.ep_normalize_path) -m str -i %s }", strings.Join(proxyRouteHosts(svc), " "), normalizePath)
		rules = append(rules, httpRequestRule{
			Type:             "return",
			ReturnStatusCode: 204,
			ReturnHeaders: normalizeReturnHeaders(
				servicecatalogapp.BuildStickyCookie(cookieName, liveRelease, svc.GetRoutePathPrefix()),
				liveRelease,
			),
			Cond:     "if",
			CondTest: condTest,
		})
		candidateRelease := strings.TrimSpace(svc.GetCandidateReleaseId())
		if candidateRelease == "" {
			continue
		}
		betaPath := servicecatalogapp.BuildStickyBetaPath(svc.GetRoutePathPrefix())
		betaCondTest := fmt.Sprintf("{ req.hdr(host) -i %s } { var(txn.ep_normalize_path) -m str -i %s }", strings.Join(proxyRouteHosts(svc), " "), betaPath)
		rules = append(rules, httpRequestRule{
			Type:             "return",
			ReturnStatusCode: 204,
			ReturnHeaders: betaReturnHeaders(
				servicecatalogapp.BuildStickyCookie(cookieName, candidateRelease, svc.GetRoutePathPrefix()),
				candidateRelease,
				liveRelease,
				releaseRoleForBeta(svc),
			),
			Cond:     "if",
			CondTest: betaCondTest,
		})
	}
	rules = append(rules, httpRequestRule{
		Type:             "return",
		ReturnStatusCode: 204,
		ReturnHeaders:    normalizeNoCacheHeaders(),
	})
	return filterHTTPRequestRules(rules)
}

func betaReturnHeaders(cookie string, candidateRelease string, liveRelease string, releaseRole string) []returnHeader {
	headers := append([]returnHeader{}, normalizeNoCacheHeaders()...)
	headers = append(headers,
		returnHeader{
			Name:   "Set-Cookie",
			Format: cookie,
		},
		returnHeader{
			Name:   servicecatalogapp.CurrentReleaseIDHeaderName,
			Format: candidateRelease,
		},
		returnHeader{
			Name:   servicecatalogapp.LiveReleaseIDHeaderName,
			Format: liveRelease,
		},
		returnHeader{
			Name:   servicecatalogapp.BetaReleaseIDHeaderName,
			Format: candidateRelease,
		},
		returnHeader{
			Name:   servicecatalogapp.ReleaseRoleHeaderName,
			Format: strings.TrimSpace(releaseRole),
		},
	)
	return headers
}

func normalizeReturnHeaders(cookie string, liveRelease string) []returnHeader {
	headers := append([]returnHeader{}, normalizeNoCacheHeaders()...)
	headers = append(headers,
		returnHeader{
			Name:   "Set-Cookie",
			Format: cookie,
		},
		returnHeader{
			Name:   servicecatalogapp.CurrentReleaseIDHeaderName,
			Format: liveRelease,
		},
		returnHeader{
			Name:   servicecatalogapp.LiveReleaseIDHeaderName,
			Format: liveRelease,
		},
		returnHeader{
			Name:   servicecatalogapp.ReleaseRoleHeaderName,
			Format: servicecatalogapp.ReleaseRoleLive,
		},
	)
	return headers
}

func normalizeNoCacheHeaders() []returnHeader {
	return []returnHeader{
		{
			Name:   "Cache-Control",
			Format: "no-store, no-cache, must-revalidate, max-age=0, private",
		},
		{
			Name:   "Surrogate-Control",
			Format: "no-store, max-age=0",
		},
		{
			Name:   "Pragma",
			Format: "no-cache",
		},
		{
			Name:   "Expires",
			Format: "0",
		},
	}
}

func releaseRoleForTarget(target serviceBackendTarget, service *grpcapi.ProxyServiceConfig) string {
	liveReleaseID := strings.TrimSpace(service.GetLiveReleaseId())
	candidateReleaseID := strings.TrimSpace(service.GetCandidateReleaseId())
	targetReleaseID := strings.TrimSpace(target.ReleaseID)
	if targetReleaseID == "" {
		return ""
	}
	if targetReleaseID == liveReleaseID {
		return servicecatalogapp.ReleaseRoleLive
	}
	if targetReleaseID == candidateReleaseID {
		if splitActiveForService(service) {
			return servicecatalogapp.ReleaseRoleCanary
		}
		return servicecatalogapp.ReleaseRoleHistorical
	}
	return servicecatalogapp.ReleaseRoleHistorical
}

func releaseRoleForBeta(service *grpcapi.ProxyServiceConfig) string {
	if splitActiveForService(service) {
		return servicecatalogapp.ReleaseRoleCanary
	}
	return servicecatalogapp.ReleaseRoleHistorical
}

func splitActiveForService(service *grpcapi.ProxyServiceConfig) bool {
	if service == nil {
		return false
	}
	percent := clampTrafficPercent(int(service.GetCandidateTrafficPercent()))
	return percent >= 1 && percent <= 99
}

func serviceLiveTarget(service *grpcapi.ProxyServiceConfig) serviceBackendTarget {
	slot := normalizedSlot(service.GetCurrentLiveSlot())
	return serviceBackendTarget{
		BackendName: strings.TrimSpace(service.GetLiveBackendName()),
		ReleaseID:   strings.TrimSpace(service.GetLiveReleaseId()),
		Slot:        slot,
	}
}

func serviceCandidateTarget(service *grpcapi.ProxyServiceConfig) serviceBackendTarget {
	return serviceBackendTarget{
		BackendName: strings.TrimSpace(service.GetCandidateBackendName()),
		ReleaseID:   strings.TrimSpace(service.GetCandidateReleaseId()),
		Slot:        oppositeSlot(normalizedSlot(service.GetCurrentLiveSlot())),
	}
}

func normalizedSlot(slot grpcapi.Slot) grpcapi.Slot {
	switch slot {
	case grpcapi.Slot_SLOT_BLUE, grpcapi.Slot_SLOT_GREEN:
		return slot
	default:
		return grpcapi.Slot_SLOT_BLUE
	}
}

func oppositeSlot(slot grpcapi.Slot) grpcapi.Slot {
	switch slot {
	case grpcapi.Slot_SLOT_BLUE:
		return grpcapi.Slot_SLOT_GREEN
	case grpcapi.Slot_SLOT_GREEN:
		return grpcapi.Slot_SLOT_BLUE
	default:
		return grpcapi.Slot_SLOT_GREEN
	}
}

func clampTrafficPercent(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func exactPathMatchValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-i __edge_pilot_invalid__"
	}
	return "-i " + value
}

func exactMatchValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-i __edge_pilot_invalid__"
	}
	return "-i " + value
}

func aclName(serviceID string, suffix string) string {
	replacer := strings.NewReplacer("/", "_", "-", "_", ".", "_", " ", "_")
	base := replacer.Replace(strings.TrimSpace(serviceID))
	if base == "" {
		base = "service"
	}
	return base + "_" + suffix
}

func retry(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := fn(); err != nil {
			lastErr = err
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		return nil
	}
	return lastErr
}

func lastIPv4(network *net.IPNet) net.IP {
	base := network.IP.To4()
	if base == nil {
		return nil
	}
	mask := network.Mask
	out := make(net.IP, len(base))
	for i := range base {
		out[i] = base[i] | ^mask[i]
	}
	return out
}

var _ agentdomain.ProxyRuntime = (*ManagedProxyRuntime)(nil)
var _ managedProxyRuntimeAPI = (*HAProxyRuntimeClient)(nil)
var _ managedProxyDataPlaneAPI = (*DataPlaneAPIClient)(nil)

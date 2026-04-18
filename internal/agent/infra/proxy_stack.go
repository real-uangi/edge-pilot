package infra

import (
	"context"
	"crypto/sha256"
	"edge-pilot/internal/agent/application"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"edge-pilot/internal/shared/model"
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

	"github.com/real-uangi/allingo/common/env"
	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
	"google.golang.org/protobuf/proto"
)

var containerIDPattern = regexp.MustCompile(`[0-9a-f]{12,64}`)

const managedProxyResolversName = "ep_dns"
const managedProxyInitAddrFallback = "last,libc,none"

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
	StartTransaction(context.Context, string) (string, error)
	CommitTransaction(context.Context, string) error
	AbortTransaction(context.Context, string) error
	ReplaceFrontendInTransaction(context.Context, string, frontendSection) error
	EnsureBackendInTransaction(context.Context, string, backendSection) error
	EnsureServerInTransaction(context.Context, string, string, backendServer) error
	ListBackends(context.Context) ([]string, error)
	DeleteBackendInTransaction(context.Context, string, string) error
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
	m.desired = cloneSnapshot(snapshot)
	m.desiredHash = snapshotHash(m.desired)
	m.logger.Infof("received proxy snapshot: agentId=%s services=%d frontend=%s", m.cfg.AgentID, len(snapshot.GetServices()), snapshot.GetFrontendName())
	if !m.prepared {
		m.ready = false
		if m.lastPrepareError == "" {
			m.lastPrepareError = "proxy stack is not prepared"
		}
		return fmt.Errorf("%w: %s", application.ErrProxyNotReady, m.lastPrepareError)
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
		return nil, fmt.Errorf("%w: %s", application.ErrProxyNotReady, prepareErr)
	}
	if !ready {
		if strings.TrimSpace(lastErr) == "" {
			lastErr = "proxy stack is still bootstrapping"
		}
		return nil, fmt.Errorf("%w: %s", application.ErrProxyNotReady, lastErr)
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
		return "", fmt.Errorf("%w: %s", application.ErrProxyNotReady, prepareErr)
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
	if err := m.docker.ensureImage(ctx, m.cfg.HAProxyImage, nil); err != nil {
		return err
	}
	if strings.TrimSpace(m.cfg.ProxyHelperImage) == "" || m.cfg.ProxyHelperImage == m.cfg.HAProxyImage {
		return nil
	}
	return m.docker.ensureImage(ctx, m.cfg.ProxyHelperImage, nil)
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
		ExpectedFrontendConfig: renderExpectedFrontendConfig(frontend),
		DefaultBackend:         snapshot.GetDefaultBackend(),
		ServiceCount:           len(snapshot.GetServices()),
	}
	for _, service := range snapshot.GetServices() {
		for _, slot := range []grpcapi.Slot{grpcapi.Slot_SLOT_BLUE, grpcapi.Slot_SLOT_GREEN} {
			backend := backendSection{
				Name: serviceBackendName(service, slot),
				Mode: "http",
				Balance: backendBalance{
					Algorithm: "roundrobin",
				},
			}
			failureContext.Backends = append(failureContext.Backends, backend)
			if err := m.dataplane.EnsureBackendInTransaction(ctx, transactionID, backend); err != nil {
				m.logDataplaneFailure(err, "dataplane ensure backend failed", failureContext)
				return err
			}
			server := backendServer{
				Name:      serviceServerName(service, slot),
				Address:   application.ManagedContainerName(service.GetServiceKey(), slot),
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
	if err := m.dataplane.ReplaceFrontendInTransaction(ctx, transactionID, frontend); err != nil {
		m.logDataplaneFailure(err, "dataplane replace frontend failed", failureContext)
		return err
	}
	existing, err := m.dataplane.ListBackends(ctx)
	if err != nil {
		return err
	}
	desiredBackends := map[string]struct{}{
		snapshot.GetDefaultBackend(): {},
	}
	for _, service := range snapshot.GetServices() {
		desiredBackends[serviceBackendName(service, grpcapi.Slot_SLOT_BLUE)] = struct{}{}
		desiredBackends[serviceBackendName(service, grpcapi.Slot_SLOT_GREEN)] = struct{}{}
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
	return nil
}

type dataplaneFailureContext struct {
	AgentID                string                   `json:"agentId"`
	TransactionID          string                   `json:"transactionId"`
	Version                string                   `json:"version"`
	Frontend               frontendSection          `json:"frontend"`
	ExpectedFrontendConfig string                   `json:"expectedFrontendConfig,omitempty"`
	DefaultBackend         string                   `json:"defaultBackend"`
	ServiceCount           int                      `json:"serviceCount"`
	Backends               []backendSection         `json:"backends,omitempty"`
	Servers                []dataplaneBackendServer `json:"servers,omitempty"`
	DesiredBackends        []string                 `json:"desiredBackends,omitempty"`
	StaleBackends          []string                 `json:"staleBackends,omitempty"`
}

type dataplaneBackendServer struct {
	Backend string        `json:"backend"`
	Server  backendServer `json:"server"`
}

func (m *ManagedProxyRuntime) logDataplaneFailure(err error, message string, failureContext dataplaneFailureContext) {
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

func formatDataplaneFailureContext(failureContext dataplaneFailureContext) string {
	encoded, err := json.Marshal(failureContext)
	if err != nil {
		return fmt.Sprintf(`{"marshalError":%q}`, err.Error())
	}
	return string(encoded)
}

func renderExpectedFrontendConfig(frontend frontendSection) string {
	lines := make([]string, 0, len(frontend.ACLList)+len(frontend.BackendSwitchingRuleList)+len(frontend.HTTPAfterResponseRules)+8)
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
	for _, rule := range frontend.HTTPAfterResponseRules {
		actionLine := fmt.Sprintf("  http-after-response %s %s %s", strings.TrimSpace(rule.Action), strings.TrimSpace(rule.Header), strings.TrimSpace(rule.Format))
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

func (m *ManagedProxyRuntime) frontendSection(snapshot *grpcapi.ProxyConfigSnapshot) frontendSection {
	services := append([]*grpcapi.ProxyServiceConfig(nil), snapshot.GetServices()...)
	sort.Slice(services, func(i, j int) bool {
		if services[i].GetRouteHost() != services[j].GetRouteHost() {
			return services[i].GetRouteHost() < services[j].GetRouteHost()
		}
		if len(services[i].GetRoutePathPrefix()) != len(services[j].GetRoutePathPrefix()) {
			return len(services[i].GetRoutePathPrefix()) > len(services[j].GetRoutePathPrefix())
		}
		return services[i].GetServiceKey() < services[j].GetServiceKey()
	})
	acls := make([]frontendACL, 0, len(services)*6)
	rules := make([]frontendSwitchRule, 0, len(services)*5)
	responseRules := make([]httpAfterResponseRule, 0, len(services)*3)
	for idx, service := range services {
		hostACL := aclName(service.GetServiceId(), "host")
		pathACL := aclName(service.GetServiceId(), "path")
		queryBlueACL := aclName(service.GetServiceId(), "query_blue")
		queryGreenACL := aclName(service.GetServiceId(), "query_green")
		cookieBlueACL := aclName(service.GetServiceId(), "cookie_blue")
		cookieGreenACL := aclName(service.GetServiceId(), "cookie_green")
		cookieName := servicecatalogapp.StickyCookieName(service.GetServiceKey())
		blueReleaseID := blueReleaseID(service)
		greenReleaseID := greenReleaseID(service)
		liveReleaseID := liveReleaseID(service)
		acls = append(acls, frontendACL{
			Name:      hostACL,
			Criterion: "hdr(host)",
			Value:     "-i " + service.GetRouteHost(),
			Index:     idx * 6,
		})
		acls = append(acls, frontendACL{
			Name:      pathACL,
			Criterion: "path_beg",
			Value:     service.GetRoutePathPrefix(),
			Index:     idx*6 + 1,
		})
		acls = append(acls,
			frontendACL{
				Name:      queryBlueACL,
				Criterion: "url_param(" + servicecatalogapp.PreviewReleaseIDQueryParam + ")",
				Value:     exactMatchValue(blueReleaseID),
				Index:     idx*6 + 2,
			},
			frontendACL{
				Name:      queryGreenACL,
				Criterion: "url_param(" + servicecatalogapp.PreviewReleaseIDQueryParam + ")",
				Value:     exactMatchValue(greenReleaseID),
				Index:     idx*6 + 3,
			},
			frontendACL{
				Name:      cookieBlueACL,
				Criterion: "cook(" + cookieName + ")",
				Value:     exactMatchValue(blueReleaseID),
				Index:     idx*6 + 4,
			},
			frontendACL{
				Name:      cookieGreenACL,
				Criterion: "cook(" + cookieName + ")",
				Value:     exactMatchValue(greenReleaseID),
				Index:     idx*6 + 5,
			},
		)
		ruleBase := idx * 5
		rules = append(rules, frontendSwitchRule{
			Name:     serviceBackendName(service, grpcapi.Slot_SLOT_BLUE),
			Cond:     "if",
			CondTest: hostACL + " " + pathACL + " " + queryBlueACL,
			Index:    ruleBase,
		})
		rules = append(rules, frontendSwitchRule{
			Name:     serviceBackendName(service, grpcapi.Slot_SLOT_GREEN),
			Cond:     "if",
			CondTest: hostACL + " " + pathACL + " " + queryGreenACL,
			Index:    ruleBase + 1,
		})
		rules = append(rules, frontendSwitchRule{
			Name:     serviceBackendName(service, grpcapi.Slot_SLOT_BLUE),
			Cond:     "if",
			CondTest: hostACL + " " + pathACL + " !" + queryBlueACL + " !" + queryGreenACL + " " + cookieBlueACL,
			Index:    ruleBase + 2,
		})
		rules = append(rules, frontendSwitchRule{
			Name:     serviceBackendName(service, grpcapi.Slot_SLOT_GREEN),
			Cond:     "if",
			CondTest: hostACL + " " + pathACL + " !" + queryBlueACL + " !" + queryGreenACL + " " + cookieGreenACL,
			Index:    ruleBase + 3,
		})
		rules = append(rules, frontendSwitchRule{
			Name:     serviceBackendName(service, liveSlot(service)),
			Cond:     "if",
			CondTest: hostACL + " " + pathACL + " !" + queryBlueACL + " !" + queryGreenACL + " !" + cookieBlueACL + " !" + cookieGreenACL,
			Index:    ruleBase + 4,
		})
		responseBase := idx * 3
		responseCondTest := hostACL + " " + pathACL
		responseRules = append(responseRules, httpAfterResponseRule{
			Action:   "add-header",
			Header:   "Set-Cookie",
			Format:   servicecatalogapp.BuildStickyCookie(cookieName, liveReleaseID, service.GetRoutePathPrefix()),
			Cond:     "if",
			CondTest: responseCondTest,
			Index:    responseBase,
		})
		responseRules = append(responseRules, httpAfterResponseRule{
			Action:   "set-header",
			Header:   servicecatalogapp.CurrentReleaseIDHeaderName,
			Format:   liveReleaseID,
			Cond:     "if",
			CondTest: responseCondTest,
			Index:    responseBase + 1,
		})
		responseRules = append(responseRules, httpAfterResponseRule{
			Action:   "set-header",
			Header:   servicecatalogapp.LiveReleaseIDHeaderName,
			Format:   liveReleaseID,
			Cond:     "if",
			CondTest: responseCondTest,
			Index:    responseBase + 2,
		})
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
		HTTPAfterResponseRules:   filterHTTPAfterResponseRules(responseRules),
	}
}

func (m *ManagedProxyRuntime) proxySpec() managedContainerSpec {
	//兼容远程docker
	haproxyApiListenAddr := env.GetOrDefault("HAPROXY_API_LISTEN_ADDR", "127.0.0.1")
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
		Exposed: map[string]map[string]string{
			portKey(servicecatalogapp.SharedFrontendBindPort): {},
			portKey(m.cfg.HAProxyRuntimePort):                 {},
			portKey(m.cfg.DataPlaneAPIPort):                   {},
		},
		PortBinds: map[string][]dockerPortBinding{
			portKey(servicecatalogapp.SharedFrontendBindPort): {
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(servicecatalogapp.SharedFrontendBindPort)},
			},
			portKey(m.cfg.HAProxyRuntimePort): {
				{HostIP: haproxyApiListenAddr, HostPort: strconv.Itoa(m.cfg.HAProxyRuntimePort)},
			},
			portKey(m.cfg.DataPlaneAPIPort): {
				{HostIP: haproxyApiListenAddr, HostPort: strconv.Itoa(m.cfg.DataPlaneAPIPort)},
			},
		},
		Network:   m.cfg.ProxyNetworkName,
		IPAddress: m.cfg.ProxyIPAddress,
		RestartPolicy: dockerRestartPolicy{
			Name:              "on-failure",
			MaximumRetryCount: 3,
		},
	}
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

defaults
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
`, m.cfg.HAProxyRuntimePort, m.cfg.DataPlaneAPIUsername, m.cfg.DataPlaneAPIPassword, managedProxyResolversName, servicecatalogapp.SharedFrontendName, servicecatalogapp.SharedFrontendBindPort, servicecatalogapp.SharedDefaultBackend, servicecatalogapp.SharedDefaultBackend)
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
			ServiceId:       item.GetServiceId(),
			ServiceKey:      item.GetServiceKey(),
			RouteHost:       item.GetRouteHost(),
			RoutePathPrefix: item.GetRoutePathPrefix(),
			BackendName:     item.GetBackendName(),
			BlueServerName:  item.GetBlueServerName(),
			GreenServerName: item.GetGreenServerName(),
			ContainerPort:   item.GetContainerPort(),
			CurrentLiveSlot: item.GetCurrentLiveSlot(),
		})
	}
	return out
}

func serviceBackendName(service *grpcapi.ProxyServiceConfig, slot grpcapi.Slot) string {
	return servicecatalogapp.BackendNameForSlot(service.GetBackendName(), model.Slot(slot))
}

func serviceServerName(service *grpcapi.ProxyServiceConfig, slot grpcapi.Slot) string {
	switch slot {
	case grpcapi.Slot_SLOT_BLUE:
		return servicecatalogapp.ServerName(model.SlotBlue)
	case grpcapi.Slot_SLOT_GREEN:
		return servicecatalogapp.ServerName(model.SlotGreen)
	default:
		return ""
	}
}

func blueReleaseID(service *grpcapi.ProxyServiceConfig) string {
	return strings.TrimSpace(service.GetBlueServerName())
}

func greenReleaseID(service *grpcapi.ProxyServiceConfig) string {
	return strings.TrimSpace(service.GetGreenServerName())
}

func liveSlot(service *grpcapi.ProxyServiceConfig) grpcapi.Slot {
	switch service.GetCurrentLiveSlot() {
	case grpcapi.Slot_SLOT_BLUE, grpcapi.Slot_SLOT_GREEN:
		return service.GetCurrentLiveSlot()
	default:
		return grpcapi.Slot_SLOT_BLUE
	}
}

func liveReleaseID(service *grpcapi.ProxyServiceConfig) string {
	switch liveSlot(service) {
	case grpcapi.Slot_SLOT_BLUE:
		return blueReleaseID(service)
	case grpcapi.Slot_SLOT_GREEN:
		return greenReleaseID(service)
	default:
		return ""
	}
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

var _ application.ProxyRuntime = (*ManagedProxyRuntime)(nil)
var _ managedProxyRuntimeAPI = (*HAProxyRuntimeClient)(nil)
var _ managedProxyDataPlaneAPI = (*DataPlaneAPIClient)(nil)

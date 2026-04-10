package infra

import (
	"context"
	"edge-pilot/internal/agent/application"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/real-uangi/allingo/common/log"
	"go.uber.org/fx"
	"google.golang.org/protobuf/proto"
)

var containerIDPattern = regexp.MustCompile(`[0-9a-f]{12,64}`)

type ManagedProxyRuntime struct {
	cfg       *config.AgentRuntimeConfig
	docker    *DockerClient
	runtime   *HAProxyRuntimeClient
	dataplane *DataPlaneAPIClient
	logger    *log.StdLogger

	mu                 sync.Mutex
	desired            *grpcapi.ProxyConfigSnapshot
	desiredHash        string
	appliedHash        string
	ready              bool
	attachedToNetwork  bool
	selfContainerID    string
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
				runtime.logger.Errorf(err, "docker socket is not accessible: agentId=%s socket=%s", runtime.cfg.AgentID, runtime.cfg.DockerSocketPath)
				return err
			}
			runtime.logger.Infof("docker socket is accessible: agentId=%s socket=%s", runtime.cfg.AgentID, runtime.cfg.DockerSocketPath)
			go runtime.runSelfHeal(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
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
			changed, err := m.ensureProxyStackLocked(ctx)
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
	return m.ensureReadyLocked(ctx)
}

func (m *ManagedProxyRuntime) EnsureReady(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureReadyLocked(ctx)
}

func (m *ManagedProxyRuntime) ShowStats(ctx context.Context) ([]*grpcapi.BackendStatPoint, error) {
	m.mu.Lock()
	ready := m.ready
	lastErr := m.lastApplyErrorText
	m.mu.Unlock()
	if !ready {
		if strings.TrimSpace(lastErr) == "" {
			lastErr = "proxy stack is still bootstrapping"
		}
		return nil, fmt.Errorf("%w: %s", application.ErrProxyNotReady, lastErr)
	}
	return m.runtime.ShowStats(ctx)
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
	if _, err := m.ensureProxyStackLocked(ctx); err != nil {
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

func (m *ManagedProxyRuntime) ensureProxyStackLocked(ctx context.Context) (bool, error) {
	if !m.ready {
		m.logger.Infof("ensuring managed proxy stack: container=%s network=%s", m.cfg.ProxyContainerName, m.cfg.ProxyNetworkName)
	}
	changed := false
	if err := m.docker.ensureNetwork(ctx, m.cfg.ProxyNetworkName, m.cfg.ProxyNetworkSubnet); err != nil {
		return false, err
	}
	if err := m.ensureReservedProxyIPLocked(ctx); err != nil {
		return false, err
	}

	proxyInspect, err := m.docker.inspectManagedContainer(ctx, m.cfg.ProxyContainerName)
	if err != nil {
		return false, err
	}
	if proxyInspect == nil {
		if err := m.docker.recreateVolume(ctx, m.cfg.HAProxyConfigVolume); err != nil {
			return false, err
		}
		if err := m.bootstrapBaseFiles(ctx); err != nil {
			return false, err
		}
		changed = true
	} else {
		if err := m.docker.ensureVolume(ctx, m.cfg.HAProxyConfigVolume); err != nil {
			return false, err
		}
	}
	if proxyInspect != nil && !proxyInspect.State.Running {
		if err := m.bootstrapBaseFiles(ctx); err != nil {
			return false, err
		}
		changed = true
	}

	containerChanged, err := m.docker.ensureManagedContainer(ctx, m.proxySpec())
	if err != nil {
		return false, err
	}
	if containerChanged {
		changed = true
	}
	if err := m.ensureSelfConnectedLocked(ctx); err != nil {
		return false, err
	}
	if err := retry(ctx, 12, time.Second, func() error {
		_, err := m.runtime.run(ctx, "show info")
		return err
	}); err != nil {
		return false, err
	}
	if err := retry(ctx, 12, time.Second, func() error {
		_, err := m.dataplane.ConfigurationVersion(ctx)
		return err
	}); err != nil {
		return false, err
	}
	return changed, nil
}

func (m *ManagedProxyRuntime) bootstrapBaseFiles(ctx context.Context) error {
	files := map[string]string{
		"haproxy.cfg":      m.baseHAProxyConfig(),
		"dataplaneapi.yml": m.dataPlaneConfig(),
	}
	m.logger.Infof("bootstrapping proxy config files: container=%s", m.cfg.ProxyContainerName)
	return m.docker.writeVolumeFiles(ctx, m.cfg.ProxyHelperImage, m.cfg.HAProxyConfigVolume, files)
}

func (m *ManagedProxyRuntime) reconcileLocked(ctx context.Context, snapshot *grpcapi.ProxyConfigSnapshot) error {
	m.logger.Infof("reconciling proxy snapshot: agentId=%s services=%d", m.cfg.AgentID, len(snapshot.GetServices()))
	if err := m.dataplane.ReplaceFrontend(ctx, m.frontendSection(snapshot)); err != nil {
		return err
	}
	for _, service := range snapshot.GetServices() {
		backend := backendSection{
			Name: service.GetBackendName(),
			Mode: "http",
			Balance: backendBalance{
				Algorithm: "roundrobin",
			},
		}
		if err := m.dataplane.EnsureBackend(ctx, backend); err != nil {
			return err
		}
		if err := m.dataplane.EnsureServer(ctx, service.GetBackendName(), backendServer{
			Name:    service.GetBlueServerName(),
			Address: application.ManagedContainerName(service.GetServiceKey(), grpcapi.Slot_SLOT_BLUE),
			Port:    int(service.GetContainerPort()),
			Check:   "enabled",
		}); err != nil {
			return err
		}
		if err := m.dataplane.EnsureServer(ctx, service.GetBackendName(), backendServer{
			Name:    service.GetGreenServerName(),
			Address: application.ManagedContainerName(service.GetServiceKey(), grpcapi.Slot_SLOT_GREEN),
			Port:    int(service.GetContainerPort()),
			Check:   "enabled",
		}); err != nil {
			return err
		}
	}
	existing, err := m.dataplane.ListBackends(ctx)
	if err != nil {
		return err
	}
	desiredBackends := map[string]struct{}{
		snapshot.GetDefaultBackend(): {},
	}
	for _, service := range snapshot.GetServices() {
		desiredBackends[service.GetBackendName()] = struct{}{}
	}
	for _, name := range existing {
		if _, ok := desiredBackends[name]; ok {
			continue
		}
		m.logger.Infof("removing stale backend from dataplane: backend=%s", name)
		if err := m.dataplane.DeleteBackend(ctx, name); err != nil {
			return err
		}
	}
	for _, service := range snapshot.GetServices() {
		if err := m.applyLiveSlot(ctx, service); err != nil {
			return err
		}
	}
	return nil
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
	acls := make([]frontendACL, 0, len(services)*2)
	rules := make([]frontendSwitchRule, 0, len(services))
	for idx, service := range services {
		hostACL := aclName(service.GetServiceId(), "host")
		pathACL := aclName(service.GetServiceId(), "path")
		acls = append(acls, frontendACL{
			Name:      hostACL,
			Criterion: "hdr(host)",
			Value:     "-i " + service.GetRouteHost(),
			Index:     idx * 2,
		})
		acls = append(acls, frontendACL{
			Name:      pathACL,
			Criterion: "path_beg",
			Value:     service.GetRoutePathPrefix(),
			Index:     idx*2 + 1,
		})
		rules = append(rules, frontendSwitchRule{
			Name:     service.GetBackendName(),
			Cond:     "if",
			CondTest: hostACL + " " + pathACL,
			Index:    idx,
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
	}
}

func (m *ManagedProxyRuntime) applyLiveSlot(ctx context.Context, service *grpcapi.ProxyServiceConfig) error {
	switch service.GetCurrentLiveSlot() {
	case grpcapi.Slot_SLOT_BLUE:
		if err := m.runtime.EnableServer(ctx, service.GetBackendName(), service.GetBlueServerName()); err != nil {
			return err
		}
		return m.runtime.DisableServer(ctx, service.GetBackendName(), service.GetGreenServerName())
	case grpcapi.Slot_SLOT_GREEN:
		if err := m.runtime.EnableServer(ctx, service.GetBackendName(), service.GetGreenServerName()); err != nil {
			return err
		}
		return m.runtime.DisableServer(ctx, service.GetBackendName(), service.GetBlueServerName())
	default:
		if err := m.runtime.DisableServer(ctx, service.GetBackendName(), service.GetBlueServerName()); err != nil {
			return err
		}
		return m.runtime.DisableServer(ctx, service.GetBackendName(), service.GetGreenServerName())
	}
}

func (m *ManagedProxyRuntime) proxySpec() managedContainerSpec {
	return managedContainerSpec{
		Name:  m.cfg.ProxyContainerName,
		Image: m.cfg.HAProxyImage,
		Labels: map[string]string{
			proxyStackLabelKey:     "true",
			proxyStackRoleLabelKey: "proxy",
			proxyStackAgentLabel:   m.cfg.AgentID,
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
				{HostIP: "127.0.0.1", HostPort: strconv.Itoa(m.cfg.HAProxyRuntimePort)},
			},
			portKey(m.cfg.DataPlaneAPIPort): {
				{HostIP: "127.0.0.1", HostPort: strconv.Itoa(m.cfg.DataPlaneAPIPort)},
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
	host := "127.0.0.1"
	if m.attachedToNetwork {
		host = m.cfg.ProxyIPAddress
	}
	return net.JoinHostPort(host, strconv.Itoa(m.cfg.HAProxyRuntimePort))
}

func (m *ManagedProxyRuntime) dataplaneBaseURL() string {
	host := "127.0.0.1"
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

func (m *ManagedProxyRuntime) baseHAProxyConfig() string {
	return fmt.Sprintf(`global
  log stdout format raw local0
  master-worker
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

frontend %s
  bind *:%d
  mode http
  default_backend %s

backend %s
  mode http
  http-request return status 503 content-type text/plain string no-route
`, m.cfg.HAProxyRuntimePort, m.cfg.DataPlaneAPIUsername, m.cfg.DataPlaneAPIPassword, servicecatalogapp.SharedFrontendName, servicecatalogapp.SharedFrontendBindPort, servicecatalogapp.SharedDefaultBackend, servicecatalogapp.SharedDefaultBackend)
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

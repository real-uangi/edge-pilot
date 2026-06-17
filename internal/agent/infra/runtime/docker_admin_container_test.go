package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/real-uangi/allingo/common/log"
)

func TestEnsureManagedContainerRecreateRemovesOldImage(t *testing.T) {
	spec := testManagedContainerSpec()
	initial := buildManagedContainerInspect(spec, "proxy-old", true, spec.Network, "172.29.0.250")
	daemon := newFakeManagedContainerDaemon(t, spec, initial, false)
	client := daemon.newClient()

	_, err := client.ensureManagedContainer(context.Background(), spec)
	if err != nil {
		t.Fatalf("expected ensureManagedContainer success, got %v", err)
	}
	if daemon.removeCount != 1 {
		t.Fatalf("expected one remove call, got %d", daemon.removeCount)
	}
	if daemon.removeImageCount != 1 {
		t.Fatalf("expected one image remove call, got %d", daemon.removeImageCount)
	}
	if len(daemon.removeImageRefs) != 1 || daemon.removeImageRefs[0] != spec.Image {
		t.Fatalf("expected old image %s to be removed, got %#v", spec.Image, daemon.removeImageRefs)
	}
}

func TestManagedContainerNetworkDriftReason(t *testing.T) {
	spec := testManagedContainerSpec()

	matched := buildManagedContainerInspect(spec, "proxy-1", true, spec.Network, spec.IPAddress)
	if reason := managedContainerNetworkDriftReason(matched, spec); reason != "" {
		t.Fatalf("expected no network drift, got %q", reason)
	}

	missingNetwork := buildManagedContainerInspect(spec, "proxy-2", true, "bridge", "172.17.0.2")
	if reason := managedContainerNetworkDriftReason(missingNetwork, spec); !strings.Contains(reason, "not attached") {
		t.Fatalf("expected missing network reason, got %q", reason)
	}

	ipMismatch := buildManagedContainerInspect(spec, "proxy-3", true, spec.Network, "172.29.0.250")
	if reason := managedContainerNetworkDriftReason(ipMismatch, spec); !strings.Contains(reason, "ip mismatch") {
		t.Fatalf("expected ip mismatch reason, got %q", reason)
	}
}

func TestEnsureManagedContainerRecreateWhenNetworkDrifts(t *testing.T) {
	spec := testManagedContainerSpec()
	initial := buildManagedContainerInspect(spec, "proxy-old", true, spec.Network, "172.29.0.250")
	daemon := newFakeManagedContainerDaemon(t, spec, initial, false)
	client := daemon.newClient()

	changed, err := client.ensureManagedContainer(context.Background(), spec)
	if err != nil {
		t.Fatalf("expected ensureManagedContainer success, got %v", err)
	}
	if !changed {
		t.Fatal("expected ensureManagedContainer report changed=true")
	}
	if daemon.connectCount != 0 {
		t.Fatalf("expected no network connect call for ip mismatch drift, got %d", daemon.connectCount)
	}
	if daemon.createCount != 1 {
		t.Fatalf("expected one recreate call, got %d", daemon.createCount)
	}
	if daemon.removeCount != 1 {
		t.Fatalf("expected one remove call, got %d", daemon.removeCount)
	}
	if len(daemon.createRequests) != 1 {
		t.Fatalf("expected one create request payload, got %d", len(daemon.createRequests))
	}
	if daemon.createRequests[0].HostConfig.NetworkMode != spec.Network {
		t.Fatalf("expected recreate request network mode %s, got %q", spec.Network, daemon.createRequests[0].HostConfig.NetworkMode)
	}
}

func TestEnsureManagedContainerRepairsMissingNetworkInPlace(t *testing.T) {
	spec := testManagedContainerSpec()
	initial := buildManagedContainerInspect(spec, "proxy-old", true, "bridge", "172.17.0.2")
	daemon := newFakeManagedContainerDaemon(t, spec, initial, false)
	client := daemon.newClient()

	changed, err := client.ensureManagedContainer(context.Background(), spec)
	if err != nil {
		t.Fatalf("expected ensureManagedContainer success, got %v", err)
	}
	if !changed {
		t.Fatal("expected ensureManagedContainer report changed=true")
	}
	if daemon.connectCount != 1 {
		t.Fatalf("expected one network connect call, got %d", daemon.connectCount)
	}
	if daemon.connectIPs[0] != spec.IPAddress {
		t.Fatalf("expected network connect use ip %s, got %q", spec.IPAddress, daemon.connectIPs[0])
	}
	if daemon.createCount != 0 {
		t.Fatalf("expected no recreate call, got %d", daemon.createCount)
	}
	if daemon.removeCount != 0 {
		t.Fatalf("expected no remove call, got %d", daemon.removeCount)
	}
}

func TestEnsureManagedContainerKeepsHealthyContainer(t *testing.T) {
	spec := testManagedContainerSpec()
	initial := buildManagedContainerInspect(spec, "proxy-healthy", true, spec.Network, spec.IPAddress)
	daemon := newFakeManagedContainerDaemon(t, spec, initial, false)
	client := daemon.newClient()

	changed, err := client.ensureManagedContainer(context.Background(), spec)
	if err != nil {
		t.Fatalf("expected ensureManagedContainer success, got %v", err)
	}
	if changed {
		t.Fatal("expected healthy container to stay unchanged")
	}
	if daemon.createCount != 0 {
		t.Fatalf("expected no recreate call, got %d", daemon.createCount)
	}
	if daemon.removeCount != 0 {
		t.Fatalf("expected no remove call, got %d", daemon.removeCount)
	}
}

func TestEnsureManagedContainerReturnsErrorWhenRecreatedContainerStillInvalid(t *testing.T) {
	spec := testManagedContainerSpec()
	initial := buildManagedContainerInspect(spec, "proxy-old", true, spec.Network, "172.29.0.250")
	daemon := newFakeManagedContainerDaemon(t, spec, initial, true)
	client := daemon.newClient()

	changed, err := client.ensureManagedContainer(context.Background(), spec)
	if !changed {
		t.Fatal("expected ensureManagedContainer to report changed=true")
	}
	if err == nil {
		t.Fatal("expected ensureManagedContainer to fail when recreated container is still invalid")
	}
	if !strings.Contains(err.Error(), "validation failed") && !strings.Contains(err.Error(), "ip mismatch") {
		t.Fatalf("expected validation failure error, got %v", err)
	}
}

type fakeManagedContainerDaemon struct {
	t                *testing.T
	spec             managedContainerSpec
	mu               sync.Mutex
	current          *dockerContainerInspect
	recreateInvalid  bool
	createCount      int
	removeCount      int
	removeImageCount int
	removeImageRefs  []string
	connectCount     int
	connectIPs       []string
	createRequests   []dockerCreateContainerRequest
	server           *httptest.Server
}

func newFakeManagedContainerDaemon(t *testing.T, spec managedContainerSpec, initial *dockerContainerInspect, recreateInvalid bool) *fakeManagedContainerDaemon {
	t.Helper()
	daemon := &fakeManagedContainerDaemon{
		t:               t,
		spec:            spec,
		current:         initial,
		recreateInvalid: recreateInvalid,
	}
	daemon.server = httptest.NewServer(http.HandlerFunc(daemon.handle))
	t.Cleanup(daemon.server.Close)
	return daemon
}

func (d *fakeManagedContainerDaemon) newClient() *DockerClient {
	return &DockerClient{
		httpClient: d.server.Client(),
		endpoint: &dockerEndpoint{
			raw:     d.server.URL,
			scheme:  "http",
			baseURL: d.server.URL,
		},
		logger: log.NewStdLogger("agent.docker.test"),
	}
}

func (d *fakeManagedContainerDaemon) handle(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/images/") && strings.HasSuffix(r.URL.Path, "/json"):
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
		return

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/containers/") && strings.HasSuffix(r.URL.Path, "/json"):
		idOrName := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/containers/"), "/json")
		if d.current == nil {
			http.NotFound(w, r)
			return
		}
		if idOrName != d.spec.Name && idOrName != d.current.ID {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(d.current); err != nil {
			d.t.Fatalf("encode inspect response failed: %v", err)
		}
		return

	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/images/"):
		id := strings.TrimPrefix(r.URL.Path, "/images/")
		if idx := strings.IndexByte(id, '?'); idx >= 0 {
			id = id[:idx]
		}
		d.removeImageCount++
		d.removeImageRefs = append(d.removeImageRefs, id)
		w.WriteHeader(http.StatusOK)
		return

	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/containers/"):
		id := strings.TrimPrefix(r.URL.Path, "/containers/")
		if idx := strings.IndexByte(id, '?'); idx >= 0 {
			id = id[:idx]
		}
		if d.current != nil && d.current.ID == id {
			d.current = nil
			d.removeCount++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
		return

	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/networks/") && strings.HasSuffix(r.URL.Path, "/connect"):
		var req dockerNetworkConnectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			d.t.Fatalf("decode connect request failed: %v", err)
		}
		if d.current == nil || req.Container != d.current.ID {
			http.NotFound(w, r)
			return
		}
		networkName := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/networks/"), "/connect")
		networkName = strings.TrimSpace(networkName)
		ip := ""
		if req.EndpointConfig != nil && req.EndpointConfig.IPAMConfig != nil {
			ip = strings.TrimSpace(req.EndpointConfig.IPAMConfig.IPv4Address)
		}
		d.current.NetworkSettings.Networks[networkName] = struct {
			IPAddress string `json:"IPAddress"`
		}{IPAddress: ip}
		d.connectCount++
		d.connectIPs = append(d.connectIPs, ip)
		w.WriteHeader(http.StatusOK)
		return

	case r.Method == http.MethodPost && r.URL.Path == "/containers/create":
		var req dockerCreateContainerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			d.t.Fatalf("decode create request failed: %v", err)
		}
		d.createRequests = append(d.createRequests, req)
		d.createCount++
		newID := fmt.Sprintf("proxy-new-%d", d.createCount)
		d.current = buildManagedContainerInspect(d.spec, newID, false, d.spec.Network, d.spec.IPAddress)
		if d.recreateInvalid {
			d.current = buildManagedContainerInspect(d.spec, newID, false, d.spec.Network, "172.29.0.250")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(dockerCreateResponse{ID: newID})
		return

	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/containers/") && strings.HasSuffix(r.URL.Path, "/start"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/containers/"), "/start")
		if d.current == nil || d.current.ID != id {
			http.NotFound(w, r)
			return
		}
		d.current.State.Running = true
		w.WriteHeader(http.StatusNoContent)
		return
	}

	d.t.Fatalf("unexpected docker api call: method=%s path=%s rawQuery=%s", r.Method, r.URL.Path, r.URL.RawQuery)
}

func testManagedContainerSpec() managedContainerSpec {
	return managedContainerSpec{
		Name:      "edge-pilot-haproxy",
		Image:     "haproxytech/haproxy-debian:s6-3.3",
		Network:   "epNet",
		IPAddress: "172.29.0.233",
		Labels: map[string]string{
			proxyStackLabelKey:     "true",
			proxyStackRoleLabelKey: "proxy",
		},
		RestartPolicy: dockerRestartPolicy{
			Name:              "on-failure",
			MaximumRetryCount: 3,
		},
	}
}

func buildManagedContainerInspect(spec managedContainerSpec, id string, running bool, network string, ip string) *dockerContainerInspect {
	inspect := &dockerContainerInspect{
		ID: id,
	}
	inspect.Config.Image = spec.Image
	inspect.Config.Labels = map[string]string{
		proxyStackLabelKey:     "true",
		proxyStackRoleLabelKey: spec.Labels[proxyStackRoleLabelKey],
		proxyStackSpecLabelKey: specHash(spec),
	}
	inspect.State.Running = running
	inspect.NetworkSettings.Networks = map[string]struct {
		IPAddress string `json:"IPAddress"`
	}{}
	if strings.TrimSpace(network) != "" {
		inspect.NetworkSettings.Networks[network] = struct {
			IPAddress string `json:"IPAddress"`
		}{IPAddress: ip}
	}
	return inspect
}

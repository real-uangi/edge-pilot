package infra

import (
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"testing"
)

func TestProxySpecUsesLimitedRestartPolicy(t *testing.T) {
	runtime := &ManagedProxyRuntime{
		cfg: &config.AgentRuntimeConfig{
			AgentID:            "81ad661e-cf19-4bab-afa4-9d00826774c2",
			HAProxyImage:       "haproxytech/haproxy-debian:s6-3.4",
			ProxyContainerName: "edge-pilot-haproxy",
			ProxyNetworkName:   "epNet",
			ProxyIPAddress:     "172.29.0.233",
			HAProxyRuntimePort: 19999,
			DataPlaneAPIPort:   5555,
		},
	}

	spec := runtime.proxySpec()
	if spec.RestartPolicy.Name != "on-failure" {
		t.Fatalf("expected proxy restart policy on-failure, got %q", spec.RestartPolicy.Name)
	}
	if spec.RestartPolicy.MaximumRetryCount != 3 {
		t.Fatalf("expected proxy max retries 3, got %d", spec.RestartPolicy.MaximumRetryCount)
	}
}

func TestSpecHashIncludesRestartPolicy(t *testing.T) {
	base := managedContainerSpec{
		Name:      "edge-pilot-haproxy",
		Image:     "haproxytech/haproxy-debian:s6-3.4",
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
	changed := base
	changed.RestartPolicy.MaximumRetryCount = 5

	if specHash(base) == specHash(changed) {
		t.Fatal("expected restart policy change to affect managed container spec hash")
	}
}

func TestSnapshotHashStableForSameSnapshot(t *testing.T) {
	snapshot := &grpcapi.ProxyConfigSnapshot{
		AgentId:        "81ad661e-cf19-4bab-afa4-9d00826774c2",
		FrontendName:   "ep_http",
		BindPort:       80,
		DefaultBackend: "ep_default",
	}
	if snapshotHash(snapshot) == "" {
		t.Fatal("expected snapshot hash to be generated")
	}
	if snapshotHash(snapshot) != snapshotHash(snapshot) {
		t.Fatal("expected same snapshot to produce stable hash")
	}
}

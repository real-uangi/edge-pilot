package infra

import (
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"testing"
)

func TestBuildWorkloadCreateRequestUsesLimitedRestartPolicy(t *testing.T) {
	cfg := &config.AgentRuntimeConfig{ProxyNetworkName: "epNet"}
	task := &grpcapi.TaskCommand{
		AgentId:       "81ad661e-cf19-4bab-afa4-9d00826774c2",
		ServiceId:     "svc-1",
		ServiceKey:    "demo",
		ReleaseId:     "rel-1",
		TargetSlot:    grpcapi.Slot_SLOT_BLUE,
		ContainerPort: 8080,
		PublishedPorts: []*grpcapi.PublishedPort{
			{HostPort: 18080, ContainerPort: 8080},
		},
	}

	req := buildWorkloadCreateRequest(cfg, "repo/demo:v1", task)
	if req.HostConfig.RestartPolicy.Name != "on-failure" {
		t.Fatalf("expected workload restart policy on-failure, got %q", req.HostConfig.RestartPolicy.Name)
	}
	if req.HostConfig.RestartPolicy.MaximumRetryCount != 5 {
		t.Fatalf("expected workload max retries 5, got %d", req.HostConfig.RestartPolicy.MaximumRetryCount)
	}
	if _, ok := req.NetworkingConfig.EndpointsConfig["epNet"]; !ok {
		t.Fatal("expected workload to attach to proxy network")
	}
}

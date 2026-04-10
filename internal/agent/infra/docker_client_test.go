package infra

import (
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"encoding/base64"
	"encoding/json"
	"strings"
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

func TestBuildRegistryAuthHeader(t *testing.T) {
	header, ok, err := buildRegistryAuthHeader(taskRegistryAuth{
		host:     "ghcr.io",
		username: "octocat",
		secret:   "token-value",
	})
	if err != nil {
		t.Fatalf("buildRegistryAuthHeader() error = %v", err)
	}
	if !ok {
		t.Fatal("expected registry auth header to be present")
	}
	payload, err := base64.URLEncoding.DecodeString(header)
	if err != nil {
		t.Fatalf("expected base64 payload, got %v", err)
	}
	var auth map[string]string
	if err := json.Unmarshal(payload, &auth); err != nil {
		t.Fatalf("expected json payload, got %v", err)
	}
	if auth["serveraddress"] != "ghcr.io" || auth["username"] != "octocat" || auth["password"] != "token-value" {
		t.Fatalf("unexpected auth payload: %#v", auth)
	}
}

func TestBuildRegistryAuthHeaderAllowsAnonymousPull(t *testing.T) {
	header, ok, err := buildRegistryAuthHeader(taskRegistryAuth{})
	if err != nil {
		t.Fatalf("buildRegistryAuthHeader() error = %v", err)
	}
	if ok || header != "" {
		t.Fatalf("expected anonymous pull path, got header=%q ok=%v", header, ok)
	}
}

func TestConsumeDockerPullStreamSuccess(t *testing.T) {
	stream := strings.NewReader(`{"status":"Pulling from library/busybox"}
{"status":"Digest: sha256:abc"}
{"status":"Status: Downloaded newer image"}
`)
	if err := consumeDockerPullStream(stream); err != nil {
		t.Fatalf("consumeDockerPullStream() error = %v", err)
	}
}

func TestConsumeDockerPullStreamReturnsStreamError(t *testing.T) {
	stream := strings.NewReader(`{"status":"Pulling from library/busybox"}
{"errorDetail":{"message":"pull access denied"}}
`)
	err := consumeDockerPullStream(stream)
	if err == nil {
		t.Fatal("expected consumeDockerPullStream to fail")
	}
	if !strings.Contains(err.Error(), "pull access denied") {
		t.Fatalf("unexpected error: %v", err)
	}
}

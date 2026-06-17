package runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
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
		CpuLimitCores: 0.5,
		MemoryLimitMb: 256,
		PublishedPorts: []*grpcapi.PublishedPort{
			{HostPort: 18080, ContainerPort: 8080},
		},
		NetworkAliases: []string{"svc-a", "svc-a-canary"},
	}

	req := buildWorkloadCreateRequest(cfg, "repo/demo:v1", task)
	if req.HostConfig.RestartPolicy.Name != "on-failure" {
		t.Fatalf("expected workload restart policy on-failure, got %q", req.HostConfig.RestartPolicy.Name)
	}
	if req.HostConfig.RestartPolicy.MaximumRetryCount != 5 {
		t.Fatalf("expected workload max retries 5, got %d", req.HostConfig.RestartPolicy.MaximumRetryCount)
	}
	if req.HostConfig.NetworkMode != "epNet" {
		t.Fatalf("expected workload network mode epNet, got %q", req.HostConfig.NetworkMode)
	}
	if req.HostConfig.NanoCpus != 500000000 {
		t.Fatalf("expected workload nano cpus 500000000, got %d", req.HostConfig.NanoCpus)
	}
	if req.HostConfig.Memory != 268435456 {
		t.Fatalf("expected workload memory bytes 268435456, got %d", req.HostConfig.Memory)
	}
	if _, ok := req.NetworkingConfig.EndpointsConfig["epNet"]; !ok {
		t.Fatal("expected workload to attach to proxy network")
	}
	if got := req.NetworkingConfig.EndpointsConfig["epNet"].Aliases; len(got) != 2 || got[0] != "svc-a" || got[1] != "svc-a-canary" {
		t.Fatalf("expected network aliases in endpoint settings, got %#v", got)
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

func TestNanoCPUsToCores(t *testing.T) {
	if got := nanoCPUsToCores(0); got != 0 {
		t.Fatalf("expected 0 cores for 0 nanoCPUs, got %f", got)
	}
	if got := nanoCPUsToCores(500000000); got != 0.5 {
		t.Fatalf("expected 0.5 cores for 500000000 nanoCPUs, got %f", got)
	}
	if got := nanoCPUsToCores(1000000000); got != 1.0 {
		t.Fatalf("expected 1.0 cores for 1000000000 nanoCPUs, got %f", got)
	}
	if got := nanoCPUsToCores(2500000000); got != 2.5 {
		t.Fatalf("expected 2.5 cores for 2500000000 nanoCPUs, got %f", got)
	}
}

func TestParseEnv(t *testing.T) {
	input := []string{"FOO=value", "EMPTY=", "NO_EQUALS", "=leading", "NORMAL=foo=bar", "PASSWORD=secret", "API_KEY=token"}
	got := parseEnv(input)
	if len(got) != 5 {
		t.Fatalf("expected 5 parsed env vars, got %d", len(got))
	}
	if got["FOO"] != "value" {
		t.Fatalf("expected FOO=value, got FOO=%q", got["FOO"])
	}
	if got["EMPTY"] != "" {
		t.Fatalf("expected EMPTY=, got EMPTY=%q", got["EMPTY"])
	}
	if got["NORMAL"] != "foo=bar" {
		t.Fatalf("expected NORMAL=foo=bar, got NORMAL=%q", got["NORMAL"])
	}
	if got["PASSWORD"] != "[REDACTED]" {
		t.Fatalf("expected PASSWORD=[REDACTED], got PASSWORD=%q", got["PASSWORD"])
	}
	if got["API_KEY"] != "[REDACTED]" {
		t.Fatalf("expected API_KEY=[REDACTED], got API_KEY=%q", got["API_KEY"])
	}
	if _, ok := got["NO_EQUALS"]; ok {
		t.Fatal("expected NO_EQUALS to be skipped")
	}
	if _, ok := got[""]; ok {
		t.Fatal("expected empty key to be skipped")
	}
}

func TestParseVolumeMounts(t *testing.T) {
	tests := []struct {
		bind    string
		wantRO  bool
		wantSrc string
		wantTgt string
	}{
		{"/host:/container", false, "/host", "/container"},
		{"/host:/container:ro", true, "/host", "/container"},
		{"/host:/container:rw", false, "/host", "/container"},
		{"/host:/container:rw,ro", true, "/host", "/container"},
		{"/host:/container:proxy", false, "/host", "/container"},
		{"/host:/container:broken", false, "/host", "/container"},
		{"/host:/container:rw,ro,nocopy", true, "/host", "/container"},
	}
	for _, tt := range tests {
		got := parseVolumeMounts([]string{tt.bind})
		if len(got) != 1 {
			t.Errorf("bind=%q: expected 1 mount, got %d", tt.bind, len(got))
			continue
		}
		if got[0].Source != tt.wantSrc {
			t.Errorf("bind=%q: expected source=%q, got %q", tt.bind, tt.wantSrc, got[0].Source)
		}
		if got[0].Target != tt.wantTgt {
			t.Errorf("bind=%q: expected target=%q, got %q", tt.bind, tt.wantTgt, got[0].Target)
		}
		if got[0].ReadOnly != tt.wantRO {
			t.Errorf("bind=%q: expected readOnly=%v, got %v", tt.bind, tt.wantRO, got[0].ReadOnly)
		}
	}
	// Edge cases
	if got := parseVolumeMounts([]string{""}); len(got) != 0 {
		t.Fatalf("expected 0 mounts for empty bind, got %d", len(got))
	}
	if got := parseVolumeMounts([]string{"nocolon"}); len(got) != 0 {
		t.Fatalf("expected 0 mounts for single-element bind, got %d", len(got))
	}
}

func TestParsePublishedPorts(t *testing.T) {
	input := map[string][]dockerPortBinding{
		"8080/tcp": {{HostIP: "0.0.0.0", HostPort: "18080"}},
		"443/tcp":  {{HostIP: "0.0.0.0", HostPort: "8443"}},
	}
	got := parsePublishedPorts(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(got))
	}
	found := make(map[int32]int32)
	for _, p := range got {
		found[p.ContainerPort] = p.HostPort
	}
	if found[8080] != 18080 {
		t.Errorf("expected container 8080 -> host 18080, got %d", found[8080])
	}
	if found[443] != 8443 {
		t.Errorf("expected container 443 -> host 8443, got %d", found[443])
	}

	// multiple host bindings for same container port
	input2 := map[string][]dockerPortBinding{
		"8080/tcp": {{HostIP: "0.0.0.0", HostPort: "18080"}, {HostIP: "0.0.0.0", HostPort: "18081"}},
	}
	got2 := parsePublishedPorts(input2)
	if len(got2) != 2 {
		t.Fatalf("expected 2 ports for multiple bindings, got %d", len(got2))
	}

	// empty input map
	got3 := parsePublishedPorts(map[string][]dockerPortBinding{})
	if len(got3) != 0 {
		t.Fatalf("expected 0 ports for empty input, got %d", len(got3))
	}

	// UDP protocol key
	input4 := map[string][]dockerPortBinding{
		"53/udp": {{HostIP: "0.0.0.0", HostPort: "53"}},
	}
	got4 := parsePublishedPorts(input4)
	if len(got4) != 1 || got4[0].ContainerPort != 53 {
		t.Fatalf("expected container port 53 for UDP, got %v", got4)
	}
}

func TestParsePublishedPortsSkipsInvalid(t *testing.T) {
	input := map[string][]dockerPortBinding{
		"8080/tcp":    {{HostIP: "0.0.0.0", HostPort: "18080"}},
		"invalid/tcp": {{HostIP: "0.0.0.0", HostPort: "12345"}},
		"9090/tcp":    {{HostIP: "0.0.0.0", HostPort: "not_a_port"}},
	}
	got := parsePublishedPorts(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 valid port, got %d", len(got))
	}
	if got[0].ContainerPort != 8080 || got[0].HostPort != 18080 {
		t.Fatalf("expected container 8080 -> host 18080, got %d -> %d", got[0].ContainerPort, got[0].HostPort)
	}
}

func TestGetContainerDetails(t *testing.T) {
	fullJSON := `{
		"Id": "abc123",
		"Name": "/test-container",
		"Created": "2024-01-01T00:00:00Z",
		"RestartCount": 2,
		"Config": {
			"Image": "nginx:latest",
			"Labels": {"app": "test"},
			"Env": ["FOO=bar","PASSWORD=secret","EMPTY="],
			"Cmd": ["nginx"],
			"Entrypoint": ["/entrypoint.sh"]
		},
		"State": {
			"Status": "running",
			"Running": true,
			"Health": {"Status": "healthy"}
		},
		"HostConfig": {
			"Binds": ["/host:/container:ro"],
			"PortBindings": {"8080/tcp":[{"HostIp":"0.0.0.0","HostPort":"18080"}]},
			"NanoCpus": 500000000,
			"Memory": 268435456
		},
		"NetworkSettings": {
			"Networks": {"epNet":{"IPAddress":"172.18.0.2"}}
		}
	}`

	minimalJSON := `{}`

	tests := []struct {
		name         string
		containerID  string
		responseBody string
		statusCode   int
		wantErr      bool
		check        func(t *testing.T, d *agentdomain.ContainerDetails)
	}{
		{
			name:         "full success",
			containerID:  "test-id",
			responseBody: fullJSON,
			statusCode:   200,
			wantErr:      false,
			check: func(t *testing.T, d *agentdomain.ContainerDetails) {
				if d.ContainerID != "abc123" {
					t.Errorf("expected ContainerID abc123, got %s", d.ContainerID)
				}
				if d.Name != "test-container" {
					t.Errorf("expected Name test-container, got %s", d.Name)
				}
				if d.State != "running" {
					t.Errorf("expected State running, got %s", d.State)
				}
				if d.Image != "nginx:latest" {
					t.Errorf("expected Image nginx:latest, got %s", d.Image)
				}
				if !d.Running {
					t.Errorf("expected Running true")
				}
				if d.RestartCount != 2 {
					t.Errorf("expected RestartCount 2, got %d", d.RestartCount)
				}
				if d.Health != "healthy" {
					t.Errorf("expected Health healthy, got %s", d.Health)
				}
				if d.IPAddress != "172.18.0.2" {
					t.Errorf("expected IPAddress 172.18.0.2, got %s", d.IPAddress)
				}
				if d.CPULimit != 0.5 {
					t.Errorf("expected CPULimit 0.5, got %f", d.CPULimit)
				}
				if d.MemoryLimit != 256 {
					t.Errorf("expected MemoryLimit 256, got %d", d.MemoryLimit)
				}
				if len(d.Volumes) != 1 || d.Volumes[0].Source != "/host" {
					t.Errorf("expected 1 volume, got %v", d.Volumes)
				}
				if len(d.Ports) != 1 || d.Ports[0].ContainerPort != 8080 {
					t.Errorf("expected 1 port, got %v", d.Ports)
				}
				if d.Env["FOO"] != "bar" {
					t.Errorf("expected FOO=bar, got %s", d.Env["FOO"])
				}
				if d.Env["PASSWORD"] != "[REDACTED]" {
					t.Errorf("expected PASSWORD=[REDACTED], got %s", d.Env["PASSWORD"])
				}
				if d.CreatedAt == 0 {
					t.Errorf("expected non-zero CreatedAt")
				}
			},
		},
		{
			name:         "404 error",
			containerID:  "missing-id",
			responseBody: `{"message":"No such container"}`,
			statusCode:   404,
			wantErr:      true,
		},
		{
			name:         "malformed json",
			containerID:  "bad-id",
			responseBody: `{"bad`,
			statusCode:   200,
			wantErr:      true,
		},
		{
			name:         "minimal response",
			containerID:  "min-id",
			responseBody: minimalJSON,
			statusCode:   200,
			wantErr:      false,
			check: func(t *testing.T, d *agentdomain.ContainerDetails) {
				if d.ContainerID != "" {
					t.Errorf("expected empty ContainerID, got %s", d.ContainerID)
				}
				if d.Health != "" {
					t.Errorf("expected empty Health, got %s", d.Health)
				}
				if d.IPAddress != "" {
					t.Errorf("expected empty IPAddress, got %s", d.IPAddress)
				}
				if len(d.Volumes) != 0 {
					t.Errorf("expected 0 volumes, got %d", len(d.Volumes))
				}
				if len(d.Ports) != 0 {
					t.Errorf("expected 0 ports, got %d", len(d.Ports))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/containers/" + tt.containerID + "/json"
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := &DockerClient{
				httpClient: &http.Client{},
				cfg:        &config.AgentRuntimeConfig{ProxyNetworkName: "epNet"},
				endpoint: &dockerEndpoint{
					scheme:  "http",
					baseURL: server.URL,
				},
			}

			details, err := client.GetContainerDetails(context.Background(), tt.containerID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, details)
			}
		})
	}
}

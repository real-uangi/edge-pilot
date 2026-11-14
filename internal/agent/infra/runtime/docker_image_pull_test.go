package runtime

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"
)

type imagePullTransport func(*http.Request) (*http.Response, error)

func (f imagePullTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestEnsureImageRefreshesWorkloadsAndCachesInfrastructure(t *testing.T) {
	for _, tt := range []struct {
		name       string
		tag        string
		workload   bool
		cached     bool
		pullStatus int
		pullBody   string
		wantPulls  int
		wantErr    bool
	}{
		{name: "cached latest workload", tag: "latest", workload: true, cached: true, wantPulls: 1},
		{name: "cached overwritten tag workload", tag: "v1.0", workload: true, cached: true, wantPulls: 1},
		{name: "cached infrastructure", tag: "stable", cached: true},
		{name: "missing infrastructure", tag: "stable", wantPulls: 1},
		{name: "cached workload pull fails", tag: "latest", workload: true, cached: true, pullStatus: http.StatusUnauthorized, wantPulls: 1, wantErr: true},
		{name: "pull stream fails", tag: "latest", workload: true, cached: true, pullBody: `{"error":"manifest unknown"}`, wantPulls: 1, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewRawDockerClient(&config.AgentRuntimeConfig{})
			if err != nil {
				t.Fatal(err)
			}
			imageRef := "ghcr.io/team/service:" + tt.tag
			cached := tt.cached
			pulls := 0
			client.httpClient.Transport = imagePullTransport(func(req *http.Request) (*http.Response, error) {
				status := http.StatusOK
				body := `{}`
				switch req.Method {
				case http.MethodGet:
					if !cached {
						status = http.StatusNotFound
					}
				case http.MethodPost:
					pulls++
					if req.URL.Path != "/images/create" || req.URL.Query().Get("fromImage") != imageRef {
						t.Fatalf("unexpected pull request: %s", req.URL)
					}
					if tt.workload && req.Header.Get("X-Registry-Auth") == "" {
						t.Fatal("workload pull is missing registry auth")
					}
					if tt.pullStatus != 0 {
						status = tt.pullStatus
					}
					body = tt.pullBody
					if body == "" {
						body = `{"status":"Download complete"}`
					}
					cached = true
				default:
					t.Fatalf("unexpected method: %s", req.Method)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			var task *grpcapi.TaskCommand
			if tt.workload {
				task = &grpcapi.TaskCommand{
					Type:             grpcapi.TaskType_TASK_TYPE_DEPLOY_GREEN,
					RegistryHost:     "ghcr.io",
					RegistryUsername: "operator",
					RegistrySecret:   "test-token",
				}
			}
			err = client.EnsureImage(context.Background(), imageRef, task)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EnsureImage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if pulls != tt.wantPulls {
				t.Fatalf("pull calls = %d, want %d", pulls, tt.wantPulls)
			}
		})
	}
}

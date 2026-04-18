package runtime

import (
	"context"
	"edge-pilot/internal/shared/config"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const dockerUnixBaseURL = "http://docker"

type dockerEndpoint struct {
	raw        string
	scheme     string
	baseURL    string
	socketPath string
}

func newDockerEndpoint(cfg *config.AgentRuntimeConfig) (*dockerEndpoint, error) {
	if cfg == nil {
		return parseDockerEndpoint("")
	}
	host, _ := config.ResolveDockerEndpointConfig(cfg.DockerHost, cfg.DockerSocketPath)
	return parseDockerEndpoint(host)
}

func parseDockerEndpoint(raw string) (*dockerEndpoint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "/var/run/docker.sock"
	}
	if looksLikeFilesystemPath(raw) {
		return &dockerEndpoint{
			raw:        raw,
			scheme:     "unix",
			baseURL:    dockerUnixBaseURL,
			socketPath: raw,
		}, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse docker host %q: %w", raw, err)
	}
	switch strings.ToLower(strings.TrimSpace(parsed.Scheme)) {
	case "unix":
		socketPath := strings.TrimSpace(parsed.Path)
		if socketPath == "" {
			socketPath = strings.TrimSpace(parsed.Opaque)
		}
		if socketPath == "" {
			return nil, fmt.Errorf("docker host %q missing unix socket path", raw)
		}
		return &dockerEndpoint{
			raw:        raw,
			scheme:     "unix",
			baseURL:    dockerUnixBaseURL,
			socketPath: socketPath,
		}, nil
	case "npipe":
		if runtime.GOOS != "windows" {
			return nil, fmt.Errorf("docker host %q uses npipe, which is unsupported on %s", raw, runtime.GOOS)
		}
		return nil, fmt.Errorf("docker host %q uses npipe, which is not implemented", raw)
	case "tcp":
		if strings.TrimSpace(parsed.Host) == "" {
			return nil, fmt.Errorf("docker host %q missing TCP host", raw)
		}
		return &dockerEndpoint{
			raw:     raw,
			scheme:  "tcp",
			baseURL: "http://" + parsed.Host,
		}, nil
	case "http":
		if strings.TrimSpace(parsed.Host) == "" {
			return nil, fmt.Errorf("docker host %q missing HTTP host", raw)
		}
		return &dockerEndpoint{
			raw:     raw,
			scheme:  "http",
			baseURL: "http://" + parsed.Host,
		}, nil
	case "https":
		return nil, fmt.Errorf("docker host %q requires TLS configuration, which is unsupported", raw)
	default:
		return nil, fmt.Errorf("unsupported docker host scheme %q", parsed.Scheme)
	}
}

func looksLikeFilesystemPath(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return true
	}
	if filepath.IsAbs(value) {
		return true
	}
	return false
}

func (e *dockerEndpoint) newHTTPClient() *http.Client {
	transport := &http.Transport{}
	if e.scheme == "unix" {
		transport.DialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", e.socketPath)
		}
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Second}
}

func (e *dockerEndpoint) newRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return http.NewRequestWithContext(ctx, method, strings.TrimRight(e.baseURL, "/")+path, body)
}

func (e *dockerEndpoint) display() string {
	if e == nil {
		return ""
	}
	if e.scheme == "unix" {
		return e.socketPath
	}
	return e.raw
}

package config

import (
	"edge-pilot/internal/shared/buildinfo"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/real-uangi/allingo/common/log"
)

type AgentRuntimeConfig struct {
	AgentID                string
	AgentToken             string
	ControlPlaneAddr       string
	AgentVersion           string
	Hostname               string
	DockerSocketPath       string
	HTTPProbeTimeoutS      int
	ProxyNetworkName       string
	ProxyNetworkSubnet     string
	HAProxyImage           string
	ProxyHelperImage       string
	ProxyContainerName     string
	ProxyIPAddress         string
	HAProxyConfigVolume    string
	HAProxyRuntimePort     int
	DataPlaneAPIPort       int
	DataPlaneAPIUsername   string
	DataPlaneAPIPassword   string
	ProxySelfHealIntervalS int
}

func LoadAgentRuntimeConfig() (*AgentRuntimeConfig, error) {
	hostname, _ := os.Hostname()
	cfg := &AgentRuntimeConfig{
		AgentID:                strings.TrimSpace(os.Getenv("AGENT_ID")),
		AgentToken:             strings.TrimSpace(os.Getenv("AGENT_TOKEN")),
		ControlPlaneAddr:       defaultString(os.Getenv("CONTROL_PLANE_GRPC_ADDR"), "127.0.0.1:9090"),
		AgentVersion:           buildinfo.Version,
		Hostname:               hostname,
		DockerSocketPath:       defaultString(os.Getenv("DOCKER_SOCKET_PATH"), "/var/run/docker.sock"),
		HTTPProbeTimeoutS:      defaultInt(os.Getenv("HTTP_PROBE_TIMEOUT_SECONDS"), 5),
		ProxyNetworkName:       defaultString(os.Getenv("PROXY_NETWORK_NAME"), "epNet"),
		ProxyNetworkSubnet:     defaultString(os.Getenv("PROXY_NETWORK_SUBNET"), "172.29.0.0/24"),
		HAProxyImage:           defaultString(os.Getenv("HAPROXY_IMAGE"), "haproxytech/haproxy-debian:s6-3.4"),
		ProxyHelperImage:       defaultString(os.Getenv("PROXY_HELPER_IMAGE"), "busybox:1.36.1"),
		ProxyContainerName:     defaultString(os.Getenv("HAPROXY_CONTAINER_NAME"), "edge-pilot-haproxy"),
		ProxyIPAddress:         defaultString(os.Getenv("HAPROXY_IP"), "172.29.0.233"),
		HAProxyConfigVolume:    defaultString(os.Getenv("HAPROXY_CONFIG_VOLUME"), "ep_haproxy_cfg"),
		HAProxyRuntimePort:     defaultInt(os.Getenv("HAPROXY_RUNTIME_PORT"), 19999),
		DataPlaneAPIPort:       defaultInt(os.Getenv("DATAPLANEAPI_PORT"), 5555),
		DataPlaneAPIUsername:   defaultString(os.Getenv("HAPROXY_DATAPLANE_USERNAME"), "admin"),
		DataPlaneAPIPassword:   defaultString(os.Getenv("HAPROXY_DATAPLANE_PASSWORD"), "edge-pilot-internal"),
		ProxySelfHealIntervalS: defaultInt(os.Getenv("PROXY_SELF_HEAL_INTERVAL_SECONDS"), 10),
	}
	logger := log.NewStdLogger("agent.config")
	if cfg.AgentID == "" {
		err := fmt.Errorf("AGENT_ID is required; create agent credentials in control-plane and set AGENT_ID to the issued UUID")
		logger.Errorf(err, "invalid agent runtime config")
		return nil, err
	}
	if _, err := uuid.Parse(cfg.AgentID); err != nil {
		err = fmt.Errorf("AGENT_ID must be a UUID issued by control-plane: %w", err)
		logger.Errorf(err, "invalid agent runtime config")
		return nil, err
	}
	if cfg.AgentToken == "" {
		err := fmt.Errorf("AGENT_TOKEN is required; create or reset agent credentials in control-plane and set AGENT_TOKEN to the issued token")
		logger.Errorf(err, "invalid agent runtime config")
		return nil, err
	}
	return cfg, nil
}

func defaultString(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func defaultInt(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	var v int
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return fallback
		}
		v = v*10 + int(ch-'0')
	}
	if v <= 0 {
		return fallback
	}
	return v
}

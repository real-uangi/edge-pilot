package config

import (
	"edge-pilot/internal/shared/buildinfo"
	"os"
	"strings"
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

func LoadAgentRuntimeConfig() *AgentRuntimeConfig {
	hostname, _ := os.Hostname()
	cfg := &AgentRuntimeConfig{
		AgentID:                defaultString(os.Getenv("AGENT_ID"), hostname),
		AgentToken:             os.Getenv("AGENT_TOKEN"),
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
		ProxyIPAddress:         defaultString(os.Getenv("HAPROXY_IP"), "172.29.0.10"),
		HAProxyConfigVolume:    defaultString(os.Getenv("HAPROXY_CONFIG_VOLUME"), "ep_haproxy_cfg"),
		HAProxyRuntimePort:     defaultInt(os.Getenv("HAPROXY_RUNTIME_PORT"), 19999),
		DataPlaneAPIPort:       defaultInt(os.Getenv("DATAPLANEAPI_PORT"), 5555),
		DataPlaneAPIUsername:   defaultString(os.Getenv("HAPROXY_DATAPLANE_USERNAME"), "admin"),
		DataPlaneAPIPassword:   defaultString(os.Getenv("HAPROXY_DATAPLANE_PASSWORD"), "edge-pilot-internal"),
		ProxySelfHealIntervalS: defaultInt(os.Getenv("PROXY_SELF_HEAL_INTERVAL_SECONDS"), 10),
	}
	return cfg
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

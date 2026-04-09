package infra

import (
	"bytes"
	"context"
	"edge-pilot/internal/agent/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/real-uangi/allingo/common/log"
)

type DockerClient struct {
	httpClient *http.Client
	cfg        *config.AgentRuntimeConfig
	logger     *log.StdLogger
}

func NewRawDockerClient(cfg *config.AgentRuntimeConfig) *DockerClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", cfg.DockerSocketPath)
		},
	}
	return &DockerClient{
		httpClient: &http.Client{Transport: transport, Timeout: 15 * time.Second},
		cfg:        cfg,
		logger:     log.NewStdLogger("agent.docker"),
	}
}

func NewDockerClient(cfg *config.AgentRuntimeConfig) application.DockerRuntime {
	return NewRawDockerClient(cfg)
}

func (c *DockerClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("docker ping failed: %s", resp.Status)
	}
	return nil
}

func (c *DockerClient) DeployContainer(ctx context.Context, task *grpcapi.TaskCommand) (*application.ContainerRuntime, error) {
	createReq := dockerCreateRequest{
		Image:      task.GetImageRepo() + ":" + task.GetImageTag(),
		Env:        flattenEnv(task.GetEnv()),
		Cmd:        task.GetCommand(),
		Entrypoint: task.GetEntrypoint(),
		Labels: map[string]string{
			application.ManagedLabelKey:        application.ManagedLabelValue,
			application.ManagedLabelAgentID:    task.GetAgentId(),
			application.ManagedLabelServiceID:  task.GetServiceId(),
			application.ManagedLabelServiceKey: task.GetServiceKey(),
			application.ManagedLabelSlot:       application.ManagedSlotValue(task.GetTargetSlot()),
			application.ManagedLabelReleaseID:  task.GetReleaseId(),
		},
		ExposedPorts: exposedPorts(task),
		HostConfig: dockerHostConfig{
			PortBindings:  flattenPublishedPorts(task.GetPublishedPorts()),
			Binds:         flattenVolumes(task.GetVolumes()),
			RestartPolicy: dockerRestartPolicy{Name: "unless-stopped"},
		},
		NetworkingConfig: dockerNetworkingConfig{
			EndpointsConfig: map[string]dockerEndpointSettings{
				c.cfg.ProxyNetworkName: {},
			},
		},
	}
	body, err := json.Marshal(createReq)
	if err != nil {
		return nil, err
	}
	name := application.ManagedContainerName(task.GetServiceKey(), task.GetTargetSlot())
	createURL := "http://docker/containers/create?name=" + url.QueryEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker create failed: %s", resp.Status)
	}
	var createResp dockerCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, err
	}
	startReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/containers/"+createResp.ID+"/start", nil)
	if err != nil {
		return nil, err
	}
	startResp, err := c.httpClient.Do(startReq)
	if err != nil {
		return nil, err
	}
	defer startResp.Body.Close()
	if startResp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker start failed: %s", startResp.Status)
	}
	listenAddress, err := c.ResolveListenAddress(ctx, createResp.ID, int(task.GetContainerPort()))
	if err != nil {
		return nil, err
	}
	return &application.ContainerRuntime{
		ContainerID:   createResp.ID,
		ListenAddress: listenAddress,
	}, nil
}

func (c *DockerClient) InspectHealth(ctx context.Context, containerID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/"+containerID+"/json", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("docker inspect failed: %s", resp.Status)
	}
	var inspectResp dockerInspectResponse
	if err := json.NewDecoder(resp.Body).Decode(&inspectResp); err != nil {
		return "", err
	}
	if inspectResp.State.Health == nil {
		if inspectResp.State.Running {
			return "", nil
		}
		return inspectResp.State.Status, nil
	}
	return inspectResp.State.Health.Status, nil
}

func (c *DockerClient) FindContainerByName(ctx context.Context, name string) (*application.ManagedContainer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/"+url.PathEscape(name)+"/json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker inspect failed: %s", resp.Status)
	}
	var inspectResp dockerInspectResponse
	if err := json.NewDecoder(resp.Body).Decode(&inspectResp); err != nil {
		return nil, err
	}
	return toManagedContainer(&inspectResp), nil
}

func (c *DockerClient) ResolveListenAddress(ctx context.Context, containerID string, port int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/"+url.PathEscape(containerID)+"/json", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("docker inspect failed: %s", resp.Status)
	}
	var inspectResp dockerInspectResponse
	if err := json.NewDecoder(resp.Body).Decode(&inspectResp); err != nil {
		return "", err
	}
	endpoint, ok := inspectResp.NetworkSettings.Networks[c.cfg.ProxyNetworkName]
	if !ok || strings.TrimSpace(endpoint.IPAddress) == "" {
		return "", fmt.Errorf("container %s is not attached to network %s", containerID, c.cfg.ProxyNetworkName)
	}
	return net.JoinHostPort(endpoint.IPAddress, strconv.Itoa(port)), nil
}

func (c *DockerClient) RemoveContainer(ctx context.Context, containerID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://docker/containers/"+url.PathEscape(containerID)+"?force=1", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("docker remove failed: %s", resp.Status)
	}
	return nil
}

func (c *DockerClient) ListManagedContainers(ctx context.Context, agentID string, serviceKey string) ([]*application.ManagedContainer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker list failed: %s", resp.Status)
	}
	var items []dockerContainerSummary
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	out := make([]*application.ManagedContainer, 0, len(items))
	for _, item := range items {
		if item.Labels[application.ManagedLabelKey] != application.ManagedLabelValue {
			continue
		}
		if item.Labels[application.ManagedLabelAgentID] != agentID {
			continue
		}
		if item.Labels[application.ManagedLabelServiceKey] != serviceKey {
			continue
		}
		out = append(out, summaryToManagedContainer(item))
	}
	return out, nil
}

type dockerCreateRequest struct {
	Image            string                       `json:"Image"`
	Env              []string                     `json:"Env,omitempty"`
	Cmd              []string                     `json:"Cmd,omitempty"`
	Entrypoint       []string                     `json:"Entrypoint,omitempty"`
	Labels           map[string]string            `json:"Labels,omitempty"`
	ExposedPorts     map[string]map[string]string `json:"ExposedPorts,omitempty"`
	HostConfig       dockerHostConfig             `json:"HostConfig"`
	NetworkingConfig dockerNetworkingConfig       `json:"NetworkingConfig,omitempty"`
}

type dockerHostConfig struct {
	PortBindings  map[string][]dockerPortBinding `json:"PortBindings,omitempty"`
	Binds         []string                       `json:"Binds,omitempty"`
	RestartPolicy dockerRestartPolicy            `json:"RestartPolicy,omitempty"`
}

type dockerPortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type dockerCreateResponse struct {
	ID string `json:"Id"`
}

type dockerInspectResponse struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status  string `json:"Status"`
		Running bool   `json:"Running"`
		Health  *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

type dockerContainerSummary struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

func flattenEnv(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for key, value := range m {
		out = append(out, key+"="+value)
	}
	return out
}

func flattenVolumes(items []*grpcapi.VolumeMount) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		bind := item.GetSource() + ":" + item.GetTarget()
		if item.GetReadOnly() {
			bind += ":ro"
		}
		out = append(out, bind)
	}
	return out
}

func flattenPublishedPorts(items []*grpcapi.PublishedPort) map[string][]dockerPortBinding {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string][]dockerPortBinding, len(items))
	for _, item := range items {
		key := portKey(int(item.GetContainerPort()))
		out[key] = append(out[key], dockerPortBinding{
			HostIP:   "0.0.0.0",
			HostPort: strconv.Itoa(int(item.GetHostPort())),
		})
	}
	return out
}

func exposedPorts(task *grpcapi.TaskCommand) map[string]map[string]string {
	out := map[string]map[string]string{
		portKey(int(task.GetContainerPort())): {},
	}
	for _, item := range task.GetPublishedPorts() {
		out[portKey(int(item.GetContainerPort()))] = map[string]string{}
	}
	return out
}

func portKey(port int) string {
	return strconv.Itoa(port) + "/tcp"
}

func toManagedContainer(resp *dockerInspectResponse) *application.ManagedContainer {
	labels := resp.Config.Labels
	return &application.ManagedContainer{
		ContainerRuntime: application.ContainerRuntime{
			ContainerID:   resp.ID,
			ListenAddress: "",
		},
		Name:       strings.TrimPrefix(resp.Name, "/"),
		Managed:    labels[application.ManagedLabelKey] == application.ManagedLabelValue,
		AgentID:    labels[application.ManagedLabelAgentID],
		ServiceID:  labels[application.ManagedLabelServiceID],
		ServiceKey: labels[application.ManagedLabelServiceKey],
		ReleaseID:  labels[application.ManagedLabelReleaseID],
		Slot:       parseSlot(labels[application.ManagedLabelSlot]),
		State:      resp.State.Status,
	}
}

func summaryToManagedContainer(item dockerContainerSummary) *application.ManagedContainer {
	name := ""
	if len(item.Names) > 0 {
		name = strings.TrimPrefix(item.Names[0], "/")
	}
	return &application.ManagedContainer{
		ContainerRuntime: application.ContainerRuntime{
			ContainerID: item.ID,
		},
		Name:       name,
		Managed:    item.Labels[application.ManagedLabelKey] == application.ManagedLabelValue,
		AgentID:    item.Labels[application.ManagedLabelAgentID],
		ServiceID:  item.Labels[application.ManagedLabelServiceID],
		ServiceKey: item.Labels[application.ManagedLabelServiceKey],
		ReleaseID:  item.Labels[application.ManagedLabelReleaseID],
		Slot:       parseSlot(item.Labels[application.ManagedLabelSlot]),
		State:      item.State,
	}
}

func parseSlot(value string) grpcapi.Slot {
	switch value {
	case "blue":
		return grpcapi.Slot_SLOT_BLUE
	case "green":
		return grpcapi.Slot_SLOT_GREEN
	default:
		return grpcapi.Slot_SLOT_UNSPECIFIED
	}
}

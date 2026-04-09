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
	"time"
)

type DockerClient struct {
	httpClient *http.Client
}

func NewDockerClient(cfg *config.AgentRuntimeConfig) application.DockerRuntime {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", cfg.DockerSocketPath)
		},
	}
	return &DockerClient{
		httpClient: &http.Client{Transport: transport, Timeout: 15 * time.Second},
	}
}

func (c *DockerClient) DeployContainer(ctx context.Context, task *grpcapi.TaskCommand) (*application.ContainerRuntime, error) {
	createReq := dockerCreateRequest{
		Image:      task.GetImageRepo() + ":" + task.GetImageTag(),
		Env:        flattenEnv(task.GetEnv()),
		Cmd:        task.GetCommand(),
		Entrypoint: task.GetEntrypoint(),
		ExposedPorts: map[string]map[string]string{
			portKey(int(task.GetContainerPort())): {},
		},
		HostConfig: dockerHostConfig{
			PortBindings: map[string][]dockerPortBinding{
				portKey(int(task.GetContainerPort())): {
					{HostIP: "0.0.0.0", HostPort: strconv.Itoa(int(task.GetHostPort()))},
				},
			},
			Binds: flattenVolumes(task.GetVolumes()),
		},
	}
	body, err := json.Marshal(createReq)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%d-%d", task.GetServiceKey(), task.GetTargetSlot(), time.Now().Unix())
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
	return &application.ContainerRuntime{
		ContainerID:   createResp.ID,
		ListenAddress: "127.0.0.1:" + strconv.Itoa(int(task.GetHostPort())),
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
		return "stopped", nil
	}
	return inspectResp.State.Health.Status, nil
}

type dockerCreateRequest struct {
	Image        string                       `json:"Image"`
	Env          []string                     `json:"Env,omitempty"`
	Cmd          []string                     `json:"Cmd,omitempty"`
	Entrypoint   []string                     `json:"Entrypoint,omitempty"`
	ExposedPorts map[string]map[string]string `json:"ExposedPorts,omitempty"`
	HostConfig   dockerHostConfig             `json:"HostConfig"`
}

type dockerHostConfig struct {
	PortBindings map[string][]dockerPortBinding `json:"PortBindings,omitempty"`
	Binds        []string                       `json:"Binds,omitempty"`
}

type dockerPortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type dockerCreateResponse struct {
	ID string `json:"Id"`
}

type dockerInspectResponse struct {
	State struct {
		Running bool `json:"Running"`
		Health  *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
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

func portKey(port int) string {
	return strconv.Itoa(port) + "/tcp"
}

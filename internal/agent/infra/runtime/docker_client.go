package runtime

import (
	"bytes"
	"context"
	agentdomain "edge-pilot/internal/agent/domain"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	endpoint   *dockerEndpoint
	logger     *log.StdLogger
}

func NewRawDockerClient(cfg *config.AgentRuntimeConfig) (*DockerClient, error) {
	endpoint, err := newDockerEndpoint(cfg)
	if err != nil {
		return nil, err
	}
	return &DockerClient{
		httpClient: endpoint.newHTTPClient(),
		cfg:        cfg,
		endpoint:   endpoint,
		logger:     log.NewStdLogger("agent.docker"),
	}, nil
}

func NewDockerClient(cfg *config.AgentRuntimeConfig) (agentdomain.DockerRuntime, error) {
	return NewRawDockerClient(cfg)
}

func (c *DockerClient) Ping(ctx context.Context) error {
	req, err := c.endpoint.newRequest(ctx, http.MethodGet, "/_ping", nil)
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

func (c *DockerClient) DeployContainer(ctx context.Context, task *grpcapi.TaskCommand) (*agentdomain.ContainerRuntime, error) {
	imageRef := task.GetImageRepo() + ":" + task.GetImageTag()
	if err := c.ensureImage(ctx, imageRef, task); err != nil {
		return nil, err
	}
	createReq := buildWorkloadCreateRequest(c.cfg, imageRef, task)
	body, err := json.Marshal(createReq)
	if err != nil {
		return nil, err
	}
	name := agentdomain.ManagedContainerNameForTask(task.GetServiceKey(), task.GetReleaseId(), task.GetTargetSlot())
	c.logger.Infof(
		"creating managed workload container: name=%s image=%s restartPolicy=%s maxRetries=%d",
		name,
		imageRef,
		createReq.HostConfig.RestartPolicy.Name,
		createReq.HostConfig.RestartPolicy.MaximumRetryCount,
	)
	req, err := c.endpoint.newRequest(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), bytes.NewReader(body))
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
	startReq, err := c.endpoint.newRequest(ctx, http.MethodPost, "/containers/"+createResp.ID+"/start", nil)
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
	return &agentdomain.ContainerRuntime{
		ContainerID:   createResp.ID,
		ListenAddress: listenAddress,
		Image:         imageRef,
	}, nil
}

func buildWorkloadCreateRequest(cfg *config.AgentRuntimeConfig, imageRef string, task *grpcapi.TaskCommand) dockerCreateRequest {
	networkAliases := task.GetNetworkAliases()
	return dockerCreateRequest{
		Image:      imageRef,
		Env:        flattenEnv(task.GetEnv()),
		Cmd:        task.GetCommand(),
		Entrypoint: task.GetEntrypoint(),
		Labels: map[string]string{
			agentdomain.ManagedLabelKey:        agentdomain.ManagedLabelValue,
			agentdomain.ManagedLabelAgentID:    task.GetAgentId(),
			agentdomain.ManagedLabelServiceID:  task.GetServiceId(),
			agentdomain.ManagedLabelServiceKey: task.GetServiceKey(),
			agentdomain.ManagedLabelSlot:       agentdomain.ManagedSlotValue(task.GetTargetSlot()),
			agentdomain.ManagedLabelReleaseID:  task.GetReleaseId(),
		},
		ExposedPorts: exposedPorts(task),
		HostConfig: dockerHostConfig{
			NetworkMode:  cfg.ProxyNetworkName,
			PortBindings: flattenPublishedPorts(task.GetPublishedPorts()),
			Binds:        flattenVolumes(task.GetVolumes()),
			NanoCpus:     cpuLimitCoresToNanoCPUs(task.GetCpuLimitCores()),
			Memory:       memoryLimitMBToBytes(task.GetMemoryLimitMb()),
			RestartPolicy: dockerRestartPolicy{
				Name:              "on-failure",
				MaximumRetryCount: 5,
			},
		},
		NetworkingConfig: dockerNetworkingConfig{
			EndpointsConfig: map[string]dockerEndpointSettings{
				cfg.ProxyNetworkName: {
					Aliases: networkAliases,
				},
			},
		},
	}
}

func (c *DockerClient) ensureImage(ctx context.Context, imageRef string, task *grpcapi.TaskCommand) error {
	exists, err := c.imageExists(ctx, imageRef)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	c.logger.Infof("pulling docker image: image=%s", imageRef)
	pullCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	pullReq, err := c.endpoint.newRequest(pullCtx, http.MethodPost, "/images/create?fromImage="+url.QueryEscape(imageRef), nil)
	if err != nil {
		return err
	}
	auth := taskRegistryAuth{}
	if task != nil {
		auth.host = task.GetRegistryHost()
		auth.username = task.GetRegistryUsername()
		auth.secret = task.GetRegistrySecret()
	}
	if header, ok, err := buildRegistryAuthHeader(auth); err != nil {
		return err
	} else if ok {
		pullReq.Header.Set("X-Registry-Auth", header)
	}
	pullClient := &http.Client{
		Transport: c.httpClient.Transport,
		Timeout:   5 * time.Minute,
	}
	pullResp, err := pullClient.Do(pullReq)
	if err != nil {
		return err
	}
	defer pullResp.Body.Close()
	if pullResp.StatusCode >= 300 {
		body, _ := io.ReadAll(pullResp.Body)
		return fmt.Errorf("docker pull image failed: %s %s", pullResp.Status, strings.TrimSpace(string(body)))
	}
	if err := consumeDockerPullStream(pullResp.Body); err != nil {
		return fmt.Errorf("docker pull image failed: %w", err)
	}

	exists, err = c.imageExists(ctx, imageRef)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("docker image %s still not present after pull", imageRef)
	}
	return nil
}

func (c *DockerClient) imageExists(ctx context.Context, imageRef string) (bool, error) {
	req, err := c.endpoint.newRequest(ctx, http.MethodGet, "/images/"+url.PathEscape(imageRef)+"/json", nil)
	if err != nil {
		return false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("docker inspect image failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return true, nil
}

type taskRegistryAuth struct {
	host     string
	username string
	secret   string
}

func buildRegistryAuthHeader(auth taskRegistryAuth) (string, bool, error) {
	if strings.TrimSpace(auth.host) == "" || strings.TrimSpace(auth.username) == "" || strings.TrimSpace(auth.secret) == "" {
		return "", false, nil
	}
	payload, err := json.Marshal(map[string]string{
		"username":      auth.username,
		"password":      auth.secret,
		"serveraddress": auth.host,
	})
	if err != nil {
		return "", false, err
	}
	return base64.URLEncoding.EncodeToString(payload), true, nil
}

type dockerPullStatusLine struct {
	Status      string `json:"status"`
	Error       string `json:"error"`
	ErrorDetail *struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func consumeDockerPullStream(r io.Reader) error {
	decoder := json.NewDecoder(r)
	for {
		var line dockerPullStatusLine
		if err := decoder.Decode(&line); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if strings.TrimSpace(line.Error) != "" {
			return fmt.Errorf("%s", strings.TrimSpace(line.Error))
		}
		if line.ErrorDetail != nil && strings.TrimSpace(line.ErrorDetail.Message) != "" {
			return fmt.Errorf("%s", strings.TrimSpace(line.ErrorDetail.Message))
		}
	}
}

func (c *DockerClient) fetchDockerInspect(ctx context.Context, containerID string) (*dockerInspectResponse, error) {
	req, err := c.endpoint.newRequest(ctx, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker inspect failed: %s", resp.Status)
	}
	var inspectResp dockerInspectResponse
	if err := json.NewDecoder(resp.Body).Decode(&inspectResp); err != nil {
		return nil, err
	}
	return &inspectResp, nil
}

func (c *DockerClient) InspectContainer(ctx context.Context, containerID string) (*agentdomain.ContainerStatus, error) {
	inspectResp, err := c.fetchDockerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}
	status := &agentdomain.ContainerStatus{
		State:   inspectResp.State.Status,
		Running: inspectResp.State.Running,
	}
	if inspectResp.State.Health != nil {
		status.Health = inspectResp.State.Health.Status
	}
	return status, nil
}

func (c *DockerClient) GetContainerDetails(ctx context.Context, containerID string) (*agentdomain.ContainerDetails, error) {
	inspectResp, err := c.fetchDockerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	details := &agentdomain.ContainerDetails{
		ContainerID:  inspectResp.ID,
		Name:         strings.TrimPrefix(inspectResp.Name, "/"),
		State:        inspectResp.State.Status,
		Image:        inspectResp.Config.Image,
		Running:      inspectResp.State.Running,
		RestartCount: int32(inspectResp.RestartCount),
		Labels:       inspectResp.Config.Labels,
		Env:          parseEnv(inspectResp.Config.Env),
		Command:      inspectResp.Config.Cmd,
		Entrypoint:   inspectResp.Config.Entrypoint,
		CPULimit:     nanoCPUsToCores(inspectResp.HostConfig.NanoCpus),
		MemoryLimit:  inspectResp.HostConfig.Memory / (1024 * 1024),
		CreatedAt:    inspectResp.Created.Unix(),
	}
	if inspectResp.State.Health != nil {
		details.Health = inspectResp.State.Health.Status
	}
	if endpoint, ok := inspectResp.NetworkSettings.Networks[c.cfg.ProxyNetworkName]; ok {
		details.IPAddress = endpoint.IPAddress
	}
	if binds := inspectResp.HostConfig.Binds; len(binds) > 0 {
		details.Volumes = parseVolumeMounts(binds)
	}
	if ports := inspectResp.HostConfig.PortBindings; len(ports) > 0 {
		details.Ports = parsePublishedPorts(ports)
	}
	return details, nil
}

func nanoCPUsToCores(nanoCPUs int64) float64 {
	if nanoCPUs <= 0 {
		return 0
	}
	return float64(nanoCPUs) / 1_000_000_000
}

func isSensitiveEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	sensitive := []string{"PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIAL", "AUTH"}
	for _, s := range sensitive {
		if strings.Contains(upper, s) {
			return true
		}
	}
	return false
}

func parseEnv(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		if idx := strings.IndexByte(e, '='); idx > 0 {
			key := e[:idx]
			value := e[idx+1:]
			if isSensitiveEnvKey(key) {
				value = "[REDACTED]"
			}
			out[key] = value
		}
	}
	return out
}

func parseVolumeMounts(binds []string) []*agentdomain.VolumeMount {
	out := make([]*agentdomain.VolumeMount, 0, len(binds))
	for _, bind := range binds {
		parts := strings.SplitN(bind, ":", 3)
		if len(parts) < 2 {
			continue
		}
		vm := &agentdomain.VolumeMount{Source: parts[0], Target: parts[1]}
		if len(parts) == 3 {
			for _, opt := range strings.Split(parts[2], ",") {
				if opt == "ro" {
					vm.ReadOnly = true
					break
				}
			}
		}
		out = append(out, vm)
	}
	return out
}

func parsePublishedPorts(portBindings map[string][]dockerPortBinding) []*agentdomain.PublishedPort {
	out := make([]*agentdomain.PublishedPort, 0, len(portBindings))
	for containerPortProto, bindings := range portBindings {
		containerPortStr := strings.SplitN(containerPortProto, "/", 2)[0]
		containerPort, err := strconv.Atoi(containerPortStr)
		if err != nil {
			continue
		}
		for _, binding := range bindings {
			hostPort, err := strconv.Atoi(binding.HostPort)
			if err != nil {
				continue
			}
			out = append(out, &agentdomain.PublishedPort{
				ContainerPort: int32(containerPort),
				HostPort:      int32(hostPort),
			})
		}
	}
	return out
}

func (c *DockerClient) StreamContainerLogs(ctx context.Context, containerID string, tailLines int, stdout, stderr, follow bool) (io.ReadCloser, error) {
	query := fmt.Sprintf("/containers/%s/logs?", url.PathEscape(containerID))
	params := []string{}
	if stdout {
		params = append(params, "stdout=1")
	}
	if stderr {
		params = append(params, "stderr=1")
	}
	if follow {
		params = append(params, "follow=1")
	}
	if tailLines > 0 {
		params = append(params, fmt.Sprintf("tail=%d", tailLines))
	}
	query += strings.Join(params, "&")

	req, err := c.endpoint.newRequest(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("docker logs stream failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return resp.Body, nil
}

func (c *DockerClient) ReadContainerLogs(ctx context.Context, containerID string, tailLines int, maxBytes int) (string, error) {
	req, err := c.endpoint.newRequest(ctx, http.MethodGet, fmt.Sprintf("/containers/%s/logs?stdout=1&stderr=1&tail=%d", url.PathEscape(containerID), tailLines), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("docker logs failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	decoded := decodeDockerLogStream(data)
	if maxBytes > 0 && len(decoded) > maxBytes {
		decoded = decoded[len(decoded)-maxBytes:]
	}
	return strings.TrimSpace(decoded), nil
}

func (c *DockerClient) FindContainerByName(ctx context.Context, name string) (*agentdomain.ManagedContainer, error) {
	req, err := c.endpoint.newRequest(ctx, http.MethodGet, "/containers/"+url.PathEscape(name)+"/json", nil)
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

func (c *DockerClient) FindManagedContainerByIdentity(ctx context.Context, identity agentdomain.ManagedContainerIdentity) (*agentdomain.ManagedContainer, error) {
	filters := map[string][]string{
		"label": {
			agentdomain.ManagedLabelKey + "=" + agentdomain.ManagedLabelValue,
			agentdomain.ManagedLabelAgentID + "=" + strings.TrimSpace(identity.AgentID),
			agentdomain.ManagedLabelServiceKey + "=" + strings.TrimSpace(identity.ServiceKey),
		},
	}
	releaseID := strings.TrimSpace(identity.ReleaseID)
	if releaseID != "" {
		filters["label"] = append(filters["label"], agentdomain.ManagedLabelReleaseID+"="+releaseID)
	} else {
		slot := strings.TrimSpace(agentdomain.ManagedSlotValue(identity.Slot))
		if slot != "" && slot != "unknown" {
			filters["label"] = append(filters["label"], agentdomain.ManagedLabelSlot+"="+slot)
		}
	}
	items, err := c.listManagedContainersByFilters(ctx, filters)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("managed container conflict: found %d containers for serviceKey=%s releaseId=%s slot=%s", len(items), identity.ServiceKey, releaseID, identity.Slot.String())
	}
	return items[0], nil
}

func (c *DockerClient) ResolveListenAddress(ctx context.Context, containerID string, port int) (string, error) {
	req, err := c.endpoint.newRequest(ctx, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/json", nil)
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
	req, err := c.endpoint.newRequest(ctx, http.MethodDelete, "/containers/"+url.PathEscape(containerID)+"?force=1", nil)
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

func (c *DockerClient) RemoveImage(ctx context.Context, imageRef string) error {
	if strings.TrimSpace(imageRef) == "" {
		return nil
	}
	c.logger.Infof("removing docker image: image=%s", imageRef)
	req, err := c.endpoint.newRequest(ctx, http.MethodDelete, "/images/"+url.PathEscape(imageRef)+"?force=1", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker remove image failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	c.logger.Infof("removed docker image: image=%s", imageRef)
	return nil
}

func (c *DockerClient) ListManagedContainers(ctx context.Context, agentID string, serviceKey string) ([]*agentdomain.ManagedContainer, error) {
	filters := map[string][]string{
		"label": {
			agentdomain.ManagedLabelKey + "=" + agentdomain.ManagedLabelValue,
			agentdomain.ManagedLabelAgentID + "=" + strings.TrimSpace(agentID),
		},
	}
	expectedServiceKey := strings.TrimSpace(serviceKey)
	if expectedServiceKey != "" {
		filters["label"] = append(filters["label"], agentdomain.ManagedLabelServiceKey+"="+expectedServiceKey)
	}
	return c.listManagedContainersByFilters(ctx, filters)
}

func (c *DockerClient) listManagedContainersByFilters(ctx context.Context, filters map[string][]string) ([]*agentdomain.ManagedContainer, error) {
	path := "/containers/json?all=1"
	if len(filters) > 0 {
		encoded, err := json.Marshal(filters)
		if err != nil {
			return nil, err
		}
		path += "&filters=" + url.QueryEscape(string(encoded))
	}
	req, err := c.endpoint.newRequest(ctx, http.MethodGet, path, nil)
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
	out := make([]*agentdomain.ManagedContainer, 0, len(items))
	for _, item := range items {
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
	NetworkMode   string                         `json:"NetworkMode,omitempty"`
	PortBindings  map[string][]dockerPortBinding `json:"PortBindings,omitempty"`
	Binds         []string                       `json:"Binds,omitempty"`
	Tmpfs         map[string]string              `json:"Tmpfs,omitempty"`
	NanoCpus      int64                          `json:"NanoCpus,omitempty"`
	Memory        int64                          `json:"Memory,omitempty"`
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
	ID           string    `json:"Id"`
	Name         string    `json:"Name"`
	Created      time.Time `json:"Created"`
	RestartCount int       `json:"RestartCount"`
	Config       struct {
		Image      string            `json:"Image"`
		Labels     map[string]string `json:"Labels"`
		Env        []string          `json:"Env"`
		Cmd        []string          `json:"Cmd"`
		Entrypoint []string          `json:"Entrypoint"`
	} `json:"Config"`
	State struct {
		Status  string `json:"Status"`
		Running bool   `json:"Running"`
		Health  *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	HostConfig struct {
		Binds        []string                       `json:"Binds"`
		PortBindings map[string][]dockerPortBinding `json:"PortBindings"`
		NanoCpus     int64                          `json:"NanoCpus"`
		Memory       int64                          `json:"Memory"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

type dockerContainerSummary struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	State   string            `json:"State"`
	Image   string            `json:"Image"`
	Created int64             `json:"Created"`
	Labels  map[string]string `json:"Labels"`
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

func cpuLimitCoresToNanoCPUs(cpuLimitCores float64) int64 {
	if cpuLimitCores <= 0 {
		return 0
	}
	return int64(math.Round(cpuLimitCores * 1_000_000_000))
}

func memoryLimitMBToBytes(memoryLimitMB int64) int64 {
	if memoryLimitMB <= 0 {
		return 0
	}
	return memoryLimitMB * 1024 * 1024
}

func toManagedContainer(resp *dockerInspectResponse) *agentdomain.ManagedContainer {
	labels := resp.Config.Labels
	return &agentdomain.ManagedContainer{
		ContainerRuntime: agentdomain.ContainerRuntime{
			ContainerID:   resp.ID,
			ListenAddress: "",
		},
		Name:       strings.TrimPrefix(resp.Name, "/"),
		Image:      resp.Config.Image,
		CreatedAt:  resp.Created.Unix(),
		Managed:    labels[agentdomain.ManagedLabelKey] == agentdomain.ManagedLabelValue,
		AgentID:    labels[agentdomain.ManagedLabelAgentID],
		ServiceID:  labels[agentdomain.ManagedLabelServiceID],
		ServiceKey: labels[agentdomain.ManagedLabelServiceKey],
		ReleaseID:  labels[agentdomain.ManagedLabelReleaseID],
		Slot:       parseSlot(labels[agentdomain.ManagedLabelSlot]),
		State:      resp.State.Status,
	}
}

func summaryToManagedContainer(item dockerContainerSummary) *agentdomain.ManagedContainer {
	name := ""
	if len(item.Names) > 0 {
		name = strings.TrimPrefix(item.Names[0], "/")
	}
	return &agentdomain.ManagedContainer{
		ContainerRuntime: agentdomain.ContainerRuntime{
			ContainerID: item.ID,
		},
		Name:       name,
		Image:      item.Image,
		CreatedAt:  item.Created,
		Managed:    item.Labels[agentdomain.ManagedLabelKey] == agentdomain.ManagedLabelValue,
		AgentID:    item.Labels[agentdomain.ManagedLabelAgentID],
		ServiceID:  item.Labels[agentdomain.ManagedLabelServiceID],
		ServiceKey: item.Labels[agentdomain.ManagedLabelServiceKey],
		ReleaseID:  item.Labels[agentdomain.ManagedLabelReleaseID],
		Slot:       parseSlot(item.Labels[agentdomain.ManagedLabelSlot]),
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

func decodeDockerLogStream(data []byte) string {
	if len(data) < 8 {
		return string(data)
	}
	var builder strings.Builder
	offset := 0
	framed := false
	for offset+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		next := offset + 8 + size
		if size < 0 || next > len(data) {
			break
		}
		builder.Write(data[offset+8 : next])
		offset = next
		framed = true
	}
	if !framed {
		return string(data)
	}
	if offset < len(data) {
		builder.Write(data[offset:])
	}
	return builder.String()
}

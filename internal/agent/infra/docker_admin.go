package infra

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	proxyStackLabelKey     = "ep.stack"
	proxyStackRoleLabelKey = "ep.stack.role"
	proxyStackAgentLabel   = "ep.agent_id"
	proxyStackSpecLabelKey = "ep.stack.spec_hash"
)

type managedContainerSpec struct {
	Name       string
	Image      string
	Labels     map[string]string
	Env        []string
	Cmd        []string
	Entrypoint []string
	Binds      []string
	PortBinds  map[string][]dockerPortBinding
	Exposed    map[string]map[string]string
	Network    string
	IPAddress  string
}

type dockerContainerInspect struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Status  string `json:"Status"`
		Running bool   `json:"Running"`
	} `json:"State"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

type dockerNetworkInspect struct {
	Name string `json:"Name"`
	IPAM struct {
		Config []struct {
			Subnet string `json:"Subnet"`
		} `json:"Config"`
	} `json:"IPAM"`
	Containers map[string]struct {
		Name        string `json:"Name"`
		IPv4Address string `json:"IPv4Address"`
	} `json:"Containers"`
}

type dockerVolumeInspect struct {
	Name string `json:"Name"`
}

type dockerCreateContainerRequest struct {
	Image            string                       `json:"Image"`
	Env              []string                     `json:"Env,omitempty"`
	Cmd              []string                     `json:"Cmd,omitempty"`
	Entrypoint       []string                     `json:"Entrypoint,omitempty"`
	Labels           map[string]string            `json:"Labels,omitempty"`
	ExposedPorts     map[string]map[string]string `json:"ExposedPorts,omitempty"`
	HostConfig       dockerHostConfig             `json:"HostConfig"`
	NetworkingConfig dockerNetworkingConfig       `json:"NetworkingConfig,omitempty"`
}

type dockerNetworkingConfig struct {
	EndpointsConfig map[string]dockerEndpointSettings `json:"EndpointsConfig,omitempty"`
}

type dockerEndpointSettings struct {
	IPAMConfig *dockerEndpointIPAMConfig `json:"IPAMConfig,omitempty"`
}

type dockerEndpointIPAMConfig struct {
	IPv4Address string `json:"IPv4Address,omitempty"`
}

type dockerRestartPolicy struct {
	Name string `json:"Name,omitempty"`
}

type dockerNetworkCreateRequest struct {
	Name           string            `json:"Name"`
	CheckDuplicate bool              `json:"CheckDuplicate"`
	Driver         string            `json:"Driver"`
	IPAM           dockerNetworkIPAM `json:"IPAM"`
}

type dockerNetworkIPAM struct {
	Config []dockerNetworkIPAMConfig `json:"Config"`
}

type dockerNetworkIPAMConfig struct {
	Subnet string `json:"Subnet"`
}

type dockerVolumeCreateRequest struct {
	Name string `json:"Name"`
}

type dockerNetworkConnectRequest struct {
	Container string `json:"Container"`
}

func (c *DockerClient) inspectManagedContainer(ctx context.Context, name string) (*dockerContainerInspect, error) {
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
		return nil, fmt.Errorf("docker inspect container failed: %s", resp.Status)
	}
	var inspect dockerContainerInspect
	if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
		return nil, err
	}
	return &inspect, nil
}

func (c *DockerClient) ensureNetwork(ctx context.Context, name string, subnet string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/networks/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		c.logger.Infof("creating docker network: name=%s subnet=%s", name, subnet)
		body, err := json.Marshal(dockerNetworkCreateRequest{
			Name:           name,
			CheckDuplicate: true,
			Driver:         "bridge",
			IPAM: dockerNetworkIPAM{
				Config: []dockerNetworkIPAMConfig{{Subnet: subnet}},
			},
		})
		if err != nil {
			return err
		}
		createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/networks/create", bytes.NewReader(body))
		if err != nil {
			return err
		}
		createReq.Header.Set("Content-Type", "application/json")
		createResp, err := c.httpClient.Do(createReq)
		if err != nil {
			return err
		}
		defer createResp.Body.Close()
		if createResp.StatusCode >= 300 {
			return fmt.Errorf("docker create network failed: %s", createResp.Status)
		}
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("docker inspect network failed: %s", resp.Status)
	}
	return nil
}

func (c *DockerClient) inspectNetwork(ctx context.Context, name string) (*dockerNetworkInspect, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/networks/"+url.PathEscape(name), nil)
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
		return nil, fmt.Errorf("docker inspect network failed: %s", resp.Status)
	}
	var inspect dockerNetworkInspect
	if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
		return nil, err
	}
	return &inspect, nil
}

func (c *DockerClient) ensureVolume(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/volumes/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		c.logger.Infof("creating docker volume: name=%s", name)
		body, err := json.Marshal(dockerVolumeCreateRequest{Name: name})
		if err != nil {
			return err
		}
		createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/volumes/create", bytes.NewReader(body))
		if err != nil {
			return err
		}
		createReq.Header.Set("Content-Type", "application/json")
		createResp, err := c.httpClient.Do(createReq)
		if err != nil {
			return err
		}
		defer createResp.Body.Close()
		if createResp.StatusCode >= 300 {
			return fmt.Errorf("docker create volume failed: %s", createResp.Status)
		}
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("docker inspect volume failed: %s", resp.Status)
	}
	return nil
}

func (c *DockerClient) recreateManagedContainer(ctx context.Context, spec managedContainerSpec) error {
	c.logger.Infof("creating managed proxy container: name=%s image=%s ip=%s network=%s", spec.Name, spec.Image, spec.IPAddress, spec.Network)
	labels := cloneStringMap(spec.Labels)
	labels[proxyStackSpecLabelKey] = specHash(spec)
	body, err := json.Marshal(dockerCreateContainerRequest{
		Image:        spec.Image,
		Env:          spec.Env,
		Cmd:          spec.Cmd,
		Entrypoint:   spec.Entrypoint,
		Labels:       labels,
		ExposedPorts: spec.Exposed,
		HostConfig: dockerHostConfig{
			PortBindings: spec.PortBinds,
			Binds:        spec.Binds,
			RestartPolicy: dockerRestartPolicy{
				Name: "unless-stopped",
			},
		},
		NetworkingConfig: dockerNetworkingConfig{
			EndpointsConfig: map[string]dockerEndpointSettings{
				spec.Network: {
					IPAMConfig: &dockerEndpointIPAMConfig{IPv4Address: spec.IPAddress},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/containers/create?name="+url.QueryEscape(spec.Name), bytes.NewReader(body))
	if err != nil {
		return err
	}
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := c.httpClient.Do(createReq)
	if err != nil {
		return err
	}
	defer createResp.Body.Close()
	if createResp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(createResp.Body)
		return fmt.Errorf("docker create container failed: %s %s", createResp.Status, strings.TrimSpace(string(respBody)))
	}
	var createOut dockerCreateResponse
	if err := json.NewDecoder(createResp.Body).Decode(&createOut); err != nil {
		return err
	}
	startReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/containers/"+createOut.ID+"/start", nil)
	if err != nil {
		return err
	}
	startResp, err := c.httpClient.Do(startReq)
	if err != nil {
		return err
	}
	defer startResp.Body.Close()
	if startResp.StatusCode >= 300 {
		return fmt.Errorf("docker start container failed: %s", startResp.Status)
	}
	return nil
}

func (c *DockerClient) ensureManagedContainer(ctx context.Context, spec managedContainerSpec) error {
	inspect, err := c.inspectManagedContainer(ctx, spec.Name)
	if err != nil {
		return err
	}
	if inspect == nil {
		c.logger.Infof("managed proxy container missing, creating: name=%s", spec.Name)
		return c.recreateManagedContainer(ctx, spec)
	}
	if inspect.Config.Labels[proxyStackLabelKey] != "true" || inspect.Config.Labels[proxyStackRoleLabelKey] != spec.Labels[proxyStackRoleLabelKey] {
		c.logger.Infof("managed proxy container name conflict: name=%s role=%s", spec.Name, spec.Labels[proxyStackRoleLabelKey])
		return fmt.Errorf("proxy stack container name conflict: %s is not managed by edge-pilot", spec.Name)
	}
	if !managedContainerMatches(inspect, spec) {
		c.logger.Infof("managed proxy container drift detected, recreating: name=%s currentImage=%s desiredImage=%s", spec.Name, inspect.Config.Image, spec.Image)
		if err := c.RemoveContainer(ctx, inspect.ID); err != nil {
			return err
		}
		return c.recreateManagedContainer(ctx, spec)
	}
	if inspect.State.Running {
		return nil
	}
	c.logger.Infof("managed proxy container stopped, restarting: name=%s containerId=%s", spec.Name, inspect.ID)
	startReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/containers/"+inspect.ID+"/start", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(startReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("docker start container failed: %s", resp.Status)
	}
	return nil
}

func (c *DockerClient) ensureContainerConnectedToNetwork(ctx context.Context, containerID string, networkName string) error {
	inspect, err := c.inspectManagedContainer(ctx, containerID)
	if err != nil {
		return err
	}
	if inspect == nil {
		return fmt.Errorf("container %s not found", containerID)
	}
	if _, ok := inspect.NetworkSettings.Networks[networkName]; ok {
		return nil
	}
	c.logger.Infof("connecting container to proxy network: containerId=%s network=%s", containerID, networkName)
	body, err := json.Marshal(dockerNetworkConnectRequest{Container: containerID})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/networks/"+url.PathEscape(networkName)+"/connect", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("docker connect network failed: %s", resp.Status)
	}
	return nil
}

func (c *DockerClient) writeVolumeFiles(ctx context.Context, helperImage string, volumeName string, files map[string]string) error {
	if len(files) == 0 {
		return nil
	}
	c.logger.Infof("writing proxy bootstrap files: volume=%s files=%d", volumeName, len(files))
	helperName := "ep-volume-init-" + strconvNow()
	spec := managedContainerSpec{
		Name:   helperName,
		Image:  helperImage,
		Labels: map[string]string{proxyStackLabelKey: "true", proxyStackRoleLabelKey: "volume-init"},
		Binds:  []string{volumeName + ":/target"},
	}
	body, err := json.Marshal(dockerCreateContainerRequest{
		Image:  spec.Image,
		Labels: spec.Labels,
		HostConfig: dockerHostConfig{
			Binds: spec.Binds,
		},
	})
	if err != nil {
		return err
	}
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/containers/create?name="+url.QueryEscape(helperName), bytes.NewReader(body))
	if err != nil {
		return err
	}
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := c.httpClient.Do(createReq)
	if err != nil {
		return err
	}
	defer createResp.Body.Close()
	if createResp.StatusCode >= 300 {
		return fmt.Errorf("docker create volume helper failed: %s", createResp.Status)
	}
	var createOut dockerCreateResponse
	if err := json.NewDecoder(createResp.Body).Decode(&createOut); err != nil {
		return err
	}
	defer func() {
		_ = c.RemoveContainer(context.Background(), createOut.ID)
	}()

	archive, err := buildTar(files)
	if err != nil {
		return err
	}
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://docker/containers/"+createOut.ID+"/archive?path=/target", bytes.NewReader(archive))
	if err != nil {
		return err
	}
	putReq.Header.Set("Content-Type", "application/x-tar")
	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return err
	}
	defer putResp.Body.Close()
	if putResp.StatusCode >= 300 {
		return fmt.Errorf("docker copy archive failed: %s", putResp.Status)
	}
	return nil
}

func managedContainerMatches(inspect *dockerContainerInspect, spec managedContainerSpec) bool {
	if inspect.Config.Image != spec.Image {
		return false
	}
	if inspect.Config.Labels[proxyStackRoleLabelKey] != spec.Labels[proxyStackRoleLabelKey] {
		return false
	}
	return inspect.Config.Labels[proxyStackSpecLabelKey] == specHash(spec)
}

func specHash(spec managedContainerSpec) string {
	parts := []string{
		spec.Name,
		spec.Image,
		spec.Network,
		spec.IPAddress,
	}
	parts = append(parts, sortedStrings(spec.Env)...)
	parts = append(parts, sortedStrings(spec.Cmd)...)
	parts = append(parts, sortedStrings(spec.Entrypoint)...)
	parts = append(parts, sortedStrings(spec.Binds)...)
	parts = append(parts, mapPairs(spec.PortBinds)...)
	parts = append(parts, mapKeys(spec.Exposed)...)
	parts = append(parts, mapPairsString(spec.Labels)...)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func buildTar(files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	keys := mapKeys(files)
	for _, name := range keys {
		content := []byte(files[name])
		header := &tar.Header{
			Name:    name,
			Mode:    0o644,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tw.Write(content); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func sortedStrings(items []string) []string {
	out := append([]string(nil), items...)
	sort.Strings(out)
	return out
}

func mapKeys[T any](items map[string]T) []string {
	out := make([]string, 0, len(items))
	for key := range items {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func mapPairs(items map[string][]dockerPortBinding) []string {
	keys := mapKeys(items)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		bindings := items[key]
		sort.Slice(bindings, func(i, j int) bool {
			if bindings[i].HostIP != bindings[j].HostIP {
				return bindings[i].HostIP < bindings[j].HostIP
			}
			return bindings[i].HostPort < bindings[j].HostPort
		})
		for _, binding := range bindings {
			out = append(out, key+"="+binding.HostIP+":"+binding.HostPort)
		}
	}
	return out
}

func mapPairsString(items map[string]string) []string {
	keys := mapKeys(items)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+items[key])
	}
	return out
}

func strconvNow() string {
	return strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
}

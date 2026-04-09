package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type DataPlaneAPIClient struct {
	httpClient  *http.Client
	resolveURL  func() string
	resolveUser func() string
	resolvePass func() string
}

type frontendSection struct {
	Name                     string                  `json:"name"`
	Mode                     string                  `json:"mode"`
	DefaultBackend           string                  `json:"default_backend"`
	Binds                    map[string]frontendBind `json:"binds"`
	ACLList                  []frontendACL           `json:"acl_list,omitempty"`
	BackendSwitchingRuleList []frontendSwitchRule    `json:"backend_switching_rule_list,omitempty"`
}

type frontendBind struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type frontendACL struct {
	Name      string `json:"acl_name"`
	Criterion string `json:"criterion"`
	Value     string `json:"value"`
	Index     int    `json:"index"`
}

type frontendSwitchRule struct {
	Name     string `json:"name"`
	Cond     string `json:"cond"`
	CondTest string `json:"cond_test"`
	Index    int    `json:"index"`
}

type backendSection struct {
	Name    string         `json:"name"`
	Mode    string         `json:"mode"`
	Balance backendBalance `json:"balance,omitempty"`
}

type backendBalance struct {
	Algorithm string `json:"algorithm"`
}

type backendServer struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Check   string `json:"check,omitempty"`
}

func newDataPlaneAPIClient(resolveURL func() string, resolveUser func() string, resolvePass func() string) *DataPlaneAPIClient {
	return &DataPlaneAPIClient{
		httpClient:  &http.Client{},
		resolveURL:  resolveURL,
		resolveUser: resolveUser,
		resolvePass: resolvePass,
	}
}

func (c *DataPlaneAPIClient) ConfigurationVersion(ctx context.Context) (string, error) {
	respBody, err := c.do(ctx, http.MethodGet, "/v3/services/haproxy/configuration/version", nil)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(respBody))
	if trimmed == "" {
		return "", fmt.Errorf("empty dataplane version response")
	}
	if strings.HasPrefix(trimmed, "{") {
		var payload map[string]any
		if err := json.Unmarshal(respBody, &payload); err != nil {
			return "", err
		}
		switch value := payload["_version"].(type) {
		case float64:
			return strconv.Itoa(int(value)), nil
		case string:
			return value, nil
		}
	}
	return strings.Trim(trimmed, `"`), nil
}

func (c *DataPlaneAPIClient) ReplaceFrontend(ctx context.Context, section frontendSection) error {
	version, err := c.ConfigurationVersion(ctx)
	if err != nil {
		return err
	}
	path := "/v3/services/haproxy/configuration/frontends/" + url.PathEscape(section.Name) + "?version=" + url.QueryEscape(version) + "&full_section=true"
	if _, err := c.do(ctx, http.MethodPut, path, section); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := "/v3/services/haproxy/configuration/frontends?version=" + url.QueryEscape(version) + "&full_section=true"
		_, err = c.do(ctx, http.MethodPost, createPath, section)
		return err
	}
	return nil
}

func (c *DataPlaneAPIClient) EnsureBackend(ctx context.Context, section backendSection) error {
	version, err := c.ConfigurationVersion(ctx)
	if err != nil {
		return err
	}
	path := "/v3/services/haproxy/configuration/backends/" + url.PathEscape(section.Name) + "?version=" + url.QueryEscape(version)
	if _, err := c.do(ctx, http.MethodPut, path, section); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := "/v3/services/haproxy/configuration/backends?version=" + url.QueryEscape(version)
		_, err = c.do(ctx, http.MethodPost, createPath, section)
		return err
	}
	return nil
}

func (c *DataPlaneAPIClient) EnsureServer(ctx context.Context, backendName string, server backendServer) error {
	version, err := c.ConfigurationVersion(ctx)
	if err != nil {
		return err
	}
	path := "/v3/services/haproxy/configuration/backends/" + url.PathEscape(backendName) + "/servers/" + url.PathEscape(server.Name) + "?version=" + url.QueryEscape(version)
	if _, err := c.do(ctx, http.MethodPut, path, server); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := "/v3/services/haproxy/configuration/backends/" + url.PathEscape(backendName) + "/servers?version=" + url.QueryEscape(version)
		_, err = c.do(ctx, http.MethodPost, createPath, server)
		return err
	}
	return nil
}

func (c *DataPlaneAPIClient) ListBackends(ctx context.Context) ([]string, error) {
	body, err := c.do(ctx, http.MethodGet, "/v3/services/haproxy/configuration/backends", nil)
	if err != nil {
		return nil, err
	}
	type named struct {
		Name string `json:"name"`
	}
	var list []named
	if err := json.Unmarshal(body, &list); err == nil {
		out := make([]string, 0, len(list))
		for _, item := range list {
			if strings.TrimSpace(item.Name) != "" {
				out = append(out, item.Name)
			}
		}
		sort.Strings(out)
		return out, nil
	}
	var wrapped struct {
		Data []named `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(wrapped.Data))
	for _, item := range wrapped.Data {
		if strings.TrimSpace(item.Name) != "" {
			out = append(out, item.Name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (c *DataPlaneAPIClient) DeleteBackend(ctx context.Context, backendName string) error {
	version, err := c.ConfigurationVersion(ctx)
	if err != nil {
		return err
	}
	path := "/v3/services/haproxy/configuration/backends/" + url.PathEscape(backendName) + "?version=" + url.QueryEscape(version)
	_, err = c.do(ctx, http.MethodDelete, path, nil)
	if isHTTPStatus(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (c *DataPlaneAPIClient) do(ctx context.Context, method string, path string, payload any) ([]byte, error) {
	baseURL := strings.TrimRight(c.resolveURL(), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("dataplane base url is empty")
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(c.resolveUser(), c.resolvePass())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, &httpStatusError{
			statusCode: resp.StatusCode,
			message:    strings.TrimSpace(string(respBody)),
		}
	}
	return respBody, nil
}

type httpStatusError struct {
	statusCode int
	message    string
}

func (e *httpStatusError) Error() string {
	if e.message == "" {
		return fmt.Sprintf("dataplane api status %d", e.statusCode)
	}
	return fmt.Sprintf("dataplane api status %d: %s", e.statusCode, e.message)
}

func isHTTPStatus(err error, statusCode int) bool {
	if err == nil {
		return false
	}
	httpErr, ok := err.(*httpStatusError)
	return ok && httpErr.statusCode == statusCode
}

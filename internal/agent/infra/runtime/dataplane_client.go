package runtime

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

type dataPlaneTransaction struct {
	ID string `json:"id"`
}

type frontendSection struct {
	Name                     string                  `json:"name"`
	Mode                     string                  `json:"mode"`
	DefaultBackend           string                  `json:"default_backend"`
	Binds                    map[string]frontendBind `json:"binds"`
	ACLList                  []frontendACL           `json:"acl_list,omitempty"`
	BackendSwitchingRuleList []frontendSwitchRule    `json:"backend_switching_rule_list,omitempty"`
	HTTPAfterResponseRules   []httpAfterResponseRule `json:"http_after_response_rule_list,omitempty"`
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

type httpAfterResponseRule struct {
	Type     string `json:"type"`
	Action   string `json:"-"`
	Header   string `json:"hdr_name,omitempty"`
	Format   string `json:"hdr_format,omitempty"`
	Cond     string `json:"cond,omitempty"`
	CondTest string `json:"cond_test,omitempty"`
	Index    int    `json:"index"`
}

type backendSection struct {
	Name              string             `json:"name"`
	Mode              string             `json:"mode"`
	From              string             `json:"from,omitempty"`
	Balance           backendBalance     `json:"balance,omitempty"`
	HTTPResponseRules []httpResponseRule `json:"http_response_rule_list,omitempty"`
}

type backendBalance struct {
	Algorithm string `json:"algorithm"`
}

type httpResponseRule struct {
	Type     string `json:"type"`
	Action   string `json:"-"`
	Header   string `json:"hdr_name,omitempty"`
	Format   string `json:"hdr_format,omitempty"`
	Cond     string `json:"cond,omitempty"`
	CondTest string `json:"cond_test,omitempty"`
	Index    int    `json:"index"`
}

type backendServer struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Check     string `json:"check,omitempty"`
	Resolvers string `json:"resolvers,omitempty"`
	InitAddr  string `json:"init_addr,omitempty"`
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

func (c *DataPlaneAPIClient) ShowRawConfig(ctx context.Context) (string, error) {
	respBody, err := c.do(ctx, http.MethodGet, "/v3/services/haproxy/configuration/raw", nil)
	if err != nil {
		return "", err
	}
	return decodeRawConfigResponse(respBody)
}

func (c *DataPlaneAPIClient) ShowRawConfigInTransaction(ctx context.Context, transactionID string) (string, error) {
	path := c.configurationPath("/v3/services/haproxy/configuration/raw", "", transactionID, false)
	respBody, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	return decodeRawConfigResponse(respBody)
}

func (c *DataPlaneAPIClient) ReplaceFrontend(ctx context.Context, section frontendSection) error {
	section.HTTPAfterResponseRules = filterHTTPAfterResponseRules(section.HTTPAfterResponseRules)
	version, err := c.ConfigurationVersion(ctx)
	if err != nil {
		return err
	}
	path := c.configurationPath("/v3/services/haproxy/configuration/frontends/"+url.PathEscape(section.Name), version, "", true)
	if _, err := c.do(ctx, http.MethodPut, path, section); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := c.configurationPath("/v3/services/haproxy/configuration/frontends", version, "", true)
		_, err = c.do(ctx, http.MethodPost, createPath, section)
		return err
	}
	return nil
}

func (c *DataPlaneAPIClient) ReplaceFrontendInTransaction(ctx context.Context, transactionID string, section frontendSection) error {
	section.HTTPAfterResponseRules = filterHTTPAfterResponseRules(section.HTTPAfterResponseRules)
	path := c.configurationPath("/v3/services/haproxy/configuration/frontends/"+url.PathEscape(section.Name), "", transactionID, true)
	if _, err := c.do(ctx, http.MethodPut, path, section); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := c.configurationPath("/v3/services/haproxy/configuration/frontends", "", transactionID, true)
		_, err = c.do(ctx, http.MethodPost, createPath, section)
		return err
	}
	return nil
}

func filterHTTPAfterResponseRules(rules []httpAfterResponseRule) []httpAfterResponseRule {
	out := make([]httpAfterResponseRule, 0, len(rules))
	for _, rule := range rules {
		rule.Type = strings.TrimSpace(rule.Type)
		if rule.Type == "" {
			continue
		}
		if strings.TrimSpace(rule.Header) == "" {
			continue
		}
		rule.Cond = strings.TrimSpace(rule.Cond)
		rule.CondTest = strings.TrimSpace(rule.CondTest)
		if rule.CondTest == "" {
			continue
		}
		if strings.TrimSpace(rule.Format) == "" {
			continue
		}
		if strings.HasPrefix(rule.CondTest, "if ") {
			rule.Cond = "if"
			rule.CondTest = strings.TrimSpace(strings.TrimPrefix(rule.CondTest, "if "))
		} else if strings.HasPrefix(rule.CondTest, "unless ") {
			rule.Cond = "unless"
			rule.CondTest = strings.TrimSpace(strings.TrimPrefix(rule.CondTest, "unless "))
		}
		if rule.Cond != "if" && rule.Cond != "unless" {
			rule.Cond = "if"
		}
		if rule.CondTest == "" {
			continue
		}
		rule.Format = normalizeHAProxyFmt(rule.Format)
		out = append(out, rule)
	}
	for i := range out {
		out[i].Index = i
	}
	return out
}

func filterHTTPResponseRules(rules []httpResponseRule) []httpResponseRule {
	out := make([]httpResponseRule, 0, len(rules))
	for _, rule := range rules {
		rule.Type = strings.TrimSpace(rule.Type)
		if rule.Type == "" {
			continue
		}
		if strings.TrimSpace(rule.Header) == "" {
			continue
		}
		if strings.TrimSpace(rule.Format) == "" {
			continue
		}
		rule.Cond = strings.TrimSpace(rule.Cond)
		rule.CondTest = strings.TrimSpace(rule.CondTest)
		if strings.HasPrefix(rule.CondTest, "if ") {
			rule.Cond = "if"
			rule.CondTest = strings.TrimSpace(strings.TrimPrefix(rule.CondTest, "if "))
		} else if strings.HasPrefix(rule.CondTest, "unless ") {
			rule.Cond = "unless"
			rule.CondTest = strings.TrimSpace(strings.TrimPrefix(rule.CondTest, "unless "))
		}
		if rule.CondTest == "" {
			rule.Cond = ""
		} else if rule.Cond != "if" && rule.Cond != "unless" {
			rule.Cond = "if"
		}
		rule.Format = normalizeHAProxyFmt(rule.Format)
		out = append(out, rule)
	}
	for i := range out {
		out[i].Index = i
	}
	return out
}

func normalizeHAProxyFmt(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if isQuotedHAProxyString(value) {
		return value
	}
	if isHAProxyFmtExpression(value) {
		return value
	}
	if strings.ContainsAny(value, " \t;") {
		return strconv.Quote(value)
	}
	return value
}

func isQuotedHAProxyString(value string) bool {
	return len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"'
}

func isHAProxyFmtExpression(value string) bool {
	return len(value) >= 4 && strings.HasPrefix(value, "%[") && strings.HasSuffix(value, "]")
}

func decodeRawConfigResponse(respBody []byte) (string, error) {
	rawText := string(respBody)
	trimmed := strings.TrimSpace(rawText)
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var payload map[string]any
		if err := json.Unmarshal(respBody, &payload); err != nil {
			return "", err
		}
		switch value := payload["data"].(type) {
		case string:
			return value, nil
		}
		switch value := payload["_data"].(type) {
		case string:
			return value, nil
		}
	}
	if strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) {
		return strings.Trim(trimmed, `"`), nil
	}
	return rawText, nil
}

func (c *DataPlaneAPIClient) EnsureBackend(ctx context.Context, section backendSection) error {
	section.From = strings.TrimSpace(section.From)
	section.HTTPResponseRules = filterHTTPResponseRules(section.HTTPResponseRules)
	version, err := c.ConfigurationVersion(ctx)
	if err != nil {
		return err
	}
	path := c.configurationPath("/v3/services/haproxy/configuration/backends/"+url.PathEscape(section.Name), version, "", true)
	if _, err := c.do(ctx, http.MethodPut, path, section); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := c.configurationPath("/v3/services/haproxy/configuration/backends", version, "", true)
		_, err = c.do(ctx, http.MethodPost, createPath, section)
		return err
	}
	return nil
}

func (c *DataPlaneAPIClient) EnsureBackendInTransaction(ctx context.Context, transactionID string, section backendSection) error {
	section.From = strings.TrimSpace(section.From)
	section.HTTPResponseRules = filterHTTPResponseRules(section.HTTPResponseRules)
	path := c.configurationPath("/v3/services/haproxy/configuration/backends/"+url.PathEscape(section.Name), "", transactionID, true)
	if _, err := c.do(ctx, http.MethodPut, path, section); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := c.configurationPath("/v3/services/haproxy/configuration/backends", "", transactionID, true)
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
	path := c.configurationPath("/v3/services/haproxy/configuration/backends/"+url.PathEscape(backendName)+"/servers/"+url.PathEscape(server.Name), version, "", false)
	if _, err := c.do(ctx, http.MethodPut, path, server); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := c.configurationPath("/v3/services/haproxy/configuration/backends/"+url.PathEscape(backendName)+"/servers", version, "", false)
		_, err = c.do(ctx, http.MethodPost, createPath, server)
		return err
	}
	return nil
}

func (c *DataPlaneAPIClient) EnsureServerInTransaction(ctx context.Context, backendName string, transactionID string, server backendServer) error {
	path := c.configurationPath("/v3/services/haproxy/configuration/backends/"+url.PathEscape(backendName)+"/servers/"+url.PathEscape(server.Name), "", transactionID, false)
	if _, err := c.do(ctx, http.MethodPut, path, server); err != nil {
		if !isHTTPStatus(err, http.StatusNotFound) {
			return err
		}
		createPath := c.configurationPath("/v3/services/haproxy/configuration/backends/"+url.PathEscape(backendName)+"/servers", "", transactionID, false)
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
	path := c.configurationPath("/v3/services/haproxy/configuration/backends/"+url.PathEscape(backendName), version, "", false)
	_, err = c.do(ctx, http.MethodDelete, path, nil)
	if isHTTPStatus(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (c *DataPlaneAPIClient) DeleteBackendInTransaction(ctx context.Context, backendName string, transactionID string) error {
	path := c.configurationPath("/v3/services/haproxy/configuration/backends/"+url.PathEscape(backendName), "", transactionID, false)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	if isHTTPStatus(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (c *DataPlaneAPIClient) StartTransaction(ctx context.Context, version string) (string, error) {
	path := "/v3/services/haproxy/transactions?version=" + url.QueryEscape(version)
	body, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	var transaction dataPlaneTransaction
	if err := json.Unmarshal(body, &transaction); err != nil {
		return "", err
	}
	if strings.TrimSpace(transaction.ID) == "" {
		return "", fmt.Errorf("empty dataplane transaction id")
	}
	return transaction.ID, nil
}

func (c *DataPlaneAPIClient) CommitTransaction(ctx context.Context, transactionID string) error {
	_, err := c.do(ctx, http.MethodPut, "/v3/services/haproxy/transactions/"+url.PathEscape(transactionID), nil)
	return err
}

func (c *DataPlaneAPIClient) AbortTransaction(ctx context.Context, transactionID string) error {
	_, err := c.do(ctx, http.MethodDelete, "/v3/services/haproxy/transactions/"+url.PathEscape(transactionID), nil)
	if isHTTPStatus(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (c *DataPlaneAPIClient) configurationPath(basePath string, version string, transactionID string, fullSection bool) string {
	values := url.Values{}
	if strings.TrimSpace(transactionID) != "" {
		values.Set("transaction_id", transactionID)
	} else if strings.TrimSpace(version) != "" {
		values.Set("version", version)
	}
	if fullSection {
		values.Set("full_section", "true")
	}
	query := values.Encode()
	if query == "" {
		return basePath
	}
	return basePath + "?" + query
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

package infra

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDataPlaneClientStartTransactionUsesVersionQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v3/services/haproxy/transactions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("version"); got != "42" {
			t.Fatalf("expected version query 42, got %q", got)
		}
		_, _ = w.Write([]byte(`{"id":"tx-1"}`))
	}))
	defer server.Close()

	client := newDataPlaneAPIClient(func() string { return server.URL }, func() string { return "admin" }, func() string { return "secret" })
	transactionID, err := client.StartTransaction(context.Background(), "42")
	if err != nil {
		t.Fatalf("StartTransaction() error = %v", err)
	}
	if transactionID != "tx-1" {
		t.Fatalf("expected transaction id tx-1, got %q", transactionID)
	}
}

func TestDataPlaneClientTransactionWritesIncludeTransactionID(t *testing.T) {
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		requests = append(requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			body:   string(body),
		})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newDataPlaneAPIClient(func() string { return server.URL }, func() string { return "admin" }, func() string { return "secret" })
	ctx := context.Background()
	if err := client.EnsureBackendInTransaction(ctx, "tx-1", backendSection{Name: "be-api", Mode: "http"}); err != nil {
		t.Fatalf("EnsureBackendInTransaction() error = %v", err)
	}
	if err := client.EnsureServerInTransaction(ctx, "be-api", "tx-1", backendServer{
		Name:      "blue",
		Address:   "svc",
		Port:      8080,
		Check:     "enabled",
		Resolvers: managedProxyResolversName,
		InitAddr:  managedProxyInitAddrFallback,
	}); err != nil {
		t.Fatalf("EnsureServerInTransaction() error = %v", err)
	}
	if err := client.ReplaceFrontendInTransaction(ctx, "tx-1", frontendSection{
		Name: "ep_http",
		Mode: "http",
		HTTPAfterResponseRules: []httpAfterResponseRule{
			{
				Type:     "set-header",
				Action:   "set-header",
				Header:   "X-Test",
				Format:   "release-1",
				CondTest: "if test_acl_expr",
				Index:    0,
			},
		},
	}); err != nil {
		t.Fatalf("ReplaceFrontendInTransaction() error = %v", err)
	}
	if err := client.DeleteBackendInTransaction(ctx, "stale-api", "tx-1"); err != nil {
		t.Fatalf("DeleteBackendInTransaction() error = %v", err)
	}
	if err := client.CommitTransaction(ctx, "tx-1"); err != nil {
		t.Fatalf("CommitTransaction() error = %v", err)
	}
	if err := client.AbortTransaction(ctx, "tx-1"); err != nil {
		t.Fatalf("AbortTransaction() error = %v", err)
	}

	if len(requests) != 6 {
		t.Fatalf("expected 6 requests, got %d", len(requests))
	}
	assertTransactionRequest(t, requests[0], http.MethodPut, "/v3/services/haproxy/configuration/backends/be-api", "tx-1", false)
	assertTransactionRequest(t, requests[1], http.MethodPut, "/v3/services/haproxy/configuration/backends/be-api/servers/blue", "tx-1", false)
	assertServerPayload(t, requests[1], managedProxyResolversName, managedProxyInitAddrFallback)
	assertTransactionRequest(t, requests[2], http.MethodPut, "/v3/services/haproxy/configuration/frontends/ep_http", "tx-1", true)
	assertFrontendResponseRulePayload(t, requests[2], "set-header", "if test_acl_expr")
	assertTransactionRequest(t, requests[3], http.MethodDelete, "/v3/services/haproxy/configuration/backends/stale-api", "tx-1", false)
	assertTransactionLifecycleRequest(t, requests[4], http.MethodPut, "/v3/services/haproxy/transactions/tx-1")
	assertTransactionLifecycleRequest(t, requests[5], http.MethodDelete, "/v3/services/haproxy/transactions/tx-1")
}

func TestDataPlaneClientEnsureBackendInTransactionCreatesWhenMissing(t *testing.T) {
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		requests = append(requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			body:   string(body),
		})
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := newDataPlaneAPIClient(func() string { return server.URL }, func() string { return "admin" }, func() string { return "secret" })
	if err := client.EnsureBackendInTransaction(context.Background(), "tx-9", backendSection{Name: "be-new", Mode: "http"}); err != nil {
		t.Fatalf("EnsureBackendInTransaction() error = %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	assertTransactionRequest(t, requests[0], http.MethodPut, "/v3/services/haproxy/configuration/backends/be-new", "tx-9", false)
	assertTransactionRequest(t, requests[1], http.MethodPost, "/v3/services/haproxy/configuration/backends", "tx-9", false)
}

func TestDataPlaneClientShowRawConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v3/services/haproxy/configuration/raw" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("global\n  daemon\n"))
	}))
	defer server.Close()

	client := newDataPlaneAPIClient(func() string { return server.URL }, func() string { return "admin" }, func() string { return "secret" })
	configText, err := client.ShowRawConfig(context.Background())
	if err != nil {
		t.Fatalf("ShowRawConfig() error = %v", err)
	}
	if configText != "global\n  daemon\n" {
		t.Fatalf("unexpected config: %q", configText)
	}
}

func TestDataPlaneClientReplaceFrontendInTransactionFiltersInvalidResponseRules(t *testing.T) {
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		requests = append(requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			body:   string(body),
		})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newDataPlaneAPIClient(func() string { return server.URL }, func() string { return "admin" }, func() string { return "secret" })
	ctx := context.Background()
	if err := client.ReplaceFrontendInTransaction(ctx, "tx-1", frontendSection{
		Name: "ep_http",
		Mode: "http",
		HTTPAfterResponseRules: []httpAfterResponseRule{
			{
				Type:     "set-header",
				Action:   "set-header",
				Header:   "Set-Cookie",
				Cond:     "if",
				CondTest: "green_acl",
				Index:    0,
			},
			{
				Type:     "set-header",
				Action:   "set-header",
				Header:   "Set-Cookie",
				Format:   "release-1;Max-Age=600;Path=/;HttpOnly;SameSite=Lax",
				Cond:     "if",
				CondTest: "blue_acl",
				Index:    1,
			},
		},
	}); err != nil {
		t.Fatalf("ReplaceFrontendInTransaction() error = %v", err)
	}

	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(requests[0].body), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	rawRules, ok := payload["http_after_response_rule_list"].([]any)
	if !ok {
		t.Fatalf("expected http_after_response_rule_list array, got %#v", payload["http_after_response_rule_list"])
	}
	if len(rawRules) != 1 {
		t.Fatalf("expected invalid response rule to be filtered, got %d rules", len(rawRules))
	}
	firstRule, ok := rawRules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first rule object, got %#v", rawRules[0])
	}
	if got := firstRule["hdr_fmt"]; got != `"release-1;Max-Age=600;Path=/;HttpOnly;SameSite=Lax"` {
		t.Fatalf("expected quoted hdr_fmt for Set-Cookie, got %#v", got)
	}
	if got := firstRule["index"]; got != float64(0) {
		t.Fatalf("expected filtered rule reindexed to 0, got %#v", got)
	}
}

func assertServerPayload(t *testing.T, request recordedRequest, resolvers string, initAddr string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(request.body), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := payload["resolvers"]; got != resolvers {
		t.Fatalf("expected resolvers %q, got %#v", resolvers, got)
	}
	if got := payload["init_addr"]; got != initAddr {
		t.Fatalf("expected init_addr %q, got %#v", initAddr, got)
	}
}

func assertFrontendResponseRulePayload(t *testing.T, request recordedRequest, ruleType string, condTest string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(request.body), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	rawRules, ok := payload["http_after_response_rule_list"].([]any)
	if !ok || len(rawRules) == 0 {
		t.Fatalf("expected non-empty http_after_response_rule_list, got %#v", payload["http_after_response_rule_list"])
	}
	firstRule, ok := rawRules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first rule object, got %#v", rawRules[0])
	}
	if got := firstRule["type"]; got != ruleType {
		t.Fatalf("expected first response rule type %q, got %#v", ruleType, got)
	}
	if got := firstRule["cond"]; got != nil {
		t.Fatalf("expected first response rule cond to be omitted when cond_test is prefixed, got %#v", got)
	}
	if got := firstRule["cond_test"]; got != condTest {
		t.Fatalf("expected first response rule cond_test %q, got %#v", condTest, got)
	}
}

func assertTransactionRequest(t *testing.T, request recordedRequest, method string, path string, transactionID string, fullSection bool) {
	t.Helper()
	if request.method != method {
		t.Fatalf("expected method %s, got %s", method, request.method)
	}
	if request.path != path {
		t.Fatalf("expected path %s, got %s", path, request.path)
	}
	if got := request.query.Get("transaction_id"); got != transactionID {
		t.Fatalf("expected transaction_id %q, got %q", transactionID, got)
	}
	if got := request.query.Get("version"); got != "" {
		t.Fatalf("expected no version query, got %q", got)
	}
	if fullSection {
		if got := request.query.Get("full_section"); got != "true" {
			t.Fatalf("expected full_section=true, got %q", got)
		}
		return
	}
	if got := request.query.Get("full_section"); got != "" {
		t.Fatalf("expected no full_section query, got %q", got)
	}
}

func assertTransactionLifecycleRequest(t *testing.T, request recordedRequest, method string, path string) {
	t.Helper()
	if request.method != method {
		t.Fatalf("expected method %s, got %s", method, request.method)
	}
	if request.path != path {
		t.Fatalf("expected path %s, got %s", path, request.path)
	}
	if len(request.query) != 0 {
		t.Fatalf("expected no query params, got %#v", request.query)
	}
}

type recordedRequest struct {
	method string
	path   string
	query  url.Values
	body   string
}

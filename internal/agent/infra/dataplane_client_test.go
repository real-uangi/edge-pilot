package infra

import (
	"context"
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
		requests = append(requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
		})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newDataPlaneAPIClient(func() string { return server.URL }, func() string { return "admin" }, func() string { return "secret" })
	ctx := context.Background()
	if err := client.EnsureBackendInTransaction(ctx, "tx-1", backendSection{Name: "be-api", Mode: "http"}); err != nil {
		t.Fatalf("EnsureBackendInTransaction() error = %v", err)
	}
	if err := client.EnsureServerInTransaction(ctx, "be-api", "tx-1", backendServer{Name: "blue", Address: "svc", Port: 8080}); err != nil {
		t.Fatalf("EnsureServerInTransaction() error = %v", err)
	}
	if err := client.ReplaceFrontendInTransaction(ctx, "tx-1", frontendSection{Name: "ep_http", Mode: "http"}); err != nil {
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
	assertTransactionRequest(t, requests[2], http.MethodPut, "/v3/services/haproxy/configuration/frontends/ep_http", "tx-1", true)
	assertTransactionRequest(t, requests[3], http.MethodDelete, "/v3/services/haproxy/configuration/backends/stale-api", "tx-1", false)
	assertTransactionLifecycleRequest(t, requests[4], http.MethodPut, "/v3/services/haproxy/transactions/tx-1")
	assertTransactionLifecycleRequest(t, requests[5], http.MethodDelete, "/v3/services/haproxy/transactions/tx-1")
}

func TestDataPlaneClientEnsureBackendInTransactionCreatesWhenMissing(t *testing.T) {
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
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
}

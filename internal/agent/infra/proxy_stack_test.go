package infra

import (
	"context"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"errors"
	"reflect"
	"testing"

	"github.com/real-uangi/allingo/common/log"
)

func TestReconcileLockedUsesTransactionAndAppliesLiveSlotAfterCommit(t *testing.T) {
	callLog := make([]string, 0, 16)
	dataplane := &fakeManagedProxyDataplane{
		version: "42",
		txID:    "tx-1",
		backends: []string{
			"ep_default",
		},
		callLog: &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)

	if err := proxy.reconcileLocked(context.Background(), testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)); err != nil {
		t.Fatalf("reconcileLocked() error = %v", err)
	}

	expected := []string{
		"version",
		"start-transaction:42",
		"ensure-backend:be-api@tx-1",
		"ensure-server:be-api/blue@tx-1",
		"ensure-server:be-api/green@tx-1",
		"replace-frontend:ep_http@tx-1",
		"list-backends",
		"commit:tx-1",
		"enable:be-api/blue",
		"disable:be-api/green",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestReconcileLockedDeletesStaleBackendsInsideTransaction(t *testing.T) {
	callLog := make([]string, 0, 16)
	dataplane := &fakeManagedProxyDataplane{
		version: "7",
		txID:    "tx-9",
		backends: []string{
			"ep_default",
			"stale-api",
		},
		callLog: &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)

	if err := proxy.reconcileLocked(context.Background(), testProxySnapshotWithoutServices()); err != nil {
		t.Fatalf("reconcileLocked() error = %v", err)
	}

	expected := []string{
		"version",
		"start-transaction:7",
		"replace-frontend:ep_http@tx-9",
		"list-backends",
		"delete-backend:stale-api@tx-9",
		"commit:tx-9",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestReconcileLockedAbortsTransactionWhenFrontendUpdateFails(t *testing.T) {
	callLog := make([]string, 0, 16)
	expectedErr := errors.New("replace frontend failed")
	dataplane := &fakeManagedProxyDataplane{
		version:    "9",
		txID:       "tx-3",
		replaceErr: expectedErr,
		callLog:    &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)

	err := proxy.reconcileLocked(context.Background(), testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE))
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	expected := []string{
		"version",
		"start-transaction:9",
		"ensure-backend:be-api@tx-3",
		"ensure-server:be-api/blue@tx-3",
		"ensure-server:be-api/green@tx-3",
		"replace-frontend:ep_http@tx-3",
		"abort:tx-3",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestReconcileLockedAbortsTransactionWhenCommitFails(t *testing.T) {
	callLog := make([]string, 0, 16)
	expectedErr := errors.New("commit failed")
	dataplane := &fakeManagedProxyDataplane{
		version:   "10",
		txID:      "tx-4",
		backends:  []string{"ep_default"},
		commitErr: expectedErr,
		callLog:   &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)

	err := proxy.reconcileLocked(context.Background(), testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN))
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	expected := []string{
		"version",
		"start-transaction:10",
		"ensure-backend:be-api@tx-4",
		"ensure-server:be-api/blue@tx-4",
		"ensure-server:be-api/green@tx-4",
		"replace-frontend:ep_http@tx-4",
		"list-backends",
		"commit:tx-4",
		"abort:tx-4",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestReconcileLockedPreservesPrimaryErrorWhenAbortFails(t *testing.T) {
	callLog := make([]string, 0, 16)
	expectedErr := errors.New("replace frontend failed")
	dataplane := &fakeManagedProxyDataplane{
		version:    "11",
		txID:       "tx-5",
		replaceErr: expectedErr,
		abortErr:   errors.New("abort failed"),
		callLog:    &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)

	err := proxy.reconcileLocked(context.Background(), testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE))
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	expected := []string{
		"version",
		"start-transaction:11",
		"ensure-backend:be-api@tx-5",
		"ensure-server:be-api/blue@tx-5",
		"ensure-server:be-api/green@tx-5",
		"replace-frontend:ep_http@tx-5",
		"abort:tx-5",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func newTestManagedProxyRuntime(dataplane managedProxyDataPlaneAPI, runtime managedProxyRuntimeAPI) *ManagedProxyRuntime {
	return &ManagedProxyRuntime{
		cfg: &config.AgentRuntimeConfig{
			AgentID: "agent-1",
		},
		runtime:   runtime,
		dataplane: dataplane,
		logger:    log.NewStdLogger("agent.proxy-stack.test"),
	}
}

func testProxySnapshotWithService(slot grpcapi.Slot) *grpcapi.ProxyConfigSnapshot {
	return &grpcapi.ProxyConfigSnapshot{
		AgentId:        "agent-1",
		FrontendName:   "ep_http",
		DefaultBackend: "ep_default",
		BindPort:       80,
		Services: []*grpcapi.ProxyServiceConfig{
			{
				ServiceId:       "svc-1",
				ServiceKey:      "svc-a",
				RouteHost:       "api.example.com",
				RoutePathPrefix: "/",
				BackendName:     "be-api",
				BlueServerName:  "blue",
				GreenServerName: "green",
				ContainerPort:   8080,
				CurrentLiveSlot: slot,
			},
		},
	}
}

func testProxySnapshotWithoutServices() *grpcapi.ProxyConfigSnapshot {
	return &grpcapi.ProxyConfigSnapshot{
		AgentId:        "agent-1",
		FrontendName:   "ep_http",
		DefaultBackend: "ep_default",
		BindPort:       80,
	}
}

type fakeManagedProxyDataplane struct {
	version    string
	txID       string
	backends   []string
	replaceErr error
	commitErr  error
	abortErr   error
	callLog    *[]string
}

func (f *fakeManagedProxyDataplane) ConfigurationVersion(context.Context) (string, error) {
	*f.callLog = append(*f.callLog, "version")
	return f.version, nil
}

func (f *fakeManagedProxyDataplane) StartTransaction(_ context.Context, version string) (string, error) {
	*f.callLog = append(*f.callLog, "start-transaction:"+version)
	return f.txID, nil
}

func (f *fakeManagedProxyDataplane) CommitTransaction(_ context.Context, transactionID string) error {
	*f.callLog = append(*f.callLog, "commit:"+transactionID)
	return f.commitErr
}

func (f *fakeManagedProxyDataplane) AbortTransaction(_ context.Context, transactionID string) error {
	*f.callLog = append(*f.callLog, "abort:"+transactionID)
	return f.abortErr
}

func (f *fakeManagedProxyDataplane) ReplaceFrontendInTransaction(_ context.Context, transactionID string, section frontendSection) error {
	*f.callLog = append(*f.callLog, "replace-frontend:"+section.Name+"@"+transactionID)
	return f.replaceErr
}

func (f *fakeManagedProxyDataplane) EnsureBackendInTransaction(_ context.Context, transactionID string, section backendSection) error {
	*f.callLog = append(*f.callLog, "ensure-backend:"+section.Name+"@"+transactionID)
	return nil
}

func (f *fakeManagedProxyDataplane) EnsureServerInTransaction(_ context.Context, backendName string, transactionID string, server backendServer) error {
	*f.callLog = append(*f.callLog, "ensure-server:"+backendName+"/"+server.Name+"@"+transactionID)
	return nil
}

func (f *fakeManagedProxyDataplane) ListBackends(context.Context) ([]string, error) {
	*f.callLog = append(*f.callLog, "list-backends")
	return append([]string(nil), f.backends...), nil
}

func (f *fakeManagedProxyDataplane) DeleteBackendInTransaction(_ context.Context, backendName string, transactionID string) error {
	*f.callLog = append(*f.callLog, "delete-backend:"+backendName+"@"+transactionID)
	return nil
}

type fakeManagedProxyRuntime struct {
	callLog *[]string
}

func (f *fakeManagedProxyRuntime) SetServerAddress(context.Context, string, string, string, int) error {
	return nil
}

func (f *fakeManagedProxyRuntime) EnableServer(_ context.Context, backend string, server string) error {
	*f.callLog = append(*f.callLog, "enable:"+backend+"/"+server)
	return nil
}

func (f *fakeManagedProxyRuntime) DisableServer(_ context.Context, backend string, server string) error {
	*f.callLog = append(*f.callLog, "disable:"+backend+"/"+server)
	return nil
}

func (f *fakeManagedProxyRuntime) ShowStats(context.Context) ([]*grpcapi.BackendStatPoint, error) {
	return nil, nil
}

func (f *fakeManagedProxyRuntime) run(context.Context, string) (string, error) {
	return "", nil
}

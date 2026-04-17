package infra

import (
	"context"
	servicecatalogapp "edge-pilot/internal/servicecatalog/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/grpcapi"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
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
		"ensure-backend:be-api_blue@tx-1",
		"ensure-server:be-api_blue/blue@tx-1",
		"ensure-backend:be-api_green@tx-1",
		"ensure-server:be-api_green/green@tx-1",
		"replace-frontend:ep_http@tx-1",
		"list-backends",
		"commit:tx-1",
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
		"ensure-backend:be-api_blue@tx-3",
		"ensure-server:be-api_blue/blue@tx-3",
		"ensure-backend:be-api_green@tx-3",
		"ensure-server:be-api_green/green@tx-3",
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
		"ensure-backend:be-api_blue@tx-4",
		"ensure-server:be-api_blue/blue@tx-4",
		"ensure-backend:be-api_green@tx-4",
		"ensure-server:be-api_green/green@tx-4",
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
		"ensure-backend:be-api_blue@tx-5",
		"ensure-server:be-api_blue/blue@tx-5",
		"ensure-backend:be-api_green@tx-5",
		"ensure-server:be-api_green/green@tx-5",
		"replace-frontend:ep_http@tx-5",
		"abort:tx-5",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestReconcileLockedAbortsTransactionWhenEnsureServerFails(t *testing.T) {
	callLog := make([]string, 0, 16)
	expectedErr := errors.New("ensure server failed")
	dataplane := &fakeManagedProxyDataplane{
		version:         "13",
		txID:            "tx-7",
		ensureServerErr: expectedErr,
		callLog:         &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)

	err := proxy.reconcileLocked(context.Background(), testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE))
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	expected := []string{
		"version",
		"start-transaction:13",
		"ensure-backend:be-api_blue@tx-7",
		"ensure-server:be-api_blue/blue@tx-7",
		"abort:tx-7",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestFormatDataplaneFailureContextIncludesCommitFailureDetails(t *testing.T) {
	contextText := formatDataplaneFailureContext(dataplaneFailureContext{
		AgentID:        "agent-1",
		TransactionID:  "tx-4",
		Version:        "10",
		Frontend:       newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{}).frontendSection(testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN)),
		DefaultBackend: "ep_default",
		ServiceCount:   1,
		Backends: []backendSection{
			{Name: "be-api_blue", Mode: "http", Balance: backendBalance{Algorithm: "roundrobin"}},
			{Name: "be-api_green", Mode: "http", Balance: backendBalance{Algorithm: "roundrobin"}},
		},
		Servers: []dataplaneBackendServer{
			{Backend: "be-api_blue", Server: backendServer{Name: "blue", Address: "svc-a-blue", Port: 8080}},
			{Backend: "be-api_green", Server: backendServer{Name: "green", Address: "svc-a-green", Port: 8080}},
		},
		DesiredBackends: []string{"be-api_blue", "be-api_green", "ep_default"},
		StaleBackends:   []string{"stale-api"},
	})

	if !strings.Contains(contextText, `"transactionId":"tx-4"`) {
		t.Fatalf("expected transaction id in context, got %s", contextText)
	}
	if !strings.Contains(contextText, `"desiredBackends":["be-api_blue","be-api_green","ep_default"]`) {
		t.Fatalf("expected desired backends in context, got %s", contextText)
	}
	if !strings.Contains(contextText, `"staleBackends":["stale-api"]`) {
		t.Fatalf("expected stale backends in context, got %s", contextText)
	}
	if !strings.Contains(contextText, `"frontend":{"name":"ep_http"`) {
		t.Fatalf("expected frontend payload in context, got %s", contextText)
	}
}

func TestFormatDataplaneFailureContextIncludesFrontendDetails(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})
	contextText := formatDataplaneFailureContext(dataplaneFailureContext{
		AgentID:        "agent-1",
		TransactionID:  "tx-3",
		Version:        "9",
		Frontend:       proxy.frontendSection(testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)),
		DefaultBackend: "ep_default",
		ServiceCount:   1,
	})

	if !strings.Contains(contextText, `"http_after_response_rule_list"`) {
		t.Fatalf("expected response rules in frontend context, got %s", contextText)
	}
	if !strings.Contains(contextText, servicecatalogapp.CurrentReleaseIDHeaderName) {
		t.Fatalf("expected current release header in frontend context, got %s", contextText)
	}
}

func TestFormatDataplaneFailureContextIncludesServerDetails(t *testing.T) {
	contextText := formatDataplaneFailureContext(dataplaneFailureContext{
		AgentID:        "agent-1",
		TransactionID:  "tx-7",
		Version:        "11",
		Frontend:       frontendSection{Name: "ep_http"},
		DefaultBackend: "ep_default",
		ServiceCount:   1,
		Servers: []dataplaneBackendServer{
			{
				Backend: "be-api_blue",
				Server: backendServer{
					Name:      "blue",
					Address:   "svc-a-blue",
					Port:      8080,
					Check:     "enabled",
					Resolvers: managedProxyResolversName,
					InitAddr:  managedProxyInitAddrFallback,
				},
			},
		},
	})

	if !strings.Contains(contextText, `"backend":"be-api_blue"`) {
		t.Fatalf("expected backend name in server context, got %s", contextText)
	}
	if !strings.Contains(contextText, `"resolvers":"`+managedProxyResolversName+`"`) {
		t.Fatalf("expected server resolvers in context, got %s", contextText)
	}
	if !json.Valid([]byte(contextText)) {
		t.Fatalf("expected valid json context, got %s", contextText)
	}
}

func TestReconcileLockedPrecreatesServersWithResolversForEmptyInstances(t *testing.T) {
	callLog := make([]string, 0, 16)
	dataplane := &fakeManagedProxyDataplane{
		version: "12",
		txID:    "tx-6",
		backends: []string{
			"ep_default",
		},
		callLog: &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)

	if err := proxy.reconcileLocked(context.Background(), testProxySnapshotWithService(grpcapi.Slot_SLOT_UNSPECIFIED)); err != nil {
		t.Fatalf("reconcileLocked() error = %v", err)
	}

	if len(dataplane.serverConfigs) != 2 {
		t.Fatalf("expected 2 precreated servers, got %d", len(dataplane.serverConfigs))
	}
	for _, server := range dataplane.serverConfigs {
		if server.Resolvers != managedProxyResolversName {
			t.Fatalf("expected resolvers %q, got %q", managedProxyResolversName, server.Resolvers)
		}
		if server.InitAddr != managedProxyInitAddrFallback {
			t.Fatalf("expected init_addr %q, got %q", managedProxyInitAddrFallback, server.InitAddr)
		}
	}
	expected := []string{
		"version",
		"start-transaction:12",
		"ensure-backend:be-api_blue@tx-6",
		"ensure-server:be-api_blue/blue@tx-6",
		"ensure-backend:be-api_green@tx-6",
		"ensure-server:be-api_green/green@tx-6",
		"replace-frontend:ep_http@tx-6",
		"list-backends",
		"commit:tx-6",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestFrontendSectionAddsStickyPreviewAndDiagnosticRules(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})

	section := proxy.frontendSection(testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN))

	if len(section.BackendSwitchingRuleList) != 5 {
		t.Fatalf("expected 5 switching rules, got %d", len(section.BackendSwitchingRuleList))
	}
	if section.BackendSwitchingRuleList[0].Name != "be-api_blue" {
		t.Fatalf("expected blue preview backend first, got %q", section.BackendSwitchingRuleList[0].Name)
	}
	if section.BackendSwitchingRuleList[4].Name != "be-api_green" {
		t.Fatalf("expected live green backend fallback, got %q", section.BackendSwitchingRuleList[4].Name)
	}
	if len(section.HTTPAfterResponseRules) != 7 {
		t.Fatalf("expected 7 response rules, got %d", len(section.HTTPAfterResponseRules))
	}
	if section.HTTPAfterResponseRules[0].Header != "Set-Cookie" {
		t.Fatalf("expected preview response to set cookie, got %#v", section.HTTPAfterResponseRules[0])
	}
	if section.HTTPAfterResponseRules[0].Type != "add-header" {
		t.Fatalf("expected response rule type add-header, got %#v", section.HTTPAfterResponseRules[0])
	}
	if strings.TrimSpace(section.HTTPAfterResponseRules[0].Cond) != "" {
		t.Fatalf("expected response rule cond to be omitted, got %#v", section.HTTPAfterResponseRules[0])
	}
	if strings.Contains(section.HTTPAfterResponseRules[0].Format, "; ") {
		t.Fatalf("expected response rule hdr_fmt to avoid spaces after ';', got %#v", section.HTTPAfterResponseRules[0])
	}
	if section.HTTPAfterResponseRules[3].Header != servicecatalogapp.CurrentReleaseIDHeaderName {
		t.Fatalf("expected current release header, got %#v", section.HTTPAfterResponseRules[3])
	}
	if section.HTTPAfterResponseRules[3].Type != "set-header" {
		t.Fatalf("expected response rule type set-header, got %#v", section.HTTPAfterResponseRules[3])
	}
	if strings.TrimSpace(section.HTTPAfterResponseRules[3].Cond) != "" {
		t.Fatalf("expected response rule cond to be omitted, got %#v", section.HTTPAfterResponseRules[3])
	}
	if section.HTTPAfterResponseRules[3].CondTest == "if" || section.HTTPAfterResponseRules[3].CondTest == "unless" {
		t.Fatalf("expected response rule cond_test ACL expression, got %#v", section.HTTPAfterResponseRules[3])
	}
	if section.HTTPAfterResponseRules[6].Header != servicecatalogapp.LiveReleaseIDHeaderName {
		t.Fatalf("expected live release header, got %#v", section.HTTPAfterResponseRules[6])
	}
	if section.HTTPAfterResponseRules[6].Type != "set-header" {
		t.Fatalf("expected response rule type set-header, got %#v", section.HTTPAfterResponseRules[6])
	}
	for i, rule := range section.HTTPAfterResponseRules {
		if strings.TrimSpace(rule.Cond) != "" {
			t.Fatalf("response rule[%d] cond should be omitted when cond_test includes if/unless, got %#v", i, rule)
		}
		if strings.TrimSpace(rule.CondTest) == "" {
			t.Fatalf("response rule[%d] cond_test should not be empty, got %#v", i, rule)
		}
		if !strings.HasPrefix(rule.CondTest, "if ") && !strings.HasPrefix(rule.CondTest, "unless ") {
			t.Fatalf("response rule[%d] cond_test should include if/unless prefix, got %#v", i, rule)
		}
	}
}

func TestFrontendSectionSkipsInvalidResponseHeaderRules(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})

	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)
	snapshot.Services[0].GreenServerName = ""

	section := proxy.frontendSection(snapshot)

	if len(section.HTTPAfterResponseRules) != 5 {
		t.Fatalf("expected 5 response rules after filtering invalid green rules, got %d", len(section.HTTPAfterResponseRules))
	}
	for i, rule := range section.HTTPAfterResponseRules {
		if strings.TrimSpace(rule.Format) == "" {
			t.Fatalf("response rule[%d] should have non-empty hdr_fmt, got %#v", i, rule)
		}
		if rule.Index != i {
			t.Fatalf("response rule[%d] should be reindexed to %d, got %d", i, i, rule.Index)
		}
	}
	for _, rule := range section.HTTPAfterResponseRules {
		if rule.CondTest == aclName("svc-1", "host")+" "+aclName("svc-1", "path")+" "+aclName("svc-1", "query_green") {
			t.Fatalf("green preview response rule should be skipped when green release is invalid, got %#v", rule)
		}
	}
}

func TestProxyInspectNeedsBootstrapRefresh(t *testing.T) {
	runtime := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})
	expectedHash := runtime.bootstrapFilesHash()

	if !proxyInspectNeedsBootstrapRefresh(nil, expectedHash) {
		t.Fatal("expected nil inspect to require bootstrap refresh")
	}

	inspect := &dockerContainerInspect{}
	inspect.Config.Labels = map[string]string{
		proxyStackBootstrapLabelKey: expectedHash,
	}
	if proxyInspectNeedsBootstrapRefresh(inspect, expectedHash) {
		t.Fatal("expected matching bootstrap hash to skip refresh")
	}

	inspect.Config.Labels[proxyStackBootstrapLabelKey] = "outdated"
	if !proxyInspectNeedsBootstrapRefresh(inspect, expectedHash) {
		t.Fatal("expected mismatched bootstrap hash to require refresh")
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
				BlueServerName:  "release-blue",
				GreenServerName: "release-green",
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
	version          string
	txID             string
	backends         []string
	ensureBackendErr error
	ensureServerErr  error
	replaceErr       error
	commitErr        error
	abortErr         error
	callLog          *[]string
	serverConfigs    []backendServer
}

func (f *fakeManagedProxyDataplane) ConfigurationVersion(context.Context) (string, error) {
	*f.callLog = append(*f.callLog, "version")
	return f.version, nil
}

func (f *fakeManagedProxyDataplane) ShowRawConfig(context.Context) (string, error) {
	*f.callLog = append(*f.callLog, "show-raw-config")
	return "global\n  daemon\n", nil
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
	return f.ensureBackendErr
}

func (f *fakeManagedProxyDataplane) EnsureServerInTransaction(_ context.Context, backendName string, transactionID string, server backendServer) error {
	*f.callLog = append(*f.callLog, "ensure-server:"+backendName+"/"+server.Name+"@"+transactionID)
	f.serverConfigs = append(f.serverConfigs, server)
	return f.ensureServerErr
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

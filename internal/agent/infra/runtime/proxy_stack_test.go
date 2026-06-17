package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	agentdomain "github.com/real-uangi/edge-pilot/internal/agent/domain"
	servicecatalogapp "github.com/real-uangi/edge-pilot/internal/servicecatalog/application"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/grpcapi"

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
		"ensure-server:be-api_blue/srv@tx-1",
		"ensure-backend:be-api_green@tx-1",
		"ensure-server:be-api_green/srv@tx-1",
		"ensure-backend:ep_normalize@tx-1",
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
		"ensure-backend:ep_normalize@tx-9",
		"replace-frontend:ep_http@tx-9",
		"list-backends",
		"delete-backend:stale-api@tx-9",
		"commit:tx-9",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestSelectStaleManagedContainerIDsReturnsOnlyMissingServices(t *testing.T) {
	items := []*agentdomain.ManagedContainer{
		{ContainerRuntime: agentdomain.ContainerRuntime{ContainerID: "keep-live"}, ServiceKey: "svc-a"},
		{ContainerRuntime: agentdomain.ContainerRuntime{ContainerID: "remove-stale"}, ServiceKey: "svc-legacy", Image: "img-legacy"},
		{ContainerRuntime: agentdomain.ContainerRuntime{ContainerID: "skip-empty"}, ServiceKey: ""},
	}
	snapshot := &grpcapi.ProxyConfigSnapshot{
		Services: []*grpcapi.ProxyServiceConfig{
			{ServiceKey: "svc-a"},
			{ServiceKey: "svc-b"},
		},
	}

	stale, images := selectStaleManagedContainerIDs(items, snapshot)
	expected := []string{"remove-stale"}
	if !reflect.DeepEqual(stale, expected) {
		t.Fatalf("expected stale container ids %v, got %v", expected, stale)
	}
	if images["remove-stale"] != "img-legacy" {
		t.Fatalf("expected stale container image map, got %#v", images)
	}
}

func TestSelectStaleManagedContainerIDsReturnsNoneWhenSnapshotKeepsService(t *testing.T) {
	items := []*agentdomain.ManagedContainer{
		{ContainerRuntime: agentdomain.ContainerRuntime{ContainerID: "keep-1"}, ServiceKey: "svc-a"},
		{ContainerRuntime: agentdomain.ContainerRuntime{ContainerID: "keep-2"}, ServiceKey: "svc-b"},
	}
	snapshot := &grpcapi.ProxyConfigSnapshot{
		Services: []*grpcapi.ProxyServiceConfig{
			{ServiceKey: "svc-a"},
			{ServiceKey: "svc-b"},
		},
	}

	stale, images := selectStaleManagedContainerIDs(items, snapshot)
	if len(stale) != 0 {
		t.Fatalf("expected no stale container ids, got %v", stale)
	}
	if len(images) != 0 {
		t.Fatalf("expected no stale image map entries, got %#v", images)
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
		"ensure-server:be-api_blue/srv@tx-3",
		"ensure-backend:be-api_green@tx-3",
		"ensure-server:be-api_green/srv@tx-3",
		"ensure-backend:ep_normalize@tx-3",
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
		"ensure-backend:be-api_green@tx-4",
		"ensure-server:be-api_green/srv@tx-4",
		"ensure-backend:be-api_blue@tx-4",
		"ensure-server:be-api_blue/srv@tx-4",
		"ensure-backend:ep_normalize@tx-4",
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
		"ensure-server:be-api_blue/srv@tx-5",
		"ensure-backend:be-api_green@tx-5",
		"ensure-server:be-api_green/srv@tx-5",
		"ensure-backend:ep_normalize@tx-5",
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
		"ensure-server:be-api_blue/srv@tx-7",
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
			{Name: "be-api_blue", Mode: "http", Balance: &backendBalance{Algorithm: "roundrobin"}},
			{Name: "be-api_green", Mode: "http", Balance: &backendBalance{Algorithm: "roundrobin"}},
		},
		Servers: []dataplaneBackendServer{
			{Backend: "be-api_blue", Server: backendServer{Name: "srv", Address: agentdomain.ManagedContainerNameForRelease("svc-a", "release-blue"), Port: 8080}},
			{Backend: "be-api_green", Server: backendServer{Name: "srv", Address: agentdomain.ManagedContainerNameForRelease("svc-a", "release-green"), Port: 8080}},
		},
		DesiredBackends: []string{"be-api_blue", "be-api_green", "ep_default", "ep_normalize"},
		StaleBackends:   []string{"stale-api"},
	})

	if !strings.Contains(contextText, `"transactionId":"tx-4"`) {
		t.Fatalf("expected transaction id in context, got %s", contextText)
	}
	if !strings.Contains(contextText, `"desiredBackends":["be-api_blue","be-api_green","ep_default","ep_normalize"]`) {
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
		Backends: []backendSection{
			{
				Name:              "be-api_blue",
				Mode:              "http",
				From:              managedProxyDefaultsName,
				Balance:           &backendBalance{Algorithm: "roundrobin"},
				HTTPResponseRules: serviceBackendResponseRules("svc-a", "/", "release-blue", "release-blue", "", servicecatalogapp.ReleaseRoleLive),
			},
		},
	})

	if !strings.Contains(contextText, `"http_response_rule_list"`) {
		t.Fatalf("expected backend response rules in context, got %s", contextText)
	}
	if !strings.Contains(contextText, servicecatalogapp.CurrentReleaseIDHeaderName) {
		t.Fatalf("expected current release header in context, got %s", contextText)
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
					Address:   agentdomain.ManagedContainerNameForRelease("svc-a", "release-blue"),
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

func TestFormatDataplaneFailureContextIncludesRenderedFrontendConfig(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})
	frontend := proxy.frontendSection(testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE))
	contextText := formatDataplaneFailureContext(dataplaneFailureContext{
		AgentID:                "agent-1",
		TransactionID:          "tx-render",
		Version:                "12",
		Frontend:               frontend,
		IntendedFrontendConfig: renderIntendedFrontendConfig(frontend),
		DefaultBackend:         "ep_default",
		ServiceCount:           1,
	})

	if !strings.Contains(contextText, "\"intendedFrontendConfig\":\"frontend ep_http") {
		t.Fatalf("expected rendered frontend config in context, got %s", contextText)
	}
	if !strings.Contains(contextText, "use_backend ep_normalize") {
		t.Fatalf("expected rendered frontend config with normalize backend rule, got %s", contextText)
	}
	if !strings.Contains(contextText, "use_backend be-api_blue") {
		t.Fatalf("expected backend switching lines in rendered config, got %s", contextText)
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
		"ensure-server:be-api_blue/srv@tx-6",
		"ensure-backend:be-api_green@tx-6",
		"ensure-server:be-api_green/srv@tx-6",
		"ensure-backend:ep_normalize@tx-6",
		"replace-frontend:ep_http@tx-6",
		"list-backends",
		"commit:tx-6",
	}
	if !reflect.DeepEqual(callLog, expected) {
		t.Fatalf("unexpected call order: %#v", callLog)
	}
}

func TestFrontendSectionAddsStickyPreviewRoutingRules(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})

	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN)
	snapshot.Services[0].CandidateTrafficPercent = 30
	section := proxy.frontendSection(snapshot)

	if len(section.BackendSwitchingRuleList) != 8 {
		t.Fatalf("expected 8 switching rules, got %d", len(section.BackendSwitchingRuleList))
	}
	if section.BackendSwitchingRuleList[0].Name != normalizeBackendName {
		t.Fatalf("expected normalize backend first, got %q", section.BackendSwitchingRuleList[0].Name)
	}
	if section.BackendSwitchingRuleList[1].Name != normalizeBackendName {
		t.Fatalf("expected beta backend second, got %q", section.BackendSwitchingRuleList[1].Name)
	}
	if section.BackendSwitchingRuleList[2].Name != "be-api_blue" {
		t.Fatalf("expected blue preview backend third, got %q", section.BackendSwitchingRuleList[2].Name)
	}
	if section.BackendSwitchingRuleList[7].Name != "be-api_green" {
		t.Fatalf("expected live green backend fallback, got %q", section.BackendSwitchingRuleList[7].Name)
	}
	assertFrontendStaticAssetRules(t, section)
	if len(section.HTTPAfterResponseRules) != 0 {
		t.Fatalf("expected 0 after-response rules after normalize move to backend, got %d", len(section.HTTPAfterResponseRules))
	}
}

func TestFrontendSectionKeepsCandidateCookieRuleForStickyWhenSplitInactive(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})

	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN)
	snapshot.Services[0].CandidateTrafficPercent = 0
	section := proxy.frontendSection(snapshot)

	assertFrontendStaticAssetRules(t, section)
	foundCandidateCookieRule := false
	for _, rule := range section.BackendSwitchingRuleList {
		if rule.Name == "be-api_blue" && strings.Contains(rule.CondTest, "cookie_candidate") {
			foundCandidateCookieRule = true
			break
		}
	}
	if !foundCandidateCookieRule {
		t.Fatalf("expected candidate cookie override rule to be present for sticky when split inactive")
	}
}

func TestReconcileLockedBuildsBackendResponseHeaderRules(t *testing.T) {
	callLog := make([]string, 0, 16)
	dataplane := &fakeManagedProxyDataplane{
		version: "8",
		txID:    "tx-backend-rules",
		backends: []string{
			"ep_default",
		},
		callLog: &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)

	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN)
	snapshot.Services[0].CandidateTrafficPercent = 30
	if err := proxy.reconcileLocked(context.Background(), snapshot); err != nil {
		t.Fatalf("reconcileLocked() error = %v", err)
	}
	expectedNormalizeCondRoot := expectedNormalizeCondTest(snapshot.Services[0].RouteHost, servicecatalogapp.BuildStickyNormalizePath(snapshot.Services[0].RoutePathPrefix))

	blue := findBackendConfig(dataplane.backendConfigs, "be-api_blue")
	green := findBackendConfig(dataplane.backendConfigs, "be-api_green")
	if blue == nil || green == nil {
		t.Fatalf("expected both blue/green backend configs, got %#v", dataplane.backendConfigs)
	}
	if blue.From != managedProxyDefaultsName || green.From != managedProxyDefaultsName {
		t.Fatalf("expected backend from defaults %q, got blue=%q green=%q", managedProxyDefaultsName, blue.From, green.From)
	}
	assertBackendResponseRule(t, blue.HTTPResponseRules, "Set-Cookie", "release-blue")
	assertBackendResponseRule(t, green.HTTPResponseRules, "Set-Cookie", "release-green")
	assertBackendStickyCookieRuleSkipsStaticAssets(t, blue.HTTPResponseRules)
	assertBackendStickyCookieRuleSkipsStaticAssets(t, green.HTTPResponseRules)
	assertBackendResponseRuleExact(t, blue.HTTPResponseRules, servicecatalogapp.CurrentReleaseIDHeaderName, "release-blue")
	assertBackendResponseRuleExact(t, green.HTTPResponseRules, servicecatalogapp.CurrentReleaseIDHeaderName, "release-green")
	assertBackendResponseRuleExact(t, blue.HTTPResponseRules, servicecatalogapp.LiveReleaseIDHeaderName, "release-green")
	assertBackendResponseRuleExact(t, green.HTTPResponseRules, servicecatalogapp.LiveReleaseIDHeaderName, "release-green")
	assertBackendResponseRuleExact(t, blue.HTTPResponseRules, servicecatalogapp.BetaReleaseIDHeaderName, "release-blue")
	assertBackendResponseRuleExact(t, green.HTTPResponseRules, servicecatalogapp.BetaReleaseIDHeaderName, "release-blue")
	assertBackendResponseRuleExact(t, blue.HTTPResponseRules, servicecatalogapp.ReleaseRoleHeaderName, servicecatalogapp.ReleaseRoleCanary)
	assertBackendResponseRuleExact(t, green.HTTPResponseRules, servicecatalogapp.ReleaseRoleHeaderName, servicecatalogapp.ReleaseRoleLive)
	normalizeBackend := findBackendConfig(dataplane.backendConfigs, normalizeBackendName)
	if normalizeBackend == nil {
		t.Fatalf("expected normalize backend config, got %#v", dataplane.backendConfigs)
	}
	if normalizeBackend.HTTPRequestRules[0].Type != "set-var" {
		t.Fatalf("expected first normalize backend request rule set-var, got %#v", normalizeBackend.HTTPRequestRules[0])
	}
	if normalizeBackend.HTTPRequestRules[0].VarScope != "txn" || normalizeBackend.HTTPRequestRules[0].VarName != "ep_normalize_path" || normalizeBackend.HTTPRequestRules[0].VarExpr != "path" {
		t.Fatalf("expected set-var txn.ep_normalize_path=path, got %#v", normalizeBackend.HTTPRequestRules[0])
	}
	if len(normalizeBackend.HTTPRequestRules) != 4 {
		t.Fatalf("expected set-var + normalize return + beta return + fallback return in normalize backend request rules, got %#v", normalizeBackend.HTTPRequestRules)
	}
	conditionalReturnRule := normalizeBackend.HTTPRequestRules[1]
	if conditionalReturnRule.Type != "return" || conditionalReturnRule.ReturnStatusCode != 204 {
		t.Fatalf("expected conditional normalize return 204, got %#v", conditionalReturnRule)
	}
	assertNormalizeCondTestExact(t, conditionalReturnRule, expectedNormalizeCondRoot)
	assertNoLegacyVarMatchSyntax(t, conditionalReturnRule)
	assertNormalizeStaticNoCacheReturnHeaders(t, conditionalReturnRule.ReturnHeaders)
	assertReturnHeaderExact(t, conditionalReturnRule.ReturnHeaders, servicecatalogapp.CurrentReleaseIDHeaderName, "release-green")
	assertReturnHeaderExact(t, conditionalReturnRule.ReturnHeaders, servicecatalogapp.LiveReleaseIDHeaderName, "release-green")
	assertReturnHeaderExact(t, conditionalReturnRule.ReturnHeaders, servicecatalogapp.ReleaseRoleHeaderName, servicecatalogapp.ReleaseRoleLive)
	assertReturnHeaderContains(t, conditionalReturnRule.ReturnHeaders, "Set-Cookie", "release-green")
	fallbackReturnRule := normalizeBackend.HTTPRequestRules[3]
	if fallbackReturnRule.Type != "return" || fallbackReturnRule.ReturnStatusCode != 204 {
		t.Fatalf("expected fallback normalize return 204, got %#v", fallbackReturnRule)
	}
	if strings.TrimSpace(fallbackReturnRule.Cond) != "" || strings.TrimSpace(fallbackReturnRule.CondTest) != "" {
		t.Fatalf("expected fallback normalize return rule to be unconditional, got %#v", fallbackReturnRule)
	}
	assertNormalizeStaticNoCacheReturnHeaders(t, fallbackReturnRule.ReturnHeaders)
	if findReturnHeader(fallbackReturnRule.ReturnHeaders, "Set-Cookie") != nil {
		t.Fatalf("fallback normalize return rule should not include Set-Cookie, got %#v", fallbackReturnRule.ReturnHeaders)
	}
	blueServer := findServerEntry(dataplane.serverEntries, "be-api_blue")
	greenServer := findServerEntry(dataplane.serverEntries, "be-api_green")
	if blueServer == nil || greenServer == nil {
		t.Fatalf("expected both backend servers, got %#v", dataplane.serverEntries)
	}
	if blueServer.Server.Address != agentdomain.ManagedContainerNameForRelease("svc-a", "release-blue") {
		t.Fatalf("expected be-api_blue server address by release id, got %q", blueServer.Server.Address)
	}
	if greenServer.Server.Address != agentdomain.ManagedContainerNameForRelease("svc-a", "release-green") {
		t.Fatalf("expected be-api_green server address by release id, got %q", greenServer.Server.Address)
	}
	for _, backend := range []backendSection{*blue, *green} {
		for i, rule := range backend.HTTPResponseRules {
			if rule.Index != i {
				t.Fatalf("backend %s rule[%d] should be reindexed, got %d", backend.Name, i, rule.Index)
			}
			if strings.TrimSpace(rule.Type) != strings.TrimSpace(rule.Action) {
				t.Fatalf("backend %s rule[%d] type should equal action, got %#v", backend.Name, i, rule)
			}
			if strings.EqualFold(strings.TrimSpace(rule.Header), "Set-Cookie") {
				switch strings.TrimSpace(rule.Type) {
				case "add-header":
					if strings.TrimSpace(rule.Cond) != "unless" || strings.TrimSpace(rule.CondTest) != staticAssetVarCondTest() {
						t.Fatalf("backend %s Set-Cookie add rule should skip static assets, got %#v", backend.Name, rule)
					}
				case "del-header":
					if strings.TrimSpace(rule.Cond) != "if" || strings.TrimSpace(rule.CondTest) != staticAssetVarCondTest() || strings.TrimSpace(rule.Format) != "" {
						t.Fatalf("backend %s Set-Cookie delete rule should clear static assets, got %#v", backend.Name, rule)
					}
				default:
					t.Fatalf("backend %s unexpected Set-Cookie rule type, got %#v", backend.Name, rule)
				}
				continue
			}
			if strings.TrimSpace(rule.Cond) != "" || strings.TrimSpace(rule.CondTest) != "" {
				t.Fatalf("backend %s rule[%d] should be unconditional, got %#v", backend.Name, i, rule)
			}
		}
	}
}

func TestStaticAssetPathRegexMatchesExpectedExtensions(t *testing.T) {
	if got := staticAssetRequestRules(); len(got) != 2 {
		t.Fatalf("expected two static asset request rules, got %#v", got)
	}

	matcher := regexp.MustCompile("(?i)" + staticAssetPathRegex)
	staticPaths := []string{
		"/assets/app.js",
		"/assets/app.mjs",
		"/assets/app.css",
		"/assets/app.js.map",
		"/assets/app.wasm",
		"/assets/logo.svg",
		"/assets/font.woff2",
		"/favicon.ico",
		"/site.webmanifest",
	}
	for _, path := range staticPaths {
		if !matcher.MatchString(path) {
			t.Fatalf("expected static asset regex to match %q", path)
		}
	}

	dynamicPaths := []string{
		"/",
		"/dashboard",
		"/api/releases",
		"/assets/app",
		"/assets/js/app",
	}
	for _, path := range dynamicPaths {
		if matcher.MatchString(path) {
			t.Fatalf("expected static asset regex not to match %q", path)
		}
	}
}

func TestFrontendSectionNormalizeRuleUsesServicePath(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})

	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)
	snapshot.Services[0].RoutePathPrefix = "/v1"
	section := proxy.frontendSection(snapshot)

	foundNormalizePathACL := false
	for _, acl := range section.ACLList {
		if strings.Contains(acl.Name, "normalize_path") {
			foundNormalizePathACL = true
			if acl.Value != "-i /v1/__ep/normalize" {
				t.Fatalf("expected normalize path acl value -i /v1/__ep/normalize, got %q", acl.Value)
			}
		}
	}
	if !foundNormalizePathACL {
		t.Fatal("expected normalize path acl")
	}
}

func TestFrontendSectionBetaRuleUsesServicePath(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})

	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)
	snapshot.Services[0].RoutePathPrefix = "/v1"
	section := proxy.frontendSection(snapshot)

	foundBetaPathACL := false
	for _, acl := range section.ACLList {
		if strings.Contains(acl.Name, "beta_path") {
			foundBetaPathACL = true
			if acl.Value != "-i /v1/__ep/beta" {
				t.Fatalf("expected beta path acl value -i /v1/__ep/beta, got %q", acl.Value)
			}
		}
	}
	if !foundBetaPathACL {
		t.Fatal("expected beta path acl")
	}
}

func TestFrontendSectionRoutesBetaPathWhenCandidateMissing(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})

	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)
	snapshot.Services[0].CandidateReleaseId = ""
	snapshot.Services[0].CandidateBackendName = ""
	section := proxy.frontendSection(snapshot)

	for _, rule := range section.BackendSwitchingRuleList {
		if rule.Name == normalizeBackendName && strings.Contains(rule.CondTest, "beta_path") {
			return
		}
	}
	t.Fatalf("expected beta path to route to normalize backend fallback, got %#v", section.BackendSwitchingRuleList)
}

func TestFrontendSectionHostACLUsesRouteHosts(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})

	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)
	snapshot.Services[0].RouteHosts = []string{"api.example.com", "api-alt.example.com"}
	section := proxy.frontendSection(snapshot)

	for _, acl := range section.ACLList {
		if strings.Contains(acl.Name, "host") {
			if acl.Value != "-i api.example.com api-alt.example.com" {
				t.Fatalf("expected multi-host acl value, got %q", acl.Value)
			}
			return
		}
	}
	t.Fatal("expected host acl")
}

func TestFrontendSectionOrdersLongerPathBeforeSharedAliasRoot(t *testing.T) {
	proxy := newTestManagedProxyRuntime(&fakeManagedProxyDataplane{}, &fakeManagedProxyRuntime{})
	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)
	snapshot.Services = []*grpcapi.ProxyServiceConfig{
		{
			ServiceId:            "svc-root",
			ServiceKey:           "svc-root",
			RouteHost:            "a.example.com",
			RouteHosts:           []string{"a.example.com", "shared.example.com"},
			RoutePathPrefix:      "/",
			LiveBackendName:      "be-root",
			LiveReleaseId:        "release-root",
			ContainerPort:        8080,
			CurrentLiveSlot:      grpcapi.Slot_SLOT_BLUE,
			CandidateBackendName: "",
		},
		{
			ServiceId:            "svc-api",
			ServiceKey:           "svc-api",
			RouteHost:            "b.example.com",
			RouteHosts:           []string{"b.example.com", "shared.example.com"},
			RoutePathPrefix:      "/api",
			LiveBackendName:      "be-api",
			LiveReleaseId:        "release-api",
			ContainerPort:        8080,
			CurrentLiveSlot:      grpcapi.Slot_SLOT_BLUE,
			CandidateBackendName: "",
		},
	}

	section := proxy.frontendSection(snapshot)
	if len(section.BackendSwitchingRuleList) < 2 {
		t.Fatalf("expected backend switching rules, got %#v", section.BackendSwitchingRuleList)
	}
	if !strings.Contains(section.BackendSwitchingRuleList[0].CondTest, "svc_api") {
		t.Fatalf("expected longer /api route first for shared alias, got %#v", section.BackendSwitchingRuleList[:2])
	}
}

func TestNormalizeBackendRequestRulesUseRouteHosts(t *testing.T) {
	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN)
	snapshot.Services[0].RouteHosts = []string{"api.example.com", "api-alt.example.com"}
	rules := normalizeBackendRequestRules(snapshot.Services)

	if len(rules) != 4 {
		t.Fatalf("expected set-var + normalize return + beta return + fallback return, got %#v", rules)
	}
	expected := expectedNormalizeCondTest("api.example.com api-alt.example.com", servicecatalogapp.BuildStickyNormalizePath(snapshot.Services[0].RoutePathPrefix))
	assertNormalizeCondTestExact(t, rules[1], expected)
}

func TestBetaBackendRequestRulesSetCandidateCookie(t *testing.T) {
	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN)
	rules := normalizeBackendRequestRules(snapshot.Services)

	if len(rules) != 4 {
		t.Fatalf("expected set-var + normalize return + beta return + fallback return, got %#v", rules)
	}

	betaRule := rules[2]
	if betaRule.Type != "return" || betaRule.ReturnStatusCode != 204 {
		t.Fatalf("expected beta return 204, got %#v", betaRule)
	}
	expectedBetaCond := expectedNormalizeCondTest(snapshot.Services[0].RouteHost, servicecatalogapp.BuildStickyBetaPath(snapshot.Services[0].RoutePathPrefix))
	assertNormalizeCondTestExact(t, betaRule, expectedBetaCond)
	assertReturnHeaderContains(t, betaRule.ReturnHeaders, "Set-Cookie", "release-blue")
	assertReturnHeaderExact(t, betaRule.ReturnHeaders, servicecatalogapp.CurrentReleaseIDHeaderName, "release-blue")
	assertReturnHeaderExact(t, betaRule.ReturnHeaders, servicecatalogapp.LiveReleaseIDHeaderName, "release-green")
	assertReturnHeaderExact(t, betaRule.ReturnHeaders, servicecatalogapp.BetaReleaseIDHeaderName, "release-blue")
	assertReturnHeaderExact(t, betaRule.ReturnHeaders, servicecatalogapp.ReleaseRoleHeaderName, servicecatalogapp.ReleaseRoleHistorical)
}

func TestBetaBackendRequestRulesUseCanaryRoleWhenSplitActive(t *testing.T) {
	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_GREEN)
	snapshot.Services[0].CandidateTrafficPercent = 30
	rules := normalizeBackendRequestRules(snapshot.Services)

	if len(rules) != 4 {
		t.Fatalf("expected set-var + normalize return + beta return + fallback return, got %#v", rules)
	}
	betaRule := rules[2]
	assertReturnHeaderExact(t, betaRule.ReturnHeaders, servicecatalogapp.ReleaseRoleHeaderName, servicecatalogapp.ReleaseRoleCanary)
}

func TestBetaBackendRequestRulesSkipBetaWhenCandidateMissing(t *testing.T) {
	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)
	snapshot.Services[0].CandidateReleaseId = ""
	snapshot.Services[0].CandidateBackendName = ""
	rules := normalizeBackendRequestRules(snapshot.Services)

	if len(rules) != 3 {
		t.Fatalf("expected set-var + normalize return + fallback return, got %#v", rules)
	}
	for _, rule := range rules {
		if findReturnHeader(rule.ReturnHeaders, servicecatalogapp.BetaReleaseIDHeaderName) != nil {
			t.Fatalf("expected no beta release header without candidate, got %#v", rule)
		}
	}
}

func TestReconcileLockedFiltersInvalidBackendResponseHeaderRules(t *testing.T) {
	callLog := make([]string, 0, 16)
	dataplane := &fakeManagedProxyDataplane{
		version: "8",
		txID:    "tx-filter-backend-rules",
		backends: []string{
			"ep_default",
		},
		callLog: &callLog,
	}
	runtime := &fakeManagedProxyRuntime{callLog: &callLog}
	proxy := newTestManagedProxyRuntime(dataplane, runtime)
	snapshot := testProxySnapshotWithService(grpcapi.Slot_SLOT_BLUE)
	snapshot.Services[0].CandidateReleaseId = ""
	snapshot.Services[0].RoutePathPrefix = "/v1"

	if err := proxy.reconcileLocked(context.Background(), snapshot); err != nil {
		t.Fatalf("reconcileLocked() error = %v", err)
	}
	expectedNormalizeCondPrefixed := expectedNormalizeCondTest(snapshot.Services[0].RouteHost, servicecatalogapp.BuildStickyNormalizePath(snapshot.Services[0].RoutePathPrefix))

	blue := findBackendConfig(dataplane.backendConfigs, "be-api_blue")
	green := findBackendConfig(dataplane.backendConfigs, "be-api_green")
	if blue == nil || green == nil {
		t.Fatalf("expected both blue/green backend configs, got %#v", dataplane.backendConfigs)
	}
	if len(blue.HTTPResponseRules) != 5 {
		t.Fatalf("expected blue backend keep 5 response rules, got %d", len(blue.HTTPResponseRules))
	}
	if len(green.HTTPResponseRules) != 2 {
		t.Fatalf("expected green backend keep Set-Cookie cleanup and live release header rules, got %d", len(green.HTTPResponseRules))
	}
	if findBackendRuleByType(green.HTTPResponseRules, "add-header", "Set-Cookie") != nil {
		t.Fatalf("green backend sticky cookie add rule should be filtered when release id is empty, got %#v", green.HTTPResponseRules)
	}
	assertBackendStaticCookieCleanupRule(t, green.HTTPResponseRules)
	if findBackendRule(green.HTTPResponseRules, servicecatalogapp.CurrentReleaseIDHeaderName) != nil {
		t.Fatalf("green backend current release rule should be filtered when release id is empty, got %#v", green.HTTPResponseRules)
	}
	assertBackendResponseRuleExact(t, green.HTTPResponseRules, servicecatalogapp.LiveReleaseIDHeaderName, "release-blue")
	normalizeBackend := findBackendConfig(dataplane.backendConfigs, normalizeBackendName)
	if normalizeBackend == nil {
		t.Fatalf("expected normalize backend in filtered test, got %#v", dataplane.backendConfigs)
	}
	if normalizeBackend.HTTPRequestRules[0].Type != "set-var" {
		t.Fatalf("expected first normalize backend request rule set-var, got %#v", normalizeBackend.HTTPRequestRules[0])
	}
	if len(normalizeBackend.HTTPRequestRules) != 3 {
		t.Fatalf("expected set-var + service return + fallback return in normalize backend request rules, got %#v", normalizeBackend.HTTPRequestRules)
	}
	conditionalReturnRule := normalizeBackend.HTTPRequestRules[1]
	if conditionalReturnRule.Type != "return" || conditionalReturnRule.ReturnStatusCode != 204 {
		t.Fatalf("expected conditional normalize return 204, got %#v", conditionalReturnRule)
	}
	assertNormalizeCondTestExact(t, conditionalReturnRule, expectedNormalizeCondPrefixed)
	assertNoLegacyVarMatchSyntax(t, conditionalReturnRule)
	assertNormalizeStaticNoCacheReturnHeaders(t, conditionalReturnRule.ReturnHeaders)
	assertReturnHeaderContains(t, conditionalReturnRule.ReturnHeaders, "Set-Cookie", "release-blue")
	fallbackReturnRule := normalizeBackend.HTTPRequestRules[len(normalizeBackend.HTTPRequestRules)-1]
	if fallbackReturnRule.Type != "return" || fallbackReturnRule.ReturnStatusCode != 204 {
		t.Fatalf("expected fallback normalize return 204, got %#v", fallbackReturnRule)
	}
	if strings.TrimSpace(fallbackReturnRule.Cond) != "" || strings.TrimSpace(fallbackReturnRule.CondTest) != "" {
		t.Fatalf("expected fallback normalize return rule to be unconditional, got %#v", fallbackReturnRule)
	}
	assertNormalizeStaticNoCacheReturnHeaders(t, fallbackReturnRule.ReturnHeaders)
	if findReturnHeader(fallbackReturnRule.ReturnHeaders, "Set-Cookie") != nil {
		t.Fatalf("fallback normalize return rule should not include Set-Cookie, got %#v", fallbackReturnRule.ReturnHeaders)
	}
	blueServer := findServerEntry(dataplane.serverEntries, "be-api_blue")
	greenServer := findServerEntry(dataplane.serverEntries, "be-api_green")
	if blueServer == nil || greenServer == nil {
		t.Fatalf("expected both backend servers, got %#v", dataplane.serverEntries)
	}
	if blueServer.Server.Address != agentdomain.ManagedContainerNameForRelease("svc-a", "release-blue") {
		t.Fatalf("expected be-api_blue server address by release id, got %q", blueServer.Server.Address)
	}
	if greenServer.Server.Address != agentdomain.ManagedContainerName("svc-a", grpcapi.Slot_SLOT_GREEN) {
		t.Fatalf("expected candidate backend server address fallback by slot, got %q", greenServer.Server.Address)
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

func findBackendConfig(backends []backendSection, name string) *backendSection {
	for i := range backends {
		if backends[i].Name == name {
			return &backends[i]
		}
	}
	return nil
}

func findServerEntry(items []dataplaneBackendServer, backend string) *dataplaneBackendServer {
	for i := range items {
		if items[i].Backend == backend {
			return &items[i]
		}
	}
	return nil
}

func findBackendRule(rules []httpResponseRule, header string) *httpResponseRule {
	for i := range rules {
		if strings.EqualFold(strings.TrimSpace(rules[i].Header), strings.TrimSpace(header)) {
			return &rules[i]
		}
	}
	return nil
}

func findBackendRuleByType(rules []httpResponseRule, ruleType string, header string) *httpResponseRule {
	for i := range rules {
		if strings.TrimSpace(rules[i].Type) == strings.TrimSpace(ruleType) && strings.EqualFold(strings.TrimSpace(rules[i].Header), strings.TrimSpace(header)) {
			return &rules[i]
		}
	}
	return nil
}

func assertFrontendStaticAssetRules(t *testing.T, section frontendSection) {
	t.Helper()
	foundStaticACL := false
	for _, acl := range section.ACLList {
		if strings.TrimSpace(acl.Name) == staticAssetPathACLName {
			foundStaticACL = true
			if strings.TrimSpace(acl.Criterion) != "path" || strings.TrimSpace(acl.Value) != "-m reg -i "+staticAssetPathRegex {
				t.Fatalf("expected static asset path ACL, got %#v", acl)
			}
			break
		}
	}
	if !foundStaticACL {
		t.Fatalf("expected static asset path ACL, got %#v", section.ACLList)
	}
	if len(section.HTTPRequestRules) != 2 {
		t.Fatalf("expected two static asset request rules, got %#v", section.HTTPRequestRules)
	}
	expected := []struct {
		expr string
		cond string
		test string
	}{
		{expr: "bool(false)"},
		{expr: "bool(true)", cond: "if", test: staticAssetPathACLName},
	}
	for i, item := range expected {
		rule := section.HTTPRequestRules[i]
		if strings.TrimSpace(rule.Type) != "set-var" || strings.TrimSpace(rule.VarScope) != "txn" || strings.TrimSpace(rule.VarName) != staticAssetTxnVarName {
			t.Fatalf("expected static asset set-var rule[%d], got %#v", i, rule)
		}
		if strings.TrimSpace(rule.VarExpr) != item.expr || strings.TrimSpace(rule.Cond) != item.cond || strings.TrimSpace(rule.CondTest) != item.test {
			t.Fatalf("expected static asset set-var rule[%d] expr=%q cond=%q condTest=%q, got %#v", i, item.expr, item.cond, item.test, rule)
		}
	}
}

func assertBackendResponseRule(t *testing.T, rules []httpResponseRule, header string, expectedContains string) {
	t.Helper()
	rule := findBackendRule(rules, header)
	if rule == nil {
		t.Fatalf("expected response header rule %q, got %#v", header, rules)
	}
	if !strings.Contains(rule.Format, expectedContains) {
		t.Fatalf("expected response header %q format contains %q, got %#v", header, expectedContains, *rule)
	}
}

func assertBackendStickyCookieRuleSkipsStaticAssets(t *testing.T, rules []httpResponseRule) {
	t.Helper()
	rule := findBackendRuleByType(rules, "add-header", "Set-Cookie")
	if rule == nil {
		t.Fatalf("expected Set-Cookie response rule, got %#v", rules)
	}
	if strings.TrimSpace(rule.Cond) != "unless" {
		t.Fatalf("expected Set-Cookie response rule cond unless, got %#v", *rule)
	}
	if strings.TrimSpace(rule.CondTest) != staticAssetVarCondTest() {
		t.Fatalf("expected Set-Cookie response rule static asset cond %q, got %#v", staticAssetVarCondTest(), *rule)
	}
	assertBackendStaticCookieCleanupRule(t, rules)
}

func assertBackendStaticCookieCleanupRule(t *testing.T, rules []httpResponseRule) {
	t.Helper()
	rule := findBackendRuleByType(rules, "del-header", "Set-Cookie")
	if rule == nil {
		t.Fatalf("expected Set-Cookie static asset cleanup rule, got %#v", rules)
	}
	if strings.TrimSpace(rule.Format) != "" {
		t.Fatalf("expected Set-Cookie cleanup rule without format, got %#v", *rule)
	}
	if strings.TrimSpace(rule.Cond) != "if" {
		t.Fatalf("expected Set-Cookie cleanup rule cond if, got %#v", *rule)
	}
	if strings.TrimSpace(rule.CondTest) != staticAssetVarCondTest() {
		t.Fatalf("expected Set-Cookie cleanup rule static asset cond %q, got %#v", staticAssetVarCondTest(), *rule)
	}
}

func assertBackendResponseRuleExact(t *testing.T, rules []httpResponseRule, header string, expected string) {
	t.Helper()
	rule := findBackendRule(rules, header)
	if rule == nil {
		t.Fatalf("expected response header rule %q, got %#v", header, rules)
	}
	if strings.TrimSpace(rule.Format) != strings.TrimSpace(expected) {
		t.Fatalf("expected response header %q format %q, got %#v", header, expected, *rule)
	}
}

func expectedNormalizeCondTest(host string, normalizePath string) string {
	return "{ req.hdr(host) -i " + host + " } { var(txn.ep_normalize_path) -m str -i " + normalizePath + " }"
}

func assertNormalizeCondTestExact(t *testing.T, rule httpRequestRule, expected string) {
	t.Helper()
	if strings.TrimSpace(rule.CondTest) != strings.TrimSpace(expected) {
		t.Fatalf("expected normalize condTest %q, got %#v", expected, rule)
	}
}

func assertNoLegacyVarMatchSyntax(t *testing.T, rule httpRequestRule) {
	t.Helper()
	if strings.Contains(rule.CondTest, "var(txn.ep_normalize_path) -i ") {
		t.Fatalf("expected normalize condTest to avoid legacy var matcher syntax, got %#v", rule)
	}
}

func findReturnHeader(headers []returnHeader, name string) *returnHeader {
	for i := range headers {
		if strings.EqualFold(strings.TrimSpace(headers[i].Name), strings.TrimSpace(name)) {
			return &headers[i]
		}
	}
	return nil
}

func assertReturnHeaderExact(t *testing.T, headers []returnHeader, name string, expected string) {
	t.Helper()
	header := findReturnHeader(headers, name)
	if header == nil {
		t.Fatalf("expected return header %q, got %#v", name, headers)
	}
	if strings.TrimSpace(header.Format) != strings.TrimSpace(expected) {
		t.Fatalf("expected return header %q format %q, got %#v", name, expected, *header)
	}
}

func assertReturnHeaderContains(t *testing.T, headers []returnHeader, name string, expectedContains string) {
	t.Helper()
	header := findReturnHeader(headers, name)
	if header == nil {
		t.Fatalf("expected return header %q, got %#v", name, headers)
	}
	if !strings.Contains(strings.TrimSpace(header.Format), strings.TrimSpace(expectedContains)) {
		t.Fatalf("expected return header %q format contains %q, got %#v", name, expectedContains, *header)
	}
}

func assertNormalizeStaticNoCacheReturnHeaders(t *testing.T, headers []returnHeader) {
	t.Helper()
	expected := []struct {
		header string
		format string
	}{
		{header: "Cache-Control", format: "no-store, no-cache, must-revalidate, max-age=0, private"},
		{header: "Surrogate-Control", format: "no-store, max-age=0"},
		{header: "Pragma", format: "no-cache"},
		{header: "Expires", format: "0"},
	}
	for _, item := range expected {
		header := findReturnHeader(headers, item.header)
		if header == nil {
			t.Fatalf("expected normalize no-cache return header %q, got %#v", item.header, headers)
		}
		expectedFormat := normalizeHAProxyFmt(item.format)
		if strings.TrimSpace(header.Format) != expectedFormat {
			t.Fatalf("expected normalize no-cache return header %q format %q, got %#v", item.header, expectedFormat, *header)
		}
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
	liveSlot := slot
	if liveSlot != grpcapi.Slot_SLOT_BLUE && liveSlot != grpcapi.Slot_SLOT_GREEN {
		liveSlot = grpcapi.Slot_SLOT_BLUE
	}
	liveReleaseID := "release-blue"
	liveBackendName := "be-api_blue"
	candidateReleaseID := "release-green"
	candidateBackendName := "be-api_green"
	if liveSlot == grpcapi.Slot_SLOT_GREEN {
		liveReleaseID = "release-green"
		liveBackendName = "be-api_green"
		candidateReleaseID = "release-blue"
		candidateBackendName = "be-api_blue"
	}
	return &grpcapi.ProxyConfigSnapshot{
		AgentId:        "agent-1",
		FrontendName:   "ep_http",
		DefaultBackend: "ep_default",
		BindPort:       80,
		Services: []*grpcapi.ProxyServiceConfig{
			{
				ServiceId:               "svc-1",
				ServiceKey:              "svc-a",
				RouteHost:               "api.example.com",
				RoutePathPrefix:         "/",
				BackendName:             "be-api",
				ContainerPort:           8080,
				CurrentLiveSlot:         liveSlot,
				LiveReleaseId:           liveReleaseID,
				LiveBackendName:         liveBackendName,
				CandidateReleaseId:      candidateReleaseID,
				CandidateBackendName:    candidateBackendName,
				CandidateTrafficPercent: 0,
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
	serverEntries    []dataplaneBackendServer
	backendConfigs   []backendSection
}

func (f *fakeManagedProxyDataplane) ConfigurationVersion(context.Context) (string, error) {
	*f.callLog = append(*f.callLog, "version")
	return f.version, nil
}

func (f *fakeManagedProxyDataplane) ShowRawConfig(context.Context) (string, error) {
	*f.callLog = append(*f.callLog, "show-raw-config")
	return "global\n  daemon\n", nil
}

func (f *fakeManagedProxyDataplane) ShowRawConfigInTransaction(_ context.Context, transactionID string) (string, error) {
	return "frontend ep_http\n  bind *:80\n", nil
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
	f.backendConfigs = append(f.backendConfigs, section)
	return f.ensureBackendErr
}

func (f *fakeManagedProxyDataplane) EnsureServerInTransaction(_ context.Context, backendName string, transactionID string, server backendServer) error {
	*f.callLog = append(*f.callLog, "ensure-server:"+backendName+"/"+server.Name+"@"+transactionID)
	f.serverConfigs = append(f.serverConfigs, server)
	f.serverEntries = append(f.serverEntries, dataplaneBackendServer{
		Backend: backendName,
		Server:  server,
	})
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

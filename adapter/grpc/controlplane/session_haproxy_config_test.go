package controlplane

import (
	"context"
	"edge-pilot/internal/agent/application/proxyconfig"
	"edge-pilot/internal/shared/grpcapi"
	"sync"
	"testing"
	"time"
)

func TestSessionHubRequestHAProxyConfigSuccess(t *testing.T) {
	hub := NewSessionHub(nil)
	session := hub.register("agent-a")
	defer hub.unregister("agent-a")

	resultCh := make(chan struct {
		config string
		err    error
	}, 1)
	go func() {
		config, err := hub.RequestHAProxyConfig(context.Background(), "agent-a")
		resultCh <- struct {
			config string
			err    error
		}{config: config, err: err}
	}()

	msg := <-session.sendCh
	req := msg.GetHaproxyConfigRequest()
	if req == nil {
		t.Fatal("expected haproxy config request to be dispatched")
	}
	hub.resolveHAProxyConfigResponse(&grpcapi.HAProxyConfigResponse{
		RequestId: req.GetRequestId(),
		AgentId:   "agent-a",
		Config:    "global\n  daemon\n",
	})

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("RequestHAProxyConfig() error = %v", result.err)
	}
	if result.config != "global\n  daemon\n" {
		t.Fatalf("unexpected config %q", result.config)
	}
}

func TestSessionHubRequestHAProxyConfigTimeout(t *testing.T) {
	hub := NewSessionHub(nil)
	hub.register("agent-a")
	defer hub.unregister("agent-a")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := hub.RequestHAProxyConfig(ctx, "agent-a")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if err != proxyconfig.ErrHAProxyConfigTimeout {
		t.Fatalf("expected ErrHAProxyConfigTimeout, got %v", err)
	}
}

func TestSessionHubRequestHAProxyConfigConcurrentIsolation(t *testing.T) {
	hub := NewSessionHub(nil)
	session := hub.register("agent-a")
	defer hub.unregister("agent-a")

	var wg sync.WaitGroup
	wg.Add(2)

	type outcome struct {
		config string
		err    error
	}
	outcomes := make(chan outcome, 2)

	call := func() {
		defer wg.Done()
		config, err := hub.RequestHAProxyConfig(context.Background(), "agent-a")
		outcomes <- outcome{config: config, err: err}
	}
	go call()
	go call()

	first := <-session.sendCh
	second := <-session.sendCh
	firstReq := first.GetHaproxyConfigRequest()
	secondReq := second.GetHaproxyConfigRequest()
	if firstReq == nil || secondReq == nil {
		t.Fatal("expected both requests to be haproxy config requests")
	}

	hub.resolveHAProxyConfigResponse(&grpcapi.HAProxyConfigResponse{
		RequestId: secondReq.GetRequestId(),
		AgentId:   "agent-a",
		Config:    "cfg-second",
	})
	hub.resolveHAProxyConfigResponse(&grpcapi.HAProxyConfigResponse{
		RequestId: firstReq.GetRequestId(),
		AgentId:   "agent-a",
		Config:    "cfg-first",
	})

	wg.Wait()
	close(outcomes)

	received := map[string]struct{}{}
	for item := range outcomes {
		if item.err != nil {
			t.Fatalf("unexpected error: %v", item.err)
		}
		received[item.config] = struct{}{}
	}
	if _, ok := received["cfg-first"]; !ok {
		t.Fatalf("cfg-first not received: %#v", received)
	}
	if _, ok := received["cfg-second"]; !ok {
		t.Fatalf("cfg-second not received: %#v", received)
	}
}

package sdk

import "testing"

func TestNewExecutorClient_UsesAddrPriority(t *testing.T) {
	client := NewExecutorClient(ExecutorClientOptions{
		Addr:      "cp:19090",
		RelayAddr: "agent:19091",
	})
	if client.opts.Addr != "cp:19090" {
		t.Fatalf("expected addr cp:19090, got %s", client.opts.Addr)
	}
}

func TestNewExecutorClient_FallbackRelayAddrForCompatibility(t *testing.T) {
	client := NewExecutorClient(ExecutorClientOptions{
		RelayAddr: "agent:19091",
	})
	if client.opts.Addr != "agent:19091" {
		t.Fatalf("expected fallback relay addr agent:19091, got %s", client.opts.Addr)
	}
}

func TestNewExecutorClient_AddsRelayTokenMetadata(t *testing.T) {
	client := NewExecutorClient(ExecutorClientOptions{
		Addr:       "cp:19090",
		RelayToken: "secret",
		Metadata: map[string]string{
			"k": "v",
		},
	})
	if got := client.opts.Metadata["relay_token"]; got != "secret" {
		t.Fatalf("expected relay_token metadata, got %q", got)
	}
	if got := client.opts.Metadata["k"]; got != "v" {
		t.Fatalf("expected custom metadata kept, got %q", got)
	}
}

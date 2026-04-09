package config

import "testing"

func TestLoadAdminAuthConfig(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "secret")
	t.Setenv("ADMIN_SESSION_SECRET", "session-secret")
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1,10.0.0.0/8")
	t.Setenv("TRUST_CLOUDFLARE", "true")

	cfg, err := LoadAdminAuthConfig()
	if err != nil {
		t.Fatalf("LoadAdminAuthConfig() error = %v", err)
	}
	if cfg.CookieName != "ep_admin_session" {
		t.Fatalf("expected default cookie name, got %q", cfg.CookieName)
	}
	if !cfg.TrustCloudflare {
		t.Fatalf("expected cloudflare trust to be enabled")
	}
	if !cfg.IsTrustedProxy("127.0.0.1:8080") {
		t.Fatalf("expected loopback proxy to be trusted")
	}
	if cfg.IsTrustedProxy("203.0.113.7:8080") {
		t.Fatalf("expected public proxy to be rejected")
	}
}

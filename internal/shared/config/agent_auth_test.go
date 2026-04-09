package config

import "testing"

func TestAgentAuthConfigGenerateTokenHashesToken(t *testing.T) {
	cfg := LoadAgentAuthConfig()

	token, hash, err := cfg.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token == "" {
		t.Fatalf("expected token to be generated")
	}
	if hash == "" {
		t.Fatalf("expected token hash to be generated")
	}
	if !cfg.ValidateHash(hash, token) {
		t.Fatalf("expected generated token to match generated hash")
	}
	if cfg.ValidateHash(hash, token+"x") {
		t.Fatalf("expected mismatched token to fail validation")
	}
}

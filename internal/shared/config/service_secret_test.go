package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadServiceSecretConfigAllowsMissingKey(t *testing.T) {
	t.Setenv("SERVICE_SECRET_MASTER_KEY", "")

	cfg, err := LoadServiceSecretConfig()
	if err != nil {
		t.Fatalf("LoadServiceSecretConfig() error = %v", err)
	}
	if cfg.EncryptionEnabled() {
		t.Fatal("expected encryption to be disabled when key is missing")
	}
	if cfg.KeyVersion != "v1" {
		t.Fatalf("expected default key version v1, got %q", cfg.KeyVersion)
	}
}

func TestLoadServiceSecretConfigRejectsInvalidBase64(t *testing.T) {
	t.Setenv("SERVICE_SECRET_MASTER_KEY", "not-base64")

	_, err := LoadServiceSecretConfig()
	if err == nil || !strings.Contains(err.Error(), "base64") {
		t.Fatalf("expected base64 error, got %v", err)
	}
}

func TestLoadServiceSecretConfigRejectsWrongLength(t *testing.T) {
	t.Setenv("SERVICE_SECRET_MASTER_KEY", base64.StdEncoding.EncodeToString([]byte("short")))

	_, err := LoadServiceSecretConfig()
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("expected wrong length error, got %v", err)
	}
}

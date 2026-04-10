package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const defaultRegistrySecretKeyVersion = "v1"

type RegistryCredentialConfig struct {
	MasterKey  []byte
	KeyVersion string
}

func LoadRegistryCredentialConfig() (*RegistryCredentialConfig, error) {
	raw := strings.TrimSpace(os.Getenv("REGISTRY_SECRET_MASTER_KEY"))
	cfg := &RegistryCredentialConfig{
		KeyVersion: defaultRegistrySecretKeyVersion,
	}
	if raw == "" {
		return cfg, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("REGISTRY_SECRET_MASTER_KEY must be base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("REGISTRY_SECRET_MASTER_KEY must decode to 32 bytes")
	}
	cfg.MasterKey = key
	return cfg, nil
}

func (c *RegistryCredentialConfig) EncryptionEnabled() bool {
	return len(c.MasterKey) == 32
}

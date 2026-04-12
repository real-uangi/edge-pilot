package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const defaultServiceSecretKeyVersion = "v1"

type ServiceSecretConfig struct {
	MasterKey  []byte
	KeyVersion string
}

func LoadServiceSecretConfig() (*ServiceSecretConfig, error) {
	raw := strings.TrimSpace(os.Getenv("SERVICE_SECRET_MASTER_KEY"))
	cfg := &ServiceSecretConfig{
		KeyVersion: defaultServiceSecretKeyVersion,
	}
	if raw == "" {
		return cfg, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("SERVICE_SECRET_MASTER_KEY must be base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("SERVICE_SECRET_MASTER_KEY must decode to 32 bytes")
	}
	cfg.MasterKey = key
	return cfg, nil
}

func (c *ServiceSecretConfig) EncryptionEnabled() bool {
	return len(c.MasterKey) == 32
}

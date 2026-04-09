package config

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

type AgentAuthConfig struct{}

func LoadAgentAuthConfig() *AgentAuthConfig {
	return &AgentAuthConfig{}
}

func (c *AgentAuthConfig) GenerateToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, c.HashToken(token), nil
}

func (c *AgentAuthConfig) HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (c *AgentAuthConfig) ValidateHash(expectedHash string, token string) bool {
	if expectedHash == "" || token == "" {
		return false
	}
	actual := c.HashToken(token)
	return subtle.ConstantTimeCompare([]byte(expectedHash), []byte(actual)) == 1
}

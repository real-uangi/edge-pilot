package application

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"edge-pilot/internal/shared/config"
	"encoding/base64"
	"io"

	"github.com/real-uangi/allingo/common/business"
)

const errRegistrySecretKeyMissing = "registry secret master key not configured"

type Crypto struct {
	cfg *config.RegistryCredentialConfig
}

func NewCrypto(cfg *config.RegistryCredentialConfig) *Crypto {
	return &Crypto{cfg: cfg}
}

func (c *Crypto) Encrypt(secret string) (string, string, error) {
	if !c.cfg.EncryptionEnabled() {
		return "", "", business.NewErrorWithCode(errRegistrySecretKeyMissing, 500)
	}
	block, err := aes.NewCipher(c.cfg.MasterKey)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), c.cfg.KeyVersion, nil
}

func (c *Crypto) Decrypt(ciphertext string, keyVersion string) (string, error) {
	if !c.cfg.EncryptionEnabled() {
		return "", business.NewErrorWithCode(errRegistrySecretKeyMissing, 500)
	}
	if keyVersion != "" && keyVersion != c.cfg.KeyVersion {
		return "", business.NewErrorWithCode("registry secret key version is not supported", 500)
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.cfg.MasterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", business.NewErrorWithCode("registry secret ciphertext is invalid", 500)
	}
	nonce := raw[:gcm.NonceSize()]
	plaintext, err := gcm.Open(nil, nonce, raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"edge-pilot/internal/shared/config"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/real-uangi/allingo/common/business"
)

const errServiceSecretKeyMissing = "service secret master key not configured"

type Codec struct {
	cfg *config.ServiceSecretConfig
}

func NewCodec(cfg *config.ServiceSecretConfig) *Codec {
	return &Codec{cfg: cfg}
}

func (c *Codec) EncryptJSON(value any) (string, string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", "", err
	}
	return c.encrypt(raw)
}

func (c *Codec) DecryptJSON(ciphertext string, keyVersion string, target any) error {
	raw, err := c.decrypt(ciphertext, keyVersion)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func (c *Codec) EncryptionEnabled() bool {
	return c != nil && c.cfg != nil && c.cfg.EncryptionEnabled()
}

func (c *Codec) encrypt(raw []byte) (string, string, error) {
	if !c.EncryptionEnabled() {
		return "", "", business.NewErrorWithCode(errServiceSecretKeyMissing, 500)
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
	ciphertext := gcm.Seal(nonce, nonce, raw, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), c.cfg.KeyVersion, nil
}

func (c *Codec) decrypt(ciphertext string, keyVersion string) ([]byte, error) {
	if !c.EncryptionEnabled() {
		return nil, business.NewErrorWithCode(errServiceSecretKeyMissing, 500)
	}
	if keyVersion != "" && keyVersion != c.cfg.KeyVersion {
		return nil, business.NewErrorWithCode("service secret key version is not supported", 500)
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(c.cfg.MasterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, business.NewErrorWithCode("service secret ciphertext is invalid", 500)
	}
	nonce := raw[:gcm.NonceSize()]
	return gcm.Open(nil, nonce, raw[gcm.NonceSize():], nil)
}

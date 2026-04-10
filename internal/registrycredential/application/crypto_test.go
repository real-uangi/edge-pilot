package application

import (
	"edge-pilot/internal/shared/config"
	"encoding/base64"
	"testing"
)

func TestCryptoEncryptDecryptRoundTrip(t *testing.T) {
	crypto := NewCrypto(&config.RegistryCredentialConfig{
		MasterKey:  []byte("0123456789abcdef0123456789abcdef"),
		KeyVersion: "v1",
	})

	ciphertext, keyVersion, err := crypto.Encrypt("secret-token")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if ciphertext == "" {
		t.Fatal("expected ciphertext to be populated")
	}
	if _, err := base64.StdEncoding.DecodeString(ciphertext); err != nil {
		t.Fatalf("expected base64 ciphertext, got %v", err)
	}
	if keyVersion != "v1" {
		t.Fatalf("expected key version v1, got %q", keyVersion)
	}

	plaintext, err := crypto.Decrypt(ciphertext, keyVersion)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "secret-token" {
		t.Fatalf("expected original plaintext, got %q", plaintext)
	}
}

func TestCryptoDecryptRejectsWrongKey(t *testing.T) {
	cryptoA := NewCrypto(&config.RegistryCredentialConfig{
		MasterKey:  []byte("0123456789abcdef0123456789abcdef"),
		KeyVersion: "v1",
	})
	cryptoB := NewCrypto(&config.RegistryCredentialConfig{
		MasterKey:  []byte("fedcba9876543210fedcba9876543210"),
		KeyVersion: "v1",
	})

	ciphertext, keyVersion, err := cryptoA.Encrypt("secret-token")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if _, err := cryptoB.Decrypt(ciphertext, keyVersion); err == nil {
		t.Fatal("expected decrypt with wrong key to fail")
	}
}

func TestCryptoRequiresConfiguredMasterKey(t *testing.T) {
	crypto := NewCrypto(&config.RegistryCredentialConfig{KeyVersion: "v1"})

	if _, _, err := crypto.Encrypt("secret-token"); err == nil {
		t.Fatal("expected Encrypt() to fail when master key is missing")
	}
	if _, err := crypto.Decrypt("ciphertext", "v1"); err == nil {
		t.Fatal("expected Decrypt() to fail when master key is missing")
	}
}

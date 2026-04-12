package secret

import (
	"edge-pilot/internal/shared/config"
	"testing"
)

func TestCodecEncryptDecryptJSONRoundTrip(t *testing.T) {
	codec := NewCodec(&config.ServiceSecretConfig{
		MasterKey:  []byte("12345678901234567890123456789012"),
		KeyVersion: "v1",
	})

	ciphertext, keyVersion, err := codec.EncryptJSON(map[string]string{"A": "1"})
	if err != nil {
		t.Fatalf("EncryptJSON() error = %v", err)
	}
	if ciphertext == "" {
		t.Fatal("expected ciphertext to be populated")
	}
	if keyVersion != "v1" {
		t.Fatalf("expected key version v1, got %q", keyVersion)
	}

	var output map[string]string
	if err := codec.DecryptJSON(ciphertext, keyVersion, &output); err != nil {
		t.Fatalf("DecryptJSON() error = %v", err)
	}
	if output["A"] != "1" {
		t.Fatalf("expected decrypted payload, got %#v", output)
	}
}

func TestCodecRejectsMismatchedKeyVersion(t *testing.T) {
	codec := NewCodec(&config.ServiceSecretConfig{
		MasterKey:  []byte("12345678901234567890123456789012"),
		KeyVersion: "v2",
	})

	ciphertext, _, err := codec.EncryptJSON(map[string]string{"A": "1"})
	if err != nil {
		t.Fatalf("EncryptJSON() error = %v", err)
	}

	var output map[string]string
	if err := codec.DecryptJSON(ciphertext, "v1", &output); err == nil {
		t.Fatal("expected mismatched key version to fail")
	}
}

package application

import (
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"testing"
	"time"
)

func TestServiceLoginAndParseSession(t *testing.T) {
	service := NewService(&config.AdminAuthConfig{
		Username:      "admin",
		Password:      "secret",
		SessionSecret: "session-secret",
		SessionTTL:    time.Hour,
	})

	token, output, err := service.Login(dto.AdminLoginRequest{
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if output.Username != "admin" {
		t.Fatalf("expected admin username, got %q", output.Username)
	}
	claims, err := service.ParseSession(token)
	if err != nil {
		t.Fatalf("ParseSession() error = %v", err)
	}
	if claims.Username != "admin" {
		t.Fatalf("expected session username admin, got %q", claims.Username)
	}
}

func TestServiceRejectsExpiredOrTamperedSession(t *testing.T) {
	service := NewService(&config.AdminAuthConfig{
		Username:      "admin",
		Password:      "secret",
		SessionSecret: "session-secret",
		SessionTTL:    -time.Minute,
	})

	token, _, err := service.Login(dto.AdminLoginRequest{
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if _, err := service.ParseSession(token); err == nil {
		t.Fatalf("expected expired session to fail")
	}
	if _, err := service.ParseSession(token + "x"); err == nil {
		t.Fatalf("expected tampered session to fail")
	}
}

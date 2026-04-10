package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	"edge-pilot/internal/shared/config"
	"edge-pilot/internal/shared/dto"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRegistryCredentialRoutesDoNotExposeSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.AdminAuthConfig{
		Username:      "admin",
		Password:      "secret",
		SessionSecret: "session-secret",
		SessionTTL:    time.Hour,
		CookieName:    "ep_admin_session",
	}
	auth := adminauthapp.NewService(cfg)
	token, _, err := auth.Login(dto.AdminLoginRequest{
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	engine := gin.New()
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	fake := &fakeRegistryCredentialActions{
		item: &dto.RegistryCredentialOutput{
			ID:               uuid.New(),
			RegistryHost:     "ghcr.io",
			Username:         "octocat",
			SecretConfigured: true,
		},
	}
	registerAdminRegistryCredentialRoutes(admin, fake)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/registry-credentials", nil)
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"secret":`) {
		t.Fatalf("expected response not to expose secret, got %s", recorder.Body.String())
	}
}

type fakeRegistryCredentialActions struct {
	item *dto.RegistryCredentialOutput
}

func (f *fakeRegistryCredentialActions) Create(dto.UpsertRegistryCredentialRequest) (*dto.RegistryCredentialOutput, error) {
	return f.item, nil
}

func (f *fakeRegistryCredentialActions) Update(uuid.UUID, dto.UpsertRegistryCredentialRequest) (*dto.RegistryCredentialOutput, error) {
	return f.item, nil
}

func (f *fakeRegistryCredentialActions) Delete(uuid.UUID) error {
	return nil
}

func (f *fakeRegistryCredentialActions) Get(uuid.UUID) (*dto.RegistryCredentialOutput, error) {
	return f.item, nil
}

func (f *fakeRegistryCredentialActions) List() ([]dto.RegistryCredentialOutput, error) {
	return []dto.RegistryCredentialOutput{*f.item}, nil
}

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

type fakeReleaseActions struct {
	lastOperator string
}

func (f *fakeReleaseActions) List() ([]dto.ReleaseOutput, error) {
	return nil, nil
}

func (f *fakeReleaseActions) Get(id uuid.UUID) (*dto.ReleaseOutput, error) {
	return &dto.ReleaseOutput{ID: id}, nil
}

func (f *fakeReleaseActions) ListTaskSnapshots(releaseID uuid.UUID) ([]dto.TaskSnapshot, error) {
	return nil, nil
}

func (f *fakeReleaseActions) Start(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	f.lastOperator = operator
	return &dto.ReleaseOutput{ID: id}, nil
}

func (f *fakeReleaseActions) Skip(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	f.lastOperator = operator
	return &dto.ReleaseOutput{ID: id}, nil
}

func (f *fakeReleaseActions) ConfirmSwitch(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	f.lastOperator = operator
	return &dto.ReleaseOutput{ID: id}, nil
}

func (f *fakeReleaseActions) Rollback(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
	f.lastOperator = operator
	return &dto.ReleaseOutput{ID: id}, nil
}

func TestAdminReleaseRoutesUseSessionUsernameAsOperator(t *testing.T) {
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
	fake := &fakeReleaseActions{}
	registerAdminReleaseRoutes(admin, fake)

	releaseID := uuid.NewString()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/releases/"+releaseID+"/start", strings.NewReader(`{"operator":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if fake.lastOperator != "admin" {
		t.Fatalf("expected operator admin, got %q", fake.lastOperator)
	}
}

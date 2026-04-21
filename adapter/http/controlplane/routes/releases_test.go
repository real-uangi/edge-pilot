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

	"github.com/real-uangi/allingo/common/business"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeReleaseActions struct {
	lastOperator string
	lastPercent  int
	startErr     error
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
	if f.startErr != nil {
		return nil, f.startErr
	}
	return &dto.ReleaseOutput{ID: id}, nil
}

func (f *fakeReleaseActions) Retry(id uuid.UUID, operator string) (*dto.ReleaseOutput, error) {
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

func (f *fakeReleaseActions) SetTrafficPercent(id uuid.UUID, percent int, operator string) (*dto.ReleaseOutput, error) {
	f.lastOperator = operator
	f.lastPercent = percent
	return &dto.ReleaseOutput{ID: id, TrafficPercent: percent}, nil
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

func TestAdminReleaseRetryRouteUsesSessionUsernameAsOperator(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, "/api/admin/releases/"+releaseID+"/retry", strings.NewReader(`{"operator":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("retry status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if fake.lastOperator != "admin" {
		t.Fatalf("expected operator admin, got %q", fake.lastOperator)
	}
}

func TestAdminReleaseTrafficRouteUsesSessionUsernameAsOperator(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, "/api/admin/releases/"+releaseID+"/traffic", strings.NewReader(`{"percent":80}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("traffic status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if fake.lastOperator != "admin" {
		t.Fatalf("expected operator admin, got %q", fake.lastOperator)
	}
	if fake.lastPercent != 80 {
		t.Fatalf("expected percent 80, got %d", fake.lastPercent)
	}
}

func TestAdminReleaseStartRoutePassesConflictMessage(t *testing.T) {
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
	fake := &fakeReleaseActions{
		startErr: business.NewErrorWithCode("service has in-progress traffic split (1-99%), finish at 0% or 100% before starting a new release", 409),
	}
	registerAdminReleaseRoutes(admin, fake)

	releaseID := uuid.NewString()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/releases/"+releaseID+"/start", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":409`) {
		t.Fatalf("expected conflict code in response body, got %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "1-99%") {
		t.Fatalf("expected conflict message in response body, got %s", recorder.Body.String())
	}
}

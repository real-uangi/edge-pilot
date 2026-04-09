package routes

import (
	adaptermiddleware "edge-pilot/adapter/http/middleware"
	adminauthapp "edge-pilot/internal/adminauth/application"
	"edge-pilot/internal/shared/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAuthRoutesLoginMeAndSecureCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "secret")
	t.Setenv("ADMIN_SESSION_SECRET", "session-secret")
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1")
	cfg, err := config.LoadAdminAuthConfig()
	if err != nil {
		t.Fatalf("LoadAdminAuthConfig() error = %v", err)
	}
	cfg.SessionTTL = time.Hour
	auth := adminauthapp.NewService(cfg)

	engine := gin.New()
	if err := adaptermiddleware.ApplyProxyTrust(engine, cfg); err != nil {
		t.Fatalf("ApplyProxyTrust() error = %v", err)
	}
	SetAuthRoutes(engine, auth, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	setCookie := recorder.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "Secure") {
		t.Fatalf("expected secure session cookie, got %q", setCookie)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(recorder.Result().Cookies()[0])
	meRecorder := httptest.NewRecorder()
	engine.ServeHTTP(meRecorder, meReq)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("me status = %d body=%s", meRecorder.Code, meRecorder.Body.String())
	}
	if !strings.Contains(meRecorder.Body.String(), `"username":"admin"`) {
		t.Fatalf("expected me body to include username, got %s", meRecorder.Body.String())
	}
}

func TestAuthRoutesRejectInvalidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "secret")
	t.Setenv("ADMIN_SESSION_SECRET", "session-secret")
	cfg, err := config.LoadAdminAuthConfig()
	if err != nil {
		t.Fatalf("LoadAdminAuthConfig() error = %v", err)
	}
	cfg.SessionTTL = time.Hour
	auth := adminauthapp.NewService(cfg)

	engine := gin.New()
	SetAuthRoutes(engine, auth, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: "bad-token"})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

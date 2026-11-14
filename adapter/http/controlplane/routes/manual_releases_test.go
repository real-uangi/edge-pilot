package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adaptermiddleware "github.com/real-uangi/edge-pilot/adapter/http/middleware"
	adminauthapp "github.com/real-uangi/edge-pilot/internal/adminauth/application"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestAdminReleaseCreateRoute(t *testing.T) {
	const serviceID = "550e8400-e29b-41d4-a716-446655440000"
	for _, tc := range []struct {
		name       string
		body       string
		withAuth   bool
		wantStatus int
	}{
		{
			name:       "uses session identity and accepts only release fields",
			body:       `{"serviceId":"550e8400-e29b-41d4-a716-446655440000","imageTag":"latest","releaseNotes":"manual deployment","operator":"spoofed","triggeredBy":"spoofed","imageRepo":"attacker.example.com/service","agentId":"spoofed"}`,
			withAuth:   true,
			wantStatus: http.StatusOK,
		},
		{name: "requires session", body: `{"serviceId":"550e8400-e29b-41d4-a716-446655440000","imageTag":"latest"}`, wantStatus: http.StatusUnauthorized},
		{name: "requires service", body: `{"imageTag":"latest"}`, withAuth: true, wantStatus: http.StatusBadRequest},
		{name: "requires tag", body: `{"serviceId":"550e8400-e29b-41d4-a716-446655440000"}`, withAuth: true, wantStatus: http.StatusBadRequest},
		{name: "invalid service id", body: `{"serviceId":"invalid","imageTag":"latest"}`, withAuth: true, wantStatus: http.StatusBadRequest},
		{name: "invalid json", body: `{`, withAuth: true, wantStatus: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			cfg := &config.AdminAuthConfig{
				Username:      "admin",
				Password:      "secret",
				SessionSecret: "session-secret",
				SessionTTL:    time.Hour,
				CookieName:    "ep_admin_session",
			}
			auth := adminauthapp.NewService(cfg)
			token, _, err := auth.Login(dto.AdminLoginRequest{Username: "admin", Password: "secret"})
			if err != nil {
				t.Fatal(err)
			}
			engine := gin.New()
			admin := engine.Group("/api/admin")
			admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
			fake := &fakeReleaseActions{}
			registerAdminReleaseRoutes(admin, fake)
			req := httptest.NewRequest(http.MethodPost, "/api/admin/releases", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.withAuth {
				req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantStatus, recorder.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				if fake.manualCalls != 0 {
					t.Fatalf("invalid request reached use case %d times", fake.manualCalls)
				}
				return
			}
			if fake.manualCalls != 1 || fake.lastOperator != "admin" {
				t.Fatalf("expected one call with session user admin, got calls=%d operator=%q", fake.manualCalls, fake.lastOperator)
			}
			if fake.manualInput != (dto.CreateManualReleaseRequest{ServiceID: uuid.MustParse(serviceID), ImageTag: "latest", ReleaseNotes: "manual deployment"}) {
				t.Fatalf("unexpected create request: %#v", fake.manualInput)
			}
		})
	}
}

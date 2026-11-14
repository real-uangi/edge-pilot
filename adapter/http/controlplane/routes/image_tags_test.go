package routes

import (
	"context"
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

type fakeServiceImageTagActions struct {
	input dto.ListServiceImageTagsRequest
	calls int
	ctx   context.Context
}

func (f *fakeServiceImageTagActions) List(ctx context.Context, input dto.ListServiceImageTagsRequest) (*dto.ServiceImageTagsOutput, error) {
	f.calls++
	f.input = input
	f.ctx = ctx
	return &dto.ServiceImageTagsOutput{ImageRepo: "ghcr.io/team/app", Tags: []string{"latest", "v2"}, NextCursor: "v2"}, nil
}

func TestAdminServiceImageTagRouteRequiresSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.AdminAuthConfig{Username: "admin", Password: "secret", SessionSecret: "session-secret", SessionTTL: time.Hour, CookieName: "ep_admin_session"}
	auth := adminauthapp.NewService(cfg)
	engine := gin.New()
	admin := engine.Group("/api/admin")
	admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
	fake := &fakeServiceImageTagActions{}
	registerAdminServiceImageTagRoutes(admin, fake)
	id := uuid.New()
	body := `{"serviceId":"` + id.String() + `","cursor":"v1","imageRepo":"evil.example.com/app"}`

	req := httptest.NewRequest(http.MethodPost, "/api/admin/services/image-tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized || fake.calls != 0 {
		t.Fatalf("unauthenticated request: status=%d calls=%d", recorder.Code, fake.calls)
	}

	token, _, err := auth.Login(dto.AdminLoginRequest{Username: cfg.Username, Password: cfg.Password})
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/admin/services/image-tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || fake.calls != 1 || fake.input.ServiceID != id || fake.input.Cursor != "v1" || fake.ctx != req.Context() {
		t.Fatalf("authenticated request: status=%d calls=%d input=%#v body=%s", recorder.Code, fake.calls, fake.input, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"nextCursor":"v2"`) || strings.Contains(recorder.Body.String(), "evil.example.com") {
		t.Fatalf("unexpected response %s", recorder.Body.String())
	}
}

func TestAdminServiceImageTagRouteRejectsInvalidServiceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	fake := &fakeServiceImageTagActions{}
	registerAdminServiceImageTagRoutes(engine.Group("/api/admin"), fake)
	for _, body := range []string{`{}`, `{"serviceId":"not-a-uuid"}`, `{"serviceId":"00000000-0000-0000-0000-000000000000"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/services/image-tags", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusBadRequest || fake.calls != 0 {
			t.Fatalf("body=%s status=%d calls=%d response=%s", body, recorder.Code, fake.calls, recorder.Body.String())
		}
	}
}

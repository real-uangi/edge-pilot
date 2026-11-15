package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	adaptermiddleware "github.com/real-uangi/edge-pilot/adapter/http/middleware"
	adminauthapp "github.com/real-uangi/edge-pilot/internal/adminauth/application"
	"github.com/real-uangi/edge-pilot/internal/shared/config"
	"github.com/real-uangi/edge-pilot/internal/shared/dto"
	"github.com/real-uangi/edge-pilot/internal/shared/model"

	"github.com/real-uangi/allingo/common/business"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeReleaseActions struct {
	lastOperator    string
	lastPercent     int
	startErr        error
	manualInput     dto.CreateManualReleaseRequest
	manualCalls     int
	tasks           []dto.TaskSnapshot
	audits          []dto.AuditLog
	pipeline        *dto.ReleasePipeline
	pipelineErr     error
	pipelineCalls   int
	pipelineRelease *dto.ReleaseOutput
	pipelineTasks   []dto.TaskSnapshot
	pipelineAudits  []dto.AuditLog
}

func (f *fakeReleaseActions) CreateManual(input dto.CreateManualReleaseRequest, operator string) (*dto.ReleaseOutput, error) {
	f.manualInput = input
	f.manualCalls++
	f.lastOperator = operator
	return &dto.ReleaseOutput{ID: uuid.New(), ServiceID: input.ServiceID, ImageTag: input.ImageTag, TriggeredBy: operator}, nil
}

func (f *fakeReleaseActions) List() ([]dto.ReleaseOutput, error) {
	return nil, nil
}

func (f *fakeReleaseActions) Get(id uuid.UUID) (*dto.ReleaseOutput, error) {
	return &dto.ReleaseOutput{ID: id}, nil
}

func (f *fakeReleaseActions) ListTaskSnapshots(releaseID uuid.UUID) ([]dto.TaskSnapshot, error) {
	return f.tasks, nil
}

func (f *fakeReleaseActions) ListReleaseAudits(releaseID uuid.UUID) ([]dto.AuditLog, error) {
	return f.audits, nil
}

func (f *fakeReleaseActions) GetPipeline(release *dto.ReleaseOutput, tasks []dto.TaskSnapshot, audits []dto.AuditLog) (*dto.ReleasePipeline, error) {
	f.pipelineCalls++
	f.pipelineRelease = release
	f.pipelineTasks = tasks
	f.pipelineAudits = audits
	return f.pipeline, f.pipelineErr
}

func (f *fakeReleaseActions) GetReleaseNotesAggregate(releaseID uuid.UUID) ([]dto.ReleaseNotesItem, error) {
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

func TestAdminReleaseDetailPipeline(t *testing.T) {
	for _, tc := range []struct {
		name        string
		withAuth    bool
		pipelineErr error
	}{
		{name: "adds pipeline and preserves existing fields", withAuth: true},
		{name: "propagates pipeline error", withAuth: true, pipelineErr: business.NewErrorWithCode("pipeline unavailable", 503)},
		{name: "requires admin session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			cfg := &config.AdminAuthConfig{
				Username: "admin", Password: "secret", SessionSecret: "session-secret",
				SessionTTL: time.Hour, CookieName: "ep_admin_session",
			}
			auth := adminauthapp.NewService(cfg)
			token, _, err := auth.Login(dto.AdminLoginRequest{Username: "admin", Password: "secret"})
			if err != nil {
				t.Fatal(err)
			}
			releaseID, taskID := uuid.New(), uuid.New()
			estimate := int64(1200)
			fake := &fakeReleaseActions{
				tasks:  []dto.TaskSnapshot{{ID: taskID, Status: model.TaskStatusRunning}},
				audits: []dto.AuditLog{{ID: uuid.New(), EventType: "task_dispatched"}},
				pipeline: &dto.ReleasePipeline{
					Stages: []dto.PipelineStage{{ID: "image", Label: "镜像准备", Kind: "automatic", EstimatedDurationMS: &estimate, SampleCount: 3}},
					Runs:   []dto.PipelineRun{{TaskID: taskID, Sequence: 1, Status: model.TaskStatusRunning, Cells: []dto.PipelineCell{{StageID: "image", Status: "running"}}}},
				},
				pipelineErr: tc.pipelineErr,
			}
			engine := gin.New()
			admin := engine.Group("/api/admin")
			admin.Use(adaptermiddleware.RequireAdminSession(auth, cfg))
			registerAdminReleaseRoutes(admin, fake)
			req := httptest.NewRequest(http.MethodGet, "/api/admin/releases/"+releaseID.String(), nil)
			if tc.withAuth {
				req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			if !tc.withAuth {
				if recorder.Code != http.StatusUnauthorized || fake.pipelineCalls != 0 {
					t.Fatalf("unauthorized status = %d, pipeline calls = %d", recorder.Code, fake.pipelineCalls)
				}
				return
			}
			if recorder.Code != http.StatusOK {
				t.Fatalf("detail status = %d body=%s", recorder.Code, recorder.Body.String())
			}
			if fake.pipelineCalls != 1 || fake.pipelineRelease.ID != releaseID || !reflect.DeepEqual(fake.pipelineTasks, fake.tasks) || !reflect.DeepEqual(fake.pipelineAudits, fake.audits) {
				t.Fatalf("pipeline did not receive the loaded release, tasks and audits: %+v", fake)
			}
			if tc.pipelineErr != nil {
				if !strings.Contains(recorder.Body.String(), `"code":503`) || !strings.Contains(recorder.Body.String(), "pipeline unavailable") {
					t.Fatalf("expected pipeline error, body=%s", recorder.Body.String())
				}
				return
			}
			var response struct {
				Data map[string]json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"release", "tasks", "audits", "releaseNotesAggregate", "pipeline"} {
				if _, exists := response.Data[key]; !exists {
					t.Errorf("detail response is missing %q: %s", key, recorder.Body.String())
				}
			}
			var pipeline dto.ReleasePipeline
			if err := json.Unmarshal(response.Data["pipeline"], &pipeline); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(pipeline, *fake.pipeline) {
				t.Fatalf("pipeline = %+v, want %+v", pipeline, *fake.pipeline)
			}
		})
	}
}

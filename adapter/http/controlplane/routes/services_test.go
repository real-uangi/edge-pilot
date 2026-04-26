package routes

import (
	"edge-pilot/internal/shared/dto"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestServiceRoutesDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/admin")
	services := &fakeServiceAdminActions{}
	registerAdminServiceRoutes(admin, services)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/services/11111111-1111-1111-1111-111111111111", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if services.deletedID != uuid.MustParse("11111111-1111-1111-1111-111111111111") {
		t.Fatalf("expected delete to receive parsed uuid, got %s", services.deletedID.String())
	}
	if !strings.Contains(recorder.Body.String(), `"deleted":true`) {
		t.Fatalf("expected delete response to confirm deletion, got %s", recorder.Body.String())
	}
}

func TestServiceRoutesDeleteRejectInvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	admin := engine.Group("/api/admin")
	registerAdminServiceRoutes(admin, &fakeServiceAdminActions{})

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/services/not-a-uuid", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

type fakeServiceAdminActions struct {
	deletedID uuid.UUID
}

func (f *fakeServiceAdminActions) Create(dto.UpsertServiceRequest) (*dto.ServiceOutput, error) {
	return &dto.ServiceOutput{}, nil
}

func (f *fakeServiceAdminActions) Update(uuid.UUID, dto.UpsertServiceRequest) (*dto.ServiceOutput, error) {
	return &dto.ServiceOutput{}, nil
}

func (f *fakeServiceAdminActions) Delete(id uuid.UUID) error {
	f.deletedID = id
	return nil
}

func (f *fakeServiceAdminActions) List() ([]dto.ServiceOutput, error) {
	return nil, nil
}

func (f *fakeServiceAdminActions) Get(uuid.UUID) (*dto.ServiceOutput, error) {
	return &dto.ServiceOutput{}, nil
}

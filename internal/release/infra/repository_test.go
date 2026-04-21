package infra

import (
	"database/sql/driver"
	"edge-pilot/internal/shared/model"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockedRepository(t *testing.T) (*repository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	conn, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	return &repository{conn: conn}, mock
}

func TestHasActiveRelease(t *testing.T) {
	repo, mock := newMockedRepository(t)
	serviceID := uuid.New()

	mock.ExpectQuery(`SELECT count\(\*\) FROM "ep_release"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	active, err := repo.HasActiveRelease(serviceID)
	if err != nil {
		t.Fatalf("HasActiveRelease() error = %v", err)
	}
	if !active {
		t.Fatalf("expected active release")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestHasActiveReleaseReturnsFalseWhenCountIsZero(t *testing.T) {
	repo, mock := newMockedRepository(t)
	serviceID := uuid.New()

	mock.ExpectQuery(`SELECT count\(\*\) FROM "ep_release"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	active, err := repo.HasActiveRelease(serviceID)
	if err != nil {
		t.Fatalf("HasActiveRelease() error = %v", err)
	}
	if active {
		t.Fatalf("expected no active release")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestListQueuedBefore(t *testing.T) {
	repo, mock := newMockedRepository(t)
	serviceID := uuid.New()
	currentReleaseID := uuid.New()
	currentCreatedAt := time.Now()

	firstID := uuid.New()
	secondID := uuid.New()

	rows := sqlmock.NewRows([]string{"id", "service_id", "status", "traffic_percent", "created_at"}).
		AddRow(firstID.String(), serviceID.String(), int(model.ReleaseStatusQueued), 0, currentCreatedAt.Add(-2*time.Minute)).
		AddRow(secondID.String(), serviceID.String(), int(model.ReleaseStatusQueued), 0, currentCreatedAt.Add(-1*time.Minute))

	mock.ExpectQuery(`SELECT \* FROM "ep_release"`).
		WithArgs(
			serviceID,
			model.ReleaseStatusQueued,
			anyTime{},
			currentReleaseID,
		).
		WillReturnRows(rows)

	items, err := repo.ListQueuedBefore(serviceID, currentCreatedAt, currentReleaseID)
	if err != nil {
		t.Fatalf("ListQueuedBefore() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != firstID || items[1].ID != secondID {
		t.Fatalf("expected ordered ids [%s, %s], got [%s, %s]", firstID, secondID, items[0].ID, items[1].ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

type anyTime struct{}

func (anyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

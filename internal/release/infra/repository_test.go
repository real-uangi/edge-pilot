package infra

import (
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/real-uangi/edge-pilot/internal/shared/model"

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

func TestHasTrafficSplitRelease(t *testing.T) {
	repo, mock := newMockedRepository(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "ep_release"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	active, err := repo.HasTrafficSplitRelease(uuid.New())
	if err != nil {
		t.Fatalf("HasTrafficSplitRelease() error = %v", err)
	}
	if !active {
		t.Fatalf("expected traffic split release")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestHasTrafficSplitReleaseReturnsFalseWhenCountIsZero(t *testing.T) {
	repo, mock := newMockedRepository(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "ep_release"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	active, err := repo.HasTrafficSplitRelease(uuid.New())
	if err != nil {
		t.Fatalf("HasTrafficSplitRelease() error = %v", err)
	}
	if active {
		t.Fatalf("expected no traffic split release")
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

func TestListRecentSuccessfulDeployTasksFiltersAndLimitsSamples(t *testing.T) {
	repo, mock := newMockedRepository(t)
	serviceID, currentReleaseID, taskID := uuid.New(), uuid.New(), uuid.New()
	completedAt := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ep_task" WHERE (service_id = $1 AND release_id <> $2 AND type = $3 AND status = $4 AND completed_at IS NOT NULL) AND "ep_task"."deleted_at" IS NULL ORDER BY completed_at desc LIMIT $5`)).
		WithArgs(serviceID, currentReleaseID, model.TaskTypeDeployGreen, model.TaskStatusSucceeded, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "service_id", "type", "status", "completed_at"}).
			AddRow(taskID, serviceID, model.TaskTypeDeployGreen, model.TaskStatusSucceeded, completedAt))

	tasks, err := repo.ListRecentSuccessfulDeployTasks(serviceID, currentReleaseID, 10)
	if err != nil {
		t.Fatalf("ListRecentSuccessfulDeployTasks() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != taskID || tasks[0].CompletedAt == nil || !tasks[0].CompletedAt.Equal(completedAt) {
		t.Fatalf("unexpected samples: %+v", tasks)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListRecentSuccessfulDeployTasksPropagatesQueryError(t *testing.T) {
	repo, mock := newMockedRepository(t)
	wantErr := errors.New("sample query failed")
	mock.ExpectQuery(`SELECT \* FROM "ep_task"`).WillReturnError(wantErr)
	_, err := repo.ListRecentSuccessfulDeployTasks(uuid.New(), uuid.New(), 10)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTaskAttemptsByTasksLoadsOnlySelectedTasksInOrder(t *testing.T) {
	repo, mock := newMockedRepository(t)
	firstTaskID, secondTaskID := uuid.New(), uuid.New()
	firstID, secondID := uuid.New(), uuid.New()
	createdAt := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ep_task_attempt" WHERE task_id IN ($1,$2) AND "ep_task_attempt"."deleted_at" IS NULL ORDER BY created_at asc`)).
		WithArgs(firstTaskID, secondTaskID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "created_at", "message"}).
			AddRow(firstID, firstTaskID, createdAt, "accepted").
			AddRow(secondID, secondTaskID, createdAt.Add(time.Second), "image_pulled"))

	attempts, err := repo.ListTaskAttemptsByTasks([]uuid.UUID{firstTaskID, secondTaskID})
	if err != nil {
		t.Fatalf("ListTaskAttemptsByTasks() error = %v", err)
	}
	if len(attempts) != 2 || attempts[0].ID != firstID || attempts[1].ID != secondID {
		t.Fatalf("unexpected attempts: %+v", attempts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTaskAttemptsByTasksEmptyDoesNotQuery(t *testing.T) {
	// No connection is needed when there are no sample tasks.
	repo := &repository{}
	attempts, err := repo.ListTaskAttemptsByTasks(nil)
	if err != nil || len(attempts) != 0 {
		t.Fatalf("ListTaskAttemptsByTasks(nil) = %+v, %v", attempts, err)
	}
}

func TestListTaskAttemptsByTasksPropagatesQueryError(t *testing.T) {
	repo, mock := newMockedRepository(t)
	wantErr := errors.New("attempt query failed")
	mock.ExpectQuery(`SELECT \* FROM "ep_task_attempt"`).WillReturnError(wantErr)
	_, err := repo.ListTaskAttemptsByTasks([]uuid.UUID{uuid.New()})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type anyTime struct{}

func (anyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

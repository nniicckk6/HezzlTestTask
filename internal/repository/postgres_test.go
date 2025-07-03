// Пакет repository содержит unit-тесты для реализации слоя доступа к данным GoodRepository
package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// Тест создания товара: проверяем успешную вставку и автогенерацию полей через RETURNING
func TestCreateGood(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()

	// успешный сценарий
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO goods(project_id, name, description)")).
		WithArgs(1, "Название", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "priority", "removed", "created_at"}).
			AddRow(10, 1, false, time.Now()))

	good, err := repo.CreateGood(ctx, 1, "Название", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if good.ID != 10 || good.Priority != 1 || good.ProjectID != 1 || good.Name != "Название" {
		t.Error("unexpected good result")
	}

	// ошибка при пустом имени
	_, err = repo.CreateGood(ctx, 1, "", nil)
	if !errors.Is(err, errors.New("name cannot be empty")) {
		t.Error("expected name empty error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestCreateGood_InsertError: проверяем, что при ошибке INSERT возвращается соответствующая ошибка
func TestCreateGood_InsertError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()
	mockErr := errors.New("insert failed")
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO goods(project_id, name, description)")).
		WithArgs(1, "Name", sqlmock.AnyArg()).
		WillReturnError(mockErr)
	_, err := repo.CreateGood(ctx, 1, "Name", nil)
	if err == nil || !strings.Contains(err.Error(), mockErr.Error()) {
		t.Errorf("expected insert error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// Тест получения товара по идентификатору:
// 1) Успешное чтение данных из БД
// 2) Обработка случая, когда запись не найдена (ErrNotFound)
func TestGetGood(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()

	// успешный сценарий
	createdAt := time.Now()
	columns := []string{"id", "project_id", "name", "description", "priority", "removed", "created_at"}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, priority, removed, created_at FROM goods WHERE id=$1 AND project_id=$2")).
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1, 2, "Name", "Desc", 3, false, createdAt))

	good, err := repo.GetGood(ctx, 2, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if good.ID != 1 || good.ProjectID != 2 || good.Name != "Name" {
		t.Error("unexpected good fields")
	}

	// не найдено
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, priority, removed, created_at FROM goods WHERE id=$1 AND project_id=$2")).
		WithArgs(3, 4).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetGood(ctx, 4, 3)
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected ErrNotFound")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestGetGood_QueryError: проверяем прокидку произвольной ошибки при SELECT
func TestGetGood_QueryError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()
	mockErr := errors.New("timeout")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, priority, removed, created_at FROM goods WHERE id=$1 AND project_id=$2")).
		WithArgs(2, 1).
		WillReturnError(mockErr)
	_, err := repo.GetGood(ctx, 1, 2)
	if err == nil || !strings.Contains(err.Error(), mockErr.Error()) {
		t.Errorf("expected query error, got %v", err)
	}
}

// Тест обновления товара (UpdateGood):
// 1) Успешный сценарий: SELECT FOR UPDATE + UPDATE + COMMIT
// 2) Обработка пустого имени (валидация)
// 3) Обработка отсутствия записи (ErrNotFound)
func TestUpdateGood(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()

	// успешный сценарий
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, priority, removed, created_at FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE")).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "description", "priority", "removed", "created_at"}).
			AddRow(1, 1, "Old", "OldDesc", 2, false, time.Now()))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE goods SET name=$1, description=$2 WHERE id=$3 AND project_id=$4")).
		WithArgs("New", "NewDesc", 1, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	good, err := repo.UpdateGood(ctx, 1, 1, "New", ptr("NewDesc"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if good.Name != "New" {
		t.Error("name not updated")
	}

	// пустое имя
	_, err = repo.UpdateGood(ctx, 1, 1, "", ptr("d"))
	if !errors.Is(err, errors.New("name cannot be empty")) {
		t.Error("expected empty name error")
	}

	// not found
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, priority, removed, created_at FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE")).
		WithArgs(2, 2).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.UpdateGood(ctx, 2, 2, "N", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected ErrNotFound")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestUpdateGood_ExecError: проверяем, что при ошибке Exec внутри транзакции происходит Rollback и возвращается ошибка
func TestUpdateGood_ExecError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, priority, removed, created_at FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE")).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "description", "priority", "removed", "created_at"}).
			AddRow(1, 1, "Old", nil, 2, false, time.Now()))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE goods SET name=$1, description=$2 WHERE id=$3 AND project_id=$4")).
		WithArgs("New", nil, 1, 1).
		WillReturnError(errors.New("exec failed"))
	mock.ExpectRollback()
	_, err := repo.UpdateGood(ctx, 1, 1, "New", nil)
	if err == nil || !strings.Contains(err.Error(), "exec failed") {
		t.Errorf("expected exec error, got %v", err)
	}
}

// TestUpdateGood_CommitError: проверяем, что при ошибке Commit транзакции возвращается ошибка
func TestUpdateGood_CommitError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, name, description, priority, removed, created_at FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE")).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "description", "priority", "removed", "created_at"}).
			AddRow(1, 1, "Old", "Desc", 2, false, time.Now()))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE goods SET name=$1, description=$2 WHERE id=$3 AND project_id=$4")).
		WithArgs("New", nil, 1, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
	_, err := repo.UpdateGood(ctx, 1, 1, "New", nil)
	if err == nil || !strings.Contains(err.Error(), "commit failed") {
		t.Errorf("expected commit error, got %v", err)
	}
}

// Тест удаления товара (RemoveGood):
// 1) Успешный сценарий: SELECT FOR UPDATE + UPDATE removed=true + COMMIT
// 2) Обработка случая, когда запись не найдена (ErrNotFound)
func TestRemoveGood(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()

	// успешный сценарий
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE")).
		WithArgs(5, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE goods SET removed=true WHERE id=$1 AND project_id=$2")).
		WithArgs(5, 5).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.RemoveGood(ctx, 5, 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// not found
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE")).
		WithArgs(6, 6).
		WillReturnError(sql.ErrNoRows)

	err = repo.RemoveGood(ctx, 6, 6)
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected ErrNotFound")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestRemoveGood_ExecError: проверяем Rollback и возврат ошибки при ошибке в UPDATE removed
func TestRemoveGood_ExecError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE")).
		WithArgs(5, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE goods SET removed=true WHERE id=$1 AND project_id=$2")).
		WithArgs(5, 5).
		WillReturnError(errors.New("remove exec failed"))
	mock.ExpectRollback()
	err := repo.RemoveGood(ctx, 5, 5)
	if err == nil || !strings.Contains(err.Error(), "remove exec failed") {
		t.Errorf("expected remove exec error, got %v", err)
	}
}

// TestRemoveGood_CommitError: проверяем возврат ошибки при ошибке Commit после удаления
func TestRemoveGood_CommitError(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	repo := NewGoodRepository(db)
	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE")).
		WithArgs(5, 5).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE goods SET removed=true WHERE id=$1 AND project_id=$2")).
		WithArgs(5, 5).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("remove commit failed"))
	err := repo.RemoveGood(ctx, 5, 5)
	if err == nil || !strings.Contains(err.Error(), "remove commit failed") {
		t.Errorf("expected remove commit error, got %v", err)
	}
}

// ptr возвращает указатель на строку, используется для передачи nullable description в тестах
func ptr(s string) *string {
	return &s
}

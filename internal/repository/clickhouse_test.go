package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"HezzlTestTask/internal/model"
)

// ptrString возвращает указатель на строку
func ptrString(s string) *string {
	return &s
}

func TestBatchInsertLogs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	repo := NewClickhouseRepo(db)
	defer db.Close()

	events := []model.Good{
		{ID: 1, ProjectID: 2, Name: "test", Description: ptrString("desc"), Priority: 5, Removed: true},
	}

	// Ожидаем начало транзакции
	mock.ExpectBegin()
	// Ожидаем подготовку запроса
	mock.ExpectPrepare("INSERT INTO events_log").
		ExpectExec().
		WithArgs(1, 2, "test", "desc", 5, uint8(1), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Ожидаем коммит
	mock.ExpectCommit()

	err = repo.BatchInsertLogs(context.Background(), events)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

package repository

import (
	"context"
	"database/sql"
	"log"
	"time"

	"HezzlTestTask/internal/model"
)

// ClickhouseRepo реализует пакетную запись событий логов в ClickHouse
// Все комментарии на русском языке
type ClickhouseRepo struct {
	db *sql.DB
}

// NewClickhouseRepo создаёт новый репозиторий для ClickHouse
func NewClickhouseRepo(db *sql.DB) *ClickhouseRepo {
	return &ClickhouseRepo{db: db}
}

// BatchInsertLogs записывает пакет логов событий в таблицу events_log в ClickHouse
// Событие содержит данные из модели Good и время события теперь
func (r *ClickhouseRepo) BatchInsertLogs(ctx context.Context, events []model.Good) error {
	// начинаем 'транзакцию' для batch insert (clickhouse-go собирает блок при PrepareContext)
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	// логируем количество событий для вставки
	log.Printf("Начало пакетной вставки %d событий в ClickHouse", len(events))
	// PrepareContext для одной строки; clickhouse-go будет собирать несколько Exec в один блок
	query := `INSERT INTO events_log (Id, ProjectId, Name, Description, Priority, Removed, EventTime) VALUES (?, ?, ?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer func() { _ = stmt.Close() }()
	// выполняем ExecContext для каждой записи; драйвер соберёт весь пакет
	for _, e := range events {
		desc := ""
		if e.Description != nil {
			desc = *e.Description
		}
		_, err := stmt.ExecContext(ctx,
			e.ID, e.ProjectID, e.Name,
			desc, e.Priority, boolToUInt8(e.Removed),
			time.Now(),
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	// коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return err
	}
	// логируем успешную вставку
	log.Printf("Успешно вставлено %d событий в ClickHouse", len(events))
	return nil
}

// boolToUInt8 конвертирует bool в UInt8 (0/1)
func boolToUInt8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

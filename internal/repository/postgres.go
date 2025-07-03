package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"HezzlTestTask/internal/model"
)

// ErrNotFound возвращается при отсутствии записи
var ErrNotFound = errors.New("record not found")

// ErrEmptyName возвращается при попытке создания или обновления с пустым именем
var ErrEmptyName = &emptyNameError{}

type emptyNameError struct{}

func (e *emptyNameError) Error() string {
	return "name cannot be empty"
}

func (e *emptyNameError) Is(target error) bool {
	return target != nil && target.Error() == e.Error()
}

// GoodRepository реализует доступ к таблице goods
type GoodRepository struct {
	db *sql.DB
}

// NewGoodRepository создает новый репозиторий товаров
func NewGoodRepository(db *sql.DB) *GoodRepository {
	return &GoodRepository{db: db}
}

// CreateGood добавляет новый товар в таблицу goods
func (r *GoodRepository) CreateGood(ctx context.Context, projectID int, name string, description *string) (*model.Good, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	// вставляем запись, priority, removed и created_at обрабатываются триггером и дефолтами в БД
	query := `INSERT INTO goods(project_id, name, description) VALUES($1, $2, $3)
		RETURNING id, priority, removed, created_at`
	var id, priority int
	var removed bool
	var createdAt time.Time
	err := r.db.QueryRowContext(ctx, query, projectID, name, description).
		Scan(&id, &priority, &removed, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert good: %w", err)
	}
	return &model.Good{
		ID:          id,
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		Priority:    priority,
		Removed:     removed,
		CreatedAt:   createdAt,
	}, nil
}

// GetGood возвращает товар по id и projectID
func (r *GoodRepository) GetGood(ctx context.Context, projectID, id int) (*model.Good, error) {
	query := `SELECT id, project_id, name, description, priority, removed, created_at
		FROM goods WHERE id=$1 AND project_id=$2`
	row := r.db.QueryRowContext(ctx, query, id, projectID)
	var g model.Good
	err := row.Scan(&g.ID, &g.ProjectID, &g.Name, &g.Description, &g.Priority, &g.Removed, &g.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get good: %w", err)
	}
	return &g, nil
}

// UpdateGood обновляет поля name и description товара, с блокировкой и транзакцией
func (r *GoodRepository) UpdateGood(ctx context.Context, projectID, id int, name string, description *string) (*model.Good, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	// выборка с блокировкой
	selectQuery := `SELECT id, project_id, name, description, priority, removed, created_at
		FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE`
	row := tx.QueryRowContext(ctx, selectQuery, id, projectID)
	var g model.Good
	err = row.Scan(&g.ID, &g.ProjectID, &g.Name, &g.Description, &g.Priority, &g.Removed, &g.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to select good for update: %w", err)
	}
	// обновление полей
	updateQuery := `UPDATE goods SET name=$1, description=$2 WHERE id=$3 AND project_id=$4`
	_, err = tx.ExecContext(ctx, updateQuery, name, description, id, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to update good: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	// возвращаем обновленную запись
	g.Name = name
	g.Description = description
	return &g, nil
}

// RemoveGood устанавливает removed=true для записи товара с блокировкой и транзакцией
func (r *GoodRepository) RemoveGood(ctx context.Context, projectID, id int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	// проверка существования с блокировкой
	selectQuery := `SELECT id FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE`
	row := tx.QueryRowContext(ctx, selectQuery, id, projectID)
	var existingID int
	if err := row.Scan(&existingID); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("failed to select good for remove: %w", err)
	}
	// установка removed
	_, err = tx.ExecContext(ctx, `UPDATE goods SET removed=true WHERE id=$1 AND project_id=$2`, id, projectID)
	if err != nil {
		return fmt.Errorf("failed to remove good: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// ListGoods возвращает список товаров с пагинацией и информацию о количестве записей
func (r *GoodRepository) ListGoods(ctx context.Context, limit, offset int) ([]model.Good, int, int, error) {
	// получаем общее число записей и число удаленных
	var total, removed int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM goods`).Scan(&total); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to count goods: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM goods WHERE removed=true`).Scan(&removed); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to count removed goods: %w", err)
	}
	// получаем список с пагинацией
	rows, err := r.db.QueryContext(ctx, `SELECT id, project_id, name, description, priority, removed, created_at FROM goods ORDER BY id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to select goods list: %w", err)
	}
	defer rows.Close()
	var goods []model.Good
	for rows.Next() {
		var g model.Good
		if err := rows.Scan(&g.ID, &g.ProjectID, &g.Name, &g.Description, &g.Priority, &g.Removed, &g.CreatedAt); err != nil {
			return nil, 0, 0, fmt.Errorf("failed to scan good: %w", err)
		}
		goods = append(goods, g)
	}
	return goods, total, removed, nil
}

// Reprioritize изменяет приоритет товара и сдвигает приоритеты других записей
func (r *GoodRepository) Reprioritize(ctx context.Context, projectID, id, newPriority int) ([]model.PriorityUpdate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	// получаем текущий приоритет с блокировкой
	var currPriority int
	row := tx.QueryRowContext(ctx, `SELECT priority FROM goods WHERE id=$1 AND project_id=$2 FOR UPDATE`, id, projectID)
	if err := row.Scan(&currPriority); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to select good for reprioritize: %w", err)
	}
	var updates []model.PriorityUpdate
	// сдвигаем приоритеты в зависимости от нового значения
	if newPriority < currPriority {
		// сдвигаем +1 для тех, чей priority в [newPriority, currPriority)
		rows, err := tx.QueryContext(ctx, `UPDATE goods SET priority = priority + 1 WHERE project_id=$1 AND priority >= $2 AND priority < $3 RETURNING id, priority`, projectID, newPriority, currPriority)
		if err != nil {
			return nil, fmt.Errorf("failed to shift priorities up: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var pu model.PriorityUpdate
			if err := rows.Scan(&pu.ID, &pu.Priority); err != nil {
				return nil, fmt.Errorf("failed to scan shifted priority: %w", err)
			}
			updates = append(updates, pu)
		}
	} else if newPriority > currPriority {
		// сдвигаем -1 для тех, чей priority в (currPriority, newPriority]
		rows, err := tx.QueryContext(ctx, `UPDATE goods SET priority = priority - 1 WHERE project_id=$1 AND priority > $2 AND priority <= $3 RETURNING id, priority`, projectID, currPriority, newPriority)
		if err != nil {
			return nil, fmt.Errorf("failed to shift priorities down: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var pu model.PriorityUpdate
			if err := rows.Scan(&pu.ID, &pu.Priority); err != nil {
				return nil, fmt.Errorf("failed to scan shifted priority: %w", err)
			}
			updates = append(updates, pu)
		}
	}
	// обновляем приоритет текущего товара
	_, err = tx.ExecContext(ctx, `UPDATE goods SET priority=$1 WHERE id=$2 AND project_id=$3`, newPriority, id, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to update priority of good: %w", err)
	}
	updates = append(updates, model.PriorityUpdate{ID: id, Priority: newPriority})
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return updates, nil
}

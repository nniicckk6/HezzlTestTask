package model

import "time"

// Project представляет проект (таблица projects)
type Project struct {
	ID        int       `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

// Good представляет сущность товара (таблица goods)
type Good struct {
	ID          int       `db:"id" json:"id"`
	ProjectID   int       `db:"project_id" json:"projectId"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	Priority    int       `db:"priority" json:"priority"`
	Removed     bool      `db:"removed" json:"removed"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

// PriorityUpdate представляет изменение приоритета товара
// ID — идентификатор товара, Priority — новый приоритет
type PriorityUpdate struct {
	ID       int `db:"id" json:"id"`
	Priority int `db:"priority" json:"priority"`
}

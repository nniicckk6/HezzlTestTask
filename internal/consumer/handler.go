package consumer

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"HezzlTestTask/internal/model"
)

// Repo описывает интерфейс репозитория ClickHouse для пакетной записи логов
// Метод BatchInsertLogs записывает слайс событий model.Good
// Все комментарии на русском языке
type Repo interface {
	BatchInsertLogs(ctx context.Context, events []model.Good) error
}

// Consumer буферизует события и отправляет их пакетно в ClickHouse
// batchSize определяет макс. количество событий до отправки
// mutex защищает доступ к буферу events

type Consumer struct {
	repo      Repo
	batchSize int
	events    []model.Good
	mu        sync.Mutex
}

// NewConsumer создаёт Consumer с указанным репозиторием и размером пакета
func NewConsumer(repo Repo, batchSize int) *Consumer {
	return &Consumer{repo: repo, batchSize: batchSize, events: make([]model.Good, 0, batchSize)}
}

// HandleMessage обрабатывает сообщение из NATS: парсит JSON, добавляет событие в буфер и при достижении batchSize отправляет в ClickHouse
func (c *Consumer) HandleMessage(ctx context.Context, data []byte) error {
	// логируем получение сообщения
	log.Printf("Получено сообщение NATS: %s", string(data))
	// парсим данные в модель Good
	var g model.Good
	if err := json.Unmarshal(data, &g); err != nil {
		return err
	}
	// логируем распарсенное событие
	log.Printf("Получено событие для логирования: %+v", g)
	c.mu.Lock()
	c.events = append(c.events, g)
	// если достигли batchSize, сбрасываем буфер
	if len(c.events) >= c.batchSize {
		eventsCopy := make([]model.Good, len(c.events))
		copy(eventsCopy, c.events)
		c.events = c.events[:0]
		c.mu.Unlock()
		// отправляем пакет логов
		return c.repo.BatchInsertLogs(ctx, eventsCopy)
	}
	c.mu.Unlock()
	return nil
}

// Flush отправляет все накопленные события, если они есть
func (c *Consumer) Flush(ctx context.Context) error {
	c.mu.Lock()
	if len(c.events) == 0 {
		c.mu.Unlock()
		return nil
	}
	eventsCopy := make([]model.Good, len(c.events))
	copy(eventsCopy, c.events)
	c.events = c.events[:0]
	c.mu.Unlock()
	return c.repo.BatchInsertLogs(ctx, eventsCopy)
}

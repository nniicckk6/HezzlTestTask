package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"HezzlTestTask/internal/model"
)

// Repo определяет интерфейс репозитория для операций с товарами (CRUD и приоритеты)
// Реализация может быть на основе базы данных Postgres
// Методы возвращают сущности model.Good и возможные ошибки
type Repo interface {
	CreateGood(ctx context.Context, projectID int, name string, description *string) (*model.Good, error)
	GetGood(ctx context.Context, projectID, id int) (*model.Good, error)
	UpdateGood(ctx context.Context, projectID, id int, name string, description *string) (*model.Good, error)
	RemoveGood(ctx context.Context, projectID, id int) error
	ListGoods(ctx context.Context, limit, offset int) ([]model.Good, int, int, error)
	Reprioritize(ctx context.Context, projectID, id, newPriority int) ([]model.PriorityUpdate, error)
}

// Cache определяет интерфейс кэширования результатов операций (Redis)
// Методы позволяют записывать, читать и инвалидировать кэш по ключу
type Cache interface {
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	Invalidate(ctx context.Context, key string) error
}

// Logger определяет интерфейс логгирования событий (NATS)
// Метод PublishLog отправляет лог-сообщение в брокер сообщений
type Logger interface {
	PublishLog(data []byte) error
}

// cacheTTL задаёт время жизни записей в кэше (Redis), по умолчанию 1 минута или из REDIS_TTL
var cacheTTL = time.Minute

func init() {
	if v := os.Getenv("REDIS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cacheTTL = d
		}
	}
}

// GoodsService реализует бизнес-логику для сущности товара:
// - проверка входных данных (валидация)
// - вызовы репозитория для CRUD операций
// - кэширование результатов и инвалидирование
// - публикация событий в лог
// Все комментарии на русском языке

type GoodsService struct {
	repo   Repo
	cache  Cache
	logger Logger
}

// NewGoodsService создаёт новый сервис для товаров
func NewGoodsService(r Repo, c Cache, l Logger) *GoodsService {
	return &GoodsService{repo: r, cache: c, logger: l}
}

// Create создаёт новый товар в базе и возвращает его:
// 1. Валидирует, что имя не пустое
// 2. Вызывает метод репозитория CreateGood
// 3. Инвалидирует кэш списка товаров и кэш конкретного товара
// 4. Публикует сериализованный в JSON объект товара в лог
func (s *GoodsService) Create(ctx context.Context, projectID int, name string, description *string) (*model.Good, error) {
	// валидация: имя не должно быть пустым
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}
	// создаём товар в БД
	good, err := s.repo.CreateGood(ctx, projectID, name, description)
	if err != nil {
		return nil, err
	}
	// инвалидируем кэш для списка и конкретного товара
	_ = s.cache.Invalidate(ctx, fmt.Sprintf("goods:list"))
	_ = s.cache.Invalidate(ctx, fmt.Sprintf("good:%d:%d", projectID, good.ID))
	// публикуем лог события в NATS
	data, _ := json.Marshal(good)
	_ = s.logger.PublishLog(data)
	return good, nil
}

// Get возвращает товар по id и projectID:
// 1. Пытается получить из кэша Redis
// 2. При промахе кэша запрашивает из репозитория
// 3. Сохраняет результат в кэш
func (s *GoodsService) Get(ctx context.Context, projectID, id int) (*model.Good, error) {
	key := fmt.Sprintf("good:%d:%d", projectID, id)
	// пытаемся получить из кэша
	bytes, err := s.cache.Get(ctx, key)
	if err == nil {
		var g model.Good
		_ = json.Unmarshal(bytes, &g)
		return &g, nil
	}
	// при ошибке redis.Nil или ErrCacheMiss получаем из БД
	good, err := s.repo.GetGood(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	// кэшируем результат
	data, _ := json.Marshal(good)
	_ = s.cache.Set(ctx, key, data, cacheTTL)
	return good, nil
}

// Update обновляет поля товара:
// 1. Валидирует, что новое имя не пустое
// 2. Вызывает метод репозитория UpdateGood
// 3. Инвалидирует кэш и публикует обновлённый объект
func (s *GoodsService) Update(ctx context.Context, projectID, id int, name string, description *string) (*model.Good, error) {
	// валидация: имя не должно быть пустым
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}
	good, err := s.repo.UpdateGood(ctx, projectID, id, name, description)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Invalidate(ctx, "goods:list")
	_ = s.cache.Invalidate(ctx, fmt.Sprintf("good:%d:%d", projectID, id))
	data, _ := json.Marshal(good)
	_ = s.logger.PublishLog(data)
	return good, nil
}

// Remove помечает товар как удалённый и публикует полный объект:
// 1. Получает существующий объект через GetGood
// 2. Вызывает RemoveGood для логического удаления
// 3. Инвалидирует кэш списка и объекта
// 4. Устанавливает флаг Removed и публикует объект в лог
func (s *GoodsService) Remove(ctx context.Context, projectID, id int) error {
	// получаем существующий товар
	good, err := s.repo.GetGood(ctx, projectID, id)
	if err != nil {
		return err
	}
	// удаляем товар
	if err := s.repo.RemoveGood(ctx, projectID, id); err != nil {
		return err
	}
	// инвалидируем кэш
	_ = s.cache.Invalidate(ctx, "goods:list")
	_ = s.cache.Invalidate(ctx, fmt.Sprintf("good:%d:%d", projectID, id))
	// помечаем объект как удалённый и отправляем в лог полный объект
	good.Removed = true
	data, _ := json.Marshal(good)
	if err := s.logger.PublishLog(data); err != nil {
		return err
	}
	return nil
}

// List возвращает список товаров с метаданными:
// 1. Пытается получить из кэша по ключу с limit/offset
// 2. При промахе кэша запрашивает из репозитория
// 3. Кэширует ответ (массив товаров и мета)
func (s *GoodsService) List(ctx context.Context, limit, offset int) ([]model.Good, int, int, error) {
	key := fmt.Sprintf("goods:list:%d:%d", limit, offset)
	// пытаемся получить из кэша
	if bytes, err := s.cache.Get(ctx, key); err == nil {
		var resp struct {
			Goods []model.Good `json:"goods"`
			Meta  struct {
				Total   int `json:"total"`
				Removed int `json:"removed"`
				Limit   int `json:"limit"`
				Offset  int `json:"offset"`
			} `json:"meta"`
		}
		_ = json.Unmarshal(bytes, &resp)
		return resp.Goods, resp.Meta.Total, resp.Meta.Removed, nil
	}
	// из БД
	goods, total, removed, err := s.repo.ListGoods(ctx, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	// кэшируем ответ
	resp := struct {
		Goods []model.Good `json:"goods"`
		Meta  struct {
			Total   int `json:"total"`
			Removed int `json:"removed"`
			Limit   int `json:"limit"`
			Offset  int `json:"offset"`
		} `json:"meta"`
	}{
		Goods: goods,
		Meta: struct {
			Total   int `json:"total"`
			Removed int `json:"removed"`
			Limit   int `json:"limit"`
			Offset  int `json:"offset"`
		}{
			Total:   total,
			Removed: removed,
			Limit:   limit,
			Offset:  offset,
		},
	}
	data, _ := json.Marshal(resp)
	_ = s.cache.Set(ctx, key, data, cacheTTL)
	return goods, total, removed, nil
}

// Reprioritize изменяет приоритет заданного товара и возвращает обновления:
// 1. Вызывает метод репозитория Reprioritize
// 2. Инвалидирует кэш
// 3. Публикует массив обновлённых приоритетов в лог
func (s *GoodsService) Reprioritize(ctx context.Context, projectID, id, newPriority int) ([]model.PriorityUpdate, error) {
	updates, err := s.repo.Reprioritize(ctx, projectID, id, newPriority)
	if err != nil {
		return nil, err
	}
	// инвалидируем кэш списка и конкретного товара
	_ = s.cache.Invalidate(ctx, "goods:list")
	_ = s.cache.Invalidate(ctx, fmt.Sprintf("good:%d:%d", projectID, id))
	// публикуем лог изменений
	data, _ := json.Marshal(updates)
	_ = s.logger.PublishLog(data)
	return updates, nil
}

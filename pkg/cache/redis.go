// Пакет cache предоставляет обёртку для работы с Redis как кешем
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

// ErrCacheMiss возвращается, когда запрошенный ключ отсутствует в кеше Redis.
// Используется для явного отличия ситуации кэш-промаха от других ошибок Redis.
var ErrCacheMiss = errors.New("cache miss")

// RedisClient представляет собой обёртку над *redis.Client,
// упрощающую работу с методами Set, Get и Del и обработку ошибок.
type RedisClient struct {
	client *redis.Client // внутренний клиент для работы с Redis
}

// NewRedisClient создаёт новый RedisClient с заданными опциями подключения.
// opts позволяет указать адрес сервера, пароль и другие параметры конфигурации.
func NewRedisClient(opts *redis.Options) *RedisClient {
	return &RedisClient{client: redis.NewClient(opts)}
}

// Set сохраняет значение value под ключом key с указанным временем жизни expiration.
// Возвращает ошибку, если операция записи завершилась неудачей.
func (r *RedisClient) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	// Rely on redis.Client.Set – метод возвращает тип StatusCmd,
	// .Err() возвращает ошибку, если команда не выполнена успешно.
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Get пытается получить значение по ключу key из кеша.
// Если ключ не найден (Redis возвращает redis.Nil), возвращается ErrCacheMiss,
// иначе при других ошибках возвращается оригинальная ошибка.
// В случае успеха возвращается массив байт, сохранённый под ключом.
func (r *RedisClient) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		// кэш-промах: ключ отсутствует
		return nil, ErrCacheMiss
	}
	if err != nil {
		// любая другая ошибка Redis
		return nil, err
	}
	// успешное получение значения
	return data, nil
}

// Invalidate удаляет ключ key из кеша Redis.
// Используется для инвалидирования устаревших или изменённых данных.
// Возвращает ошибку, если операция удаления не удалась.
func (r *RedisClient) Invalidate(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Пакет cache содержит unit-тесты для проверки работы RedisClient: Set, Get и Invalidate
package cache

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	// библиотека-мок для эмуляции Redis клиента
	redismock "github.com/go-redis/redismock/v8"
)

// TestSetGetInvalidate проверяет корректную работу методов Set, Get (hit и miss) и Invalidate
func TestSetGetInvalidate(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := &RedisClient{client: db}
	ctx := context.Background()
	key := "key"
	val := []byte("value")
	exp := time.Minute

	// Set
	mock.ExpectSet(key, val, exp).SetVal("OK")
	if err := client.Set(ctx, key, val, exp); err != nil {
		t.Errorf("Set error: %v", err)
	}

	// Get hit
	mock.ExpectGet(key).SetVal(string(val))
	got, err := client.Get(ctx, key)
	if err != nil {
		t.Errorf("Get error: %v", err)
	}
	if string(got) != string(val) {
		t.Errorf("Get expected %s, got %s", val, got)
	}

	// Get miss
	mock.ExpectGet("missing").RedisNil()
	_, err = client.Get(ctx, "missing")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss, got %v", err)
	}

	// Invalidate
	mock.ExpectDel(key).SetVal(1)
	if err := client.Invalidate(ctx, key); err != nil {
		t.Errorf("Invalidate error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestSet_Error проверяет возвращение ошибки при неудаче операции Set
func TestSet_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := &RedisClient{client: db}
	ctx := context.Background()
	key := "key"
	val := []byte("value")
	exp := time.Minute
	mock.ExpectSet(key, val, exp).SetErr(errors.New("set failed"))
	err := client.Set(ctx, key, val, exp)
	if err == nil || !strings.Contains(err.Error(), "set failed") {
		t.Errorf("expected set error, got %v", err)
	}
}

// TestGet_OtherError проверяет возвращение произвольной ошибки при Get, не связанной с cache miss
func TestGet_OtherError(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := &RedisClient{client: db}
	ctx := context.Background()
	mock.ExpectGet("key").SetErr(errors.New("get failed"))
	_, err := client.Get(ctx, "key")
	if err == nil || !strings.Contains(err.Error(), "get failed") {
		t.Errorf("expected get error, got %v", err)
	}
}

// TestInvalidate_Error проверяет возвращение ошибки при неудаче операции Invalidate (Del)
func TestInvalidate_Error(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := &RedisClient{client: db}
	ctx := context.Background()
	mock.ExpectDel("key").SetErr(errors.New("del failed"))
	err := client.Invalidate(ctx, "key")
	if err == nil || !strings.Contains(err.Error(), "del failed") {
		t.Errorf("expected invalidate error, got %v", err)
	}
}

package service

import (
	cachepkg "HezzlTestTask/pkg/cache"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"HezzlTestTask/internal/model"
	"HezzlTestTask/internal/repository"
)

// mockRepo реализует интерфейс репозитория для тестирования сервиса GoodsService.
// Поля-функции позволяют настроить возвращаемые значения и ошибки для каждого метода:
// - createFn: поведение CreateGood
// - getFn: поведение GetGood
// - updateFn: поведение UpdateGood
// - removeFn: поведение RemoveGood
// - listFn: поведение ListGoods
// - reprioritizeFn: поведение Reprioritize
type mockRepo struct {
	createFn       func(ctx context.Context, projectID int, name string, description *string) (*model.Good, error)
	getFn          func(ctx context.Context, projectID, id int) (*model.Good, error)
	updateFn       func(ctx context.Context, projectID, id int, name string, description *string) (*model.Good, error)
	removeFn       func(ctx context.Context, projectID, id int) error
	listFn         func(ctx context.Context, limit, offset int) ([]model.Good, int, int, error)
	reprioritizeFn func(ctx context.Context, projectID, id, newPriority int) ([]model.PriorityUpdate, error)
}

func (m *mockRepo) CreateGood(ctx context.Context, projectID int, name string, description *string) (*model.Good, error) {
	return m.createFn(ctx, projectID, name, description)
}
func (m *mockRepo) GetGood(ctx context.Context, projectID, id int) (*model.Good, error) {
	if m.getFn != nil {
		return m.getFn(ctx, projectID, id)
	}
	// по умолчанию возвращаем объект без ошибки, чтобы не паниковать
	return &model.Good{ID: id, ProjectID: projectID}, nil
}
func (m *mockRepo) UpdateGood(ctx context.Context, projectID, id int, name string, description *string) (*model.Good, error) {
	return m.updateFn(ctx, projectID, id, name, description)
}
func (m *mockRepo) RemoveGood(ctx context.Context, projectID, id int) error {
	return m.removeFn(ctx, projectID, id)
}
func (m *mockRepo) ListGoods(ctx context.Context, limit, offset int) ([]model.Good, int, int, error) {
	return m.listFn(ctx, limit, offset)
}
func (m *mockRepo) Reprioritize(ctx context.Context, projectID, id, newPriority int) ([]model.PriorityUpdate, error) {
	return m.reprioritizeFn(ctx, projectID, id, newPriority)
}

// mockCache симулирует кэш Redis с настраиваемым поведением методов
// - set: сохраняет данные
// - get: получает данные
// - inval: инвалидирует ключ
type mockCache struct {
	set   func(ctx context.Context, key string, value []byte, ttl time.Duration) error
	get   func(ctx context.Context, key string) ([]byte, error)
	inval func(ctx context.Context, key string) error
}

func (m *mockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.set == nil {
		return nil
	}
	return m.set(ctx, key, value, ttl)
}
func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if m.get == nil {
		return nil, cachepkg.ErrCacheMiss
	}
	return m.get(ctx, key)
}
func (m *mockCache) Invalidate(ctx context.Context, key string) error {
	if m.inval == nil {
		return nil
	}
	return m.inval(ctx, key)
}

// mockLogger симулирует логгер, принимает данные для публикации
// pub: функция, записывающая переданное сообщение
type mockLogger struct {
	pub func(data []byte) error
}

func (m *mockLogger) PublishLog(data []byte) error {
	return m.pub(data)
}

func newService(repo *mockRepo, cache *mockCache, logger *mockLogger) *GoodsService {
	return &GoodsService{repo: repo, cache: cache, logger: logger}
}

// TestCreate_Success проверяет сценарий успешного создания товара
func TestCreate_Success(t *testing.T) {
	// Arrange: настраиваем репозиторий-заглушку, возвращающую готовый объект good
	good := &model.Good{ID: 1, ProjectID: 10, Name: "n", Description: ptr("d"), Priority: 1}
	repo := &mockRepo{createFn: func(ctx context.Context, projectID int, name string, description *string) (*model.Good, error) {
		// Act stub: проверяем, что переданы ожидаемые projectID, name и description
		if projectID != 10 || name != "n" || description == nil || *description != "d" {
			t.Fatalf("unexpected args: projectID=%d, name=%s, description=%v", projectID, name, description)
		}
		// Возвращаем заранее подготовленный объект без ошибки
		return good, nil
	}}
	// Arrange: готовим срез для проверки ключей, которые инвалидируются в кеше
	var keysInvalidated []string
	cache := &mockCache{
		inval: func(ctx context.Context, key string) error {
			// накапливаем переданные ключи
			keysInvalidated = append(keysInvalidated, key)
			return nil
		},
	}
	// Arrange: настраиваем логгер-заглушку, записывающую публикуемые данные
	var logged []byte
	logger := &mockLogger{pub: func(data []byte) error {
		// сохраняем payload для последующей проверки
		logged = data
		return nil
	}}
	// Act: создаём сервис и вызываем Create
	s := newService(repo, cache, logger)
	r, err := s.Create(context.Background(), 10, "n", ptr("d"))
	// Assert: проверяем, что ошибки нет и возвращён правильный объект
	if err != nil || !reflect.DeepEqual(r, good) {
		t.Fatalf("Create returned %v, %v, want %v, nil", r, err, good)
	}
	// Assert: проверяем, что кэш инвалидировался дважды (список и конкретный товар)
	if len(keysInvalidated) != 2 {
		t.Fatalf("expected 2 cache invalidations, got %d", len(keysInvalidated))
	}
	// Assert: проверяем содержимое лог-сообщения
	var out model.Good
	_ = json.Unmarshal(logged, &out)
	if out.ID != good.ID || out.Name != good.Name {
		t.Fatalf("logged payload mismatch, got %+v", out)
	}
}

// TestCreate_EmptyName проверяет, что при пустом имени возвращается ошибка валидации
func TestCreate_EmptyName(t *testing.T) {
	repo := &mockRepo{createFn: nil}
	cache := &mockCache{}
	logger := &mockLogger{}
	s := newService(repo, cache, logger)
	_, err := s.Create(context.Background(), 1, "", nil)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

// TestGet_Success проверяет получение товара при промахе кэша
func TestGet_Success(t *testing.T) {
	// Arrange: настраиваем заглушку кэша, возвращающую ErrCacheMiss
	repoData := &model.Good{ID: 2, ProjectID: 3, Name: "a"}
	repo := &mockRepo{getFn: func(ctx context.Context, projectID, id int) (*model.Good, error) {
		// Act stub: проверяем корректность аргументов вызова GetGood
		if projectID != 3 || id != 2 {
			t.Fatalf("unexpected repo args: projectID=%d, id=%d", projectID, id)
		}
		// Возвращаем данные из репозитория
		return repoData, nil
	}}
	cache := &mockCache{get: func(ctx context.Context, key string) ([]byte, error) {
		// эмулируем cache miss
		return nil, cachepkg.ErrCacheMiss
	}}
	logger := &mockLogger{pub: func(data []byte) error {
		// при cache miss лог не вызывается для Get
		return nil
	}}
	// Act: вызываем Get
	s := newService(repo, cache, logger)
	g, err := s.Get(context.Background(), 3, 2)
	// Assert: проверяем, что данные из репозитория возвращены без ошибок
	if err != nil || !reflect.DeepEqual(g, repoData) {
		t.Fatalf("Get returned %v, %v; want %v, nil", g, err, repoData)
	}
}

// TestGet_FromCache проверяет получение товара напрямую из кэша без вызова репозитория
func TestGet_FromCache(t *testing.T) {
	// Arrange: сериализуем ожидаемый объект в JSON и настраиваем кэш-заглушку
	exp := &model.Good{ID: 5, ProjectID: 1, Name: "c"}
	data, _ := json.Marshal(exp)
	repo := &mockRepo{} // репозиторий не должен вызываться
	cache := &mockCache{get: func(ctx context.Context, key string) ([]byte, error) {
		// возвращаем заранее сериализованный объект
		return data, nil
	}}
	logger := &mockLogger{pub: func(data []byte) error {
		// проверяем, что лог при чтении из кэша не вызывается
		t.Fatal("logger should not be called on cache hit")
		return nil
	}}
	// Act: вызываем Get, должно вернуть объект из кэша
	s := newService(repo, cache, logger)
	g, err := s.Get(context.Background(), 1, 5)
	// Assert: объект и ошибка
	if err != nil || !reflect.DeepEqual(g, exp) {
		t.Fatalf("Get cache returned %v, %v; want %v, nil", g, err, exp)
	}
}

// TestGet_Error проверяет обработку ошибки репозитория при попытке получить товар
func TestGet_Error(t *testing.T) {
	testErr := errors.New("repo error")
	repo := &mockRepo{getFn: func(ctx context.Context, projectID, id int) (*model.Good, error) {
		return nil, testErr
	}}
	cache := &mockCache{get: func(ctx context.Context, key string) ([]byte, error) { return nil, cachepkg.ErrCacheMiss }}
	logger := &mockLogger{}
	s := newService(repo, cache, logger)
	_, err := s.Get(context.Background(), 1, 1)
	if err != testErr {
		t.Fatalf("expected error %v, got %v", testErr, err)
	}
}

// TestUpdate_Success проверяет сценарий успешного обновления товара
func TestUpdate_Success(t *testing.T) {
	exp := &model.Good{ID: 3, ProjectID: 4, Name: "u"}
	repo := &mockRepo{updateFn: func(ctx context.Context, projectID, id int, name string, description *string) (*model.Good, error) {
		return exp, nil
	}}
	var inv []string
	cache := &mockCache{inval: func(ctx context.Context, key string) error { inv = append(inv, key); return nil }}
	logger := &mockLogger{pub: func(data []byte) error { return nil }}
	s := newService(repo, cache, logger)
	g, err := s.Update(context.Background(), 4, 3, "u", nil)
	if err != nil || !reflect.DeepEqual(g, exp) {
		t.Fatal("Update failed")
	}
	if len(inv) != 2 {
		t.Fatal("invalidate")
	}
}

// TestUpdate_EmptyName проверяет ошибку при попытке обновить товар с пустым именем
func TestUpdate_EmptyName(t *testing.T) {
	repo := &mockRepo{}
	cache := &mockCache{}
	logger := &mockLogger{}
	s := newService(repo, cache, logger)
	_, err := s.Update(context.Background(), 1, 1, "", nil)
	if err == nil {
		t.Fatal("empty name")
	}
}

// TestUpdate_NotFound проверяет возврат ErrNotFound при обновлении несуществующего товара
func TestUpdate_NotFound(t *testing.T) {
	repo := &mockRepo{updateFn: func(ctx context.Context, projectID, id int, name string, description *string) (*model.Good, error) {
		return nil, repository.ErrNotFound
	}}
	cache := &mockCache{}
	logger := &mockLogger{}
	s := newService(repo, cache, logger)
	_, err := s.Update(context.Background(), 1, 1, "name", nil)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestRemove_Success проверяет успешное логическое удаление товара и публикацию лога
func TestRemove_Success(t *testing.T) {
	repo := &mockRepo{removeFn: func(ctx context.Context, projectID, id int) error { return nil }}
	var inv []string
	cache := &mockCache{inval: func(ctx context.Context, key string) error { inv = append(inv, key); return nil }}
	var logged []byte
	logger := &mockLogger{pub: func(data []byte) error { logged = data; return nil }}
	s := newService(repo, cache, logger)
	err := s.Remove(context.Background(), 7, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv) != 2 {
		t.Fatal("invalidate")
	}
	var out model.Good
	_ = json.Unmarshal(logged, &out)
	if !out.Removed {
		t.Fatal("log removed")
	}
}

// TestRemove_GetError проверяет ситуацию, когда не удалось получить товар для удаления
func TestRemove_GetError(t *testing.T) {
	testErr := errors.New("get error")
	repo := &mockRepo{getFn: func(ctx context.Context, projectID, id int) (*model.Good, error) {
		return nil, testErr
	}}
	s := newService(repo, &mockCache{}, &mockLogger{})
	err := s.Remove(context.Background(), 1, 1)
	if err != testErr {
		t.Fatalf("expected error %v, got %v", testErr, err)
	}
}

// TestRemove_RemoveError проверяет обработку ошибки удаления товара в репозитории
func TestRemove_RemoveError(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, projectID, id int) (*model.Good, error) {
			return &model.Good{ID: id, ProjectID: projectID}, nil
		},
		removeFn: func(ctx context.Context, projectID, id int) error {
			return errors.New("remove error")
		},
	}
	s := newService(repo, &mockCache{}, &mockLogger{})
	err := s.Remove(context.Background(), 1, 1)
	if err == nil || err.Error() != "remove error" {
		t.Fatalf("expected remove error, got %v", err)
	}
}

// TestRemove_NotFound проверяет возвращаемый ErrNotFound при отсутствии товара
func TestRemove_NotFound(t *testing.T) {
	repo := &mockRepo{removeFn: func(ctx context.Context, projectID, id int) error { return repository.ErrNotFound }}
	s := newService(repo, &mockCache{}, &mockLogger{})
	err := s.Remove(context.Background(), 1, 1)
	if err != repository.ErrNotFound {
		t.Fatal("expected notfound")
	}
}

// TestList_Success проверяет успешное получение списка товаров и запись в кэш
func TestList_Success(t *testing.T) {
	list := []model.Good{{ID: 9, ProjectID: 1, Name: "x"}}
	repo := &mockRepo{listFn: func(ctx context.Context, limit, offset int) ([]model.Good, int, int, error) { return list, 5, 1, nil }}
	var cached []byte
	cache := &mockCache{set: func(ctx context.Context, key string, value []byte, ttl time.Duration) error {
		cached = value
		return nil
	}}
	logger := &mockLogger{pub: func(data []byte) error { return nil }}
	s := newService(repo, cache, logger)
	goods, total, removed, err := s.List(context.Background(), 2, 3)
	if err != nil || total != 5 || removed != 1 || !reflect.DeepEqual(goods, list) {
		t.Fatal("List failed")
	}
	if len(cached) == 0 {
		t.Fatal("cache set")
	}
}

// TestList_CacheHit проверяет получение списка товаров из кэша без вызова БД
func TestList_CacheHit(t *testing.T) {
	goods := []model.Good{{ID: 1}}
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
	}
	resp.Meta.Total = 2
	resp.Meta.Removed = 1
	resp.Meta.Limit = 5
	resp.Meta.Offset = 0
	data, _ := json.Marshal(resp)
	repo := &mockRepo{}
	cache := &mockCache{get: func(ctx context.Context, key string) ([]byte, error) { return data, nil }}
	logger := &mockLogger{}
	s := newService(repo, cache, logger)
	gotGoods, total, removed, err := s.List(context.Background(), 5, 0)
	if err != nil {
		t.Fatalf("List cache hit returned error: %v", err)
	}
	if total != resp.Meta.Total || removed != resp.Meta.Removed || !reflect.DeepEqual(gotGoods, goods) {
		t.Fatalf("List cache hit: got %v, %v, %v want %v, %v, %v", gotGoods, total, removed, goods, resp.Meta.Total, resp.Meta.Removed)
	}
}

// TestList_ServiceError проверяет обработку ошибки репозитория при получении списка
func TestList_ServiceError(t *testing.T) {
	testErr := errors.New("service error")
	repo := &mockRepo{listFn: func(ctx context.Context, limit, offset int) ([]model.Good, int, int, error) {
		return nil, 0, 0, testErr
	}}
	cache := &mockCache{}
	logger := &mockLogger{}
	s := newService(repo, cache, logger)
	_, _, _, err := s.List(context.Background(), 0, 0)
	if err == nil || err.Error() != testErr.Error() {
		t.Fatalf("expected error %v, got %v", testErr, err)
	}
}

// TestReprioritize_Success проверяет успешное изменение приоритетов и публикацию лога
func TestReprioritize_Success(t *testing.T) {
	exp := []model.PriorityUpdate{{ID: 1, Priority: 2}}
	repo := &mockRepo{reprioritizeFn: func(ctx context.Context, projectID, id, new int) ([]model.PriorityUpdate, error) { return exp, nil }}
	var inv []string
	cache := &mockCache{inval: func(ctx context.Context, key string) error { inv = append(inv, key); return nil }}
	var logged []byte
	logger := &mockLogger{pub: func(data []byte) error { logged = data; return nil }}
	s := newService(repo, cache, logger)
	ups, err := s.Reprioritize(context.Background(), 2, 3, 4)
	if err != nil || !reflect.DeepEqual(ups, exp) {
		t.Fatal("repr failed")
	}
	if len(inv) != 2 {
		t.Fatal("invalidate repr")
	}
	// log содержит JSON массив
	var arr []model.PriorityUpdate
	_ = json.Unmarshal(logged, &arr)
	if !reflect.DeepEqual(arr, exp) {
		t.Fatal("log repr")
	}
}

// TestReprioritize_Error проверяет обработку ошибки при пересортировке приоритетов
func TestReprioritize_Error(t *testing.T) {
	testErr := errors.New("repr error")
	repo := &mockRepo{reprioritizeFn: func(ctx context.Context, projectID, id, newPriority int) ([]model.PriorityUpdate, error) {
		return nil, testErr
	}}
	cache := &mockCache{}
	logger := &mockLogger{}
	s := newService(repo, cache, logger)
	_, err := s.Reprioritize(context.Background(), 1, 1, 2)
	if err != testErr {
		t.Fatalf("expected error %v, got %v", testErr, err)
	}
}

// helper
func ptr(s string) *string { return &s }

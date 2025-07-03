package http

import (
	"HezzlTestTask/internal/repository"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"HezzlTestTask/internal/model"
)

// mockService реализует GoodsService для тестирования HTTP-хендлера.
// Поля-функции позволяют контролировать возвращаемые сервисом данные и ошибки:
// - CreateFn: stub для обработки Create
// - GetFn: stub для обработки Get
// - UpdateFn: stub для обработки Update
// - RemoveFn: stub для обработки Remove
// - ListFn: stub для обработки List
// - ReprioritizeFn: stub для обработки Reprioritize
// Во время теста в этих функциях можно проверять переданные аргументы и эмулировать разные сценарии.
type mockService struct {
	CreateFn       func(projectID int, name string, description *string) (*model.Good, error)
	GetFn          func(projectID, id int) (*model.Good, error)
	UpdateFn       func(projectID, id int, name string, description *string) (*model.Good, error)
	RemoveFn       func(projectID, id int) error
	ListFn         func(limit, offset int) ([]model.Good, int, int, error)
	ReprioritizeFn func(projectID, id, newPriority int) ([]model.PriorityUpdate, error)
}

func (m *mockService) Create(_ context.Context, projectID int, name string, description *string) (*model.Good, error) {
	return m.CreateFn(projectID, name, description)
}
func (m *mockService) Get(_ context.Context, projectID, id int) (*model.Good, error) {
	return m.GetFn(projectID, id)
}
func (m *mockService) Update(_ context.Context, projectID, id int, name string, description *string) (*model.Good, error) {
	return m.UpdateFn(projectID, id, name, description)
}
func (m *mockService) Remove(_ context.Context, projectID, id int) error {
	return m.RemoveFn(projectID, id)
}
func (m *mockService) List(_ context.Context, limit, offset int) ([]model.Good, int, int, error) {
	return m.ListFn(limit, offset)
}
func (m *mockService) Reprioritize(_ context.Context, projectID, id, newPriority int) ([]model.PriorityUpdate, error) {
	return m.ReprioritizeFn(projectID, id, newPriority)
}

// TestCreate_Success проверяет корректную обработку успешной операции создания товара через HTTP запрос
func TestCreate_Success(t *testing.T) {
	ms := &mockService{}
	expected := &model.Good{ID: 1, ProjectID: 2, Name: "test", Description: ptr("d"), Priority: 1, Removed: false}
	ms.CreateFn = func(projectID int, name string, description *string) (*model.Good, error) {
		// Arrange: ожидаемые значения projectID, name и description
		if projectID != 2 || name != "test" || *description != "d" {
			t.Fatalf("unexpected args %d %s %v", projectID, name, description)
		}
		// Act: возврат ожидаемого товара
		return expected, nil
	}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	// make request
	reqBody := `{"name":"test","description":"d"}`
	req := httptest.NewRequest(http.MethodPost, "/good/create?projectId=2", bytes.NewBufferString(reqBody))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusOK {
		t.Fatalf("status = %d", rq.Code)
	}
	var got model.Good
	_ = json.Unmarshal(rq.Body.Bytes(), &got)
	if !reflect.DeepEqual(&got, expected) {
		t.Fatalf("got %+v, want %+v", got, expected)
	}
}

// TestGet_NotFound проверяет возврат 404 при обращении к несуществующему товару
func TestGet_NotFound(t *testing.T) {
	ms := &mockService{}
	notFound := repository.ErrNotFound
	ms.GetFn = func(projectID, id int) (*model.Good, error) { return nil, notFound }
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/good/get?projectId=1&id=10", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rq.Code)
	}
}

// TestCreate_InvalidProjectId проверяет возврат 400 при некорректном projectId в запросе создания товара
func TestCreate_InvalidProjectId(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/good/create?projectId=abc", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rq.Code)
	}
}

// TestCreate_InvalidJSON проверяет возврат 400 при некорректном JSON в теле запроса создания товара
func TestCreate_InvalidJSON(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPost, "/good/create?projectId=1", bytes.NewBufferString(`invalid`))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rq.Code)
	}
}

// TestCreate_ServiceError проверяет возврат 500 при ошибке сервиса Create
func TestCreate_ServiceError(t *testing.T) {
	errTest := errors.New("create fail")
	ms := &mockService{CreateFn: func(projectID int, name string, description *string) (*model.Good, error) {
		return nil, errTest
	}}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	body := `{"name":"n"}`
	req := httptest.NewRequest(http.MethodPost, "/good/create?projectId=1", bytes.NewBufferString(body))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rq.Code)
	}
}

// TestGet_InvalidParams проверяет возврат 400 при некорректных параметрах id или projectId в запросе GET
func TestGet_InvalidParams(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/good/get?projectId=x&id=y", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rq.Code)
	}
}

// TestGet_ServiceError проверяет возврат 500 при ошибке сервиса Get
func TestGet_ServiceError(t *testing.T) {
	errTest := errors.New("get fail")
	ms := &mockService{GetFn: func(projectID, id int) (*model.Good, error) { return nil, errTest }}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/good/get?projectId=1&id=1", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rq.Code)
	}
}

// TestUpdate_Success проверяет корректную обработку успешного обновления товара через HTTP PATCH
func TestUpdate_Success(t *testing.T) {
	ms := &mockService{}
	expected := &model.Good{ID: 5, ProjectID: 3, Name: "upd", Description: ptr("x"), Priority: 2}
	ms.UpdateFn = func(projectID, id int, name string, description *string) (*model.Good, error) {
		// Arrange: ожидаемые значения projectID, id, name и description
		if projectID != 3 || id != 5 || name != "upd" || *description != "x" {
			t.Fatalf("unexpected args %d %d %s %v", projectID, id, name, description)
		}
		// Act: возврат ожидаемого товара
		return expected, nil
	}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	body := `{"name":"upd","description":"x"}`
	req := httptest.NewRequest(http.MethodPatch, "/good/update?projectId=3&id=5", bytes.NewBufferString(body))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusOK {
		t.Fatalf("status = %d", rq.Code)
	}
	var got model.Good
	_ = json.Unmarshal(rq.Body.Bytes(), &got)
	if !reflect.DeepEqual(&got, expected) {
		t.Fatalf("got %+v, want %+v", got, expected)
	}
}

// TestUpdate_NotFound проверяет возврат 404 при обновлении несуществующего товара
func TestUpdate_NotFound(t *testing.T) {
	ms := &mockService{}
	ms.UpdateFn = func(projectID, id int, name string, description *string) (*model.Good, error) {
		return nil, repository.ErrNotFound
	}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPatch, "/good/update?projectId=1&id=1", bytes.NewBufferString(`{"name":"n","description":"d"}`))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rq.Code)
	}
}

// TestUpdate_InvalidParams проверяет возврат 400 при некорректных параметрах projectId или id
func TestUpdate_InvalidParams(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPatch, "/good/update?projectId=a&id=b", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rq.Code)
	}
}

// TestUpdate_InvalidJSON проверяет возврат 400 при некорректном JSON в теле PATCH запроса
func TestUpdate_InvalidJSON(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPatch, "/good/update?projectId=1&id=1", bytes.NewBufferString(`bad`))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rq.Code)
	}
}

// TestUpdate_ServiceError проверяет возврат 500 при ошибке сервиса Update
func TestUpdate_ServiceError(t *testing.T) {
	errTest := errors.New("update fail")
	ms := &mockService{UpdateFn: func(projectID, id int, name string, description *string) (*model.Good, error) { return nil, errTest }}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	body := `{"name":"n"}`
	req := httptest.NewRequest(http.MethodPatch, "/good/update?projectId=1&id=1", bytes.NewBufferString(body))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rq.Code)
	}
}

// TestRemove_Success проверяет корректное логическое удаление товара через HTTP DELETE
func TestRemove_Success(t *testing.T) {
	ms := &mockService{}
	ms.RemoveFn = func(projectID, id int) error {
		// Arrange: ожидаемые значения projectID и id
		if projectID != 4 || id != 2 {
			t.Fatal("bad args")
		}
		// Act: успешное удаление товара
		return nil
	}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodDelete, "/good/remove?projectId=4&id=2", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusOK {
		t.Fatalf("status = %d", rq.Code)
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(rq.Body.Bytes(), &resp)
	if resp["removed"] != true {
		t.Fatalf("removed flag")
	}
}

// TestRemove_NotFound проверяет возврат 404 при попытке удалить несуществующий товар
func TestRemove_NotFound(t *testing.T) {
	ms := &mockService{}
	ms.RemoveFn = func(projectID, id int) error { return repository.ErrNotFound }
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodDelete, "/good/remove?projectId=1&id=1", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rq.Code)
	}
}

// TestRemove_InvalidParams проверяет возврат 400 при некорректных параметрах запроса удаления
func TestRemove_InvalidParams(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodDelete, "/good/remove?projectId=x&id=y", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rq.Code)
	}
}

// TestRemove_ServiceError проверяет возврат 500 при ошибке сервиса Remove
func TestRemove_ServiceError(t *testing.T) {
	ms := &mockService{RemoveFn: func(projectID, id int) error { return errors.New("remove fail") }}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodDelete, "/good/remove?projectId=1&id=1", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rq.Code)
	}
}

// TestList_Success проверяет корректный возврат списка товаров с учетом параметров limit и offset
func TestList_Success(t *testing.T) {
	ms := &mockService{}
	goods := []model.Good{{ID: 1, ProjectID: 1, Name: "a", Priority: 1, Removed: false, CreatedAt: time.Now()}}
	ms.ListFn = func(limit, offset int) ([]model.Good, int, int, error) { return goods, 10, 2, nil }
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/goods/list?limit=5&offset=1", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusOK {
		t.Fatalf("status = %d", rq.Code)
	}
	var out struct {
		Meta struct {
			Total   int
			Removed int
			Limit   int
			Offset  int
		}
		Goods []model.Good
	}
	_ = json.Unmarshal(rq.Body.Bytes(), &out)
	if out.Meta.Total != 10 || out.Meta.Removed != 2 || out.Meta.Limit != 5 || out.Meta.Offset != 1 {
		t.Fatal("meta")
	}
}

// TestList_ServiceError проверяет возврат 500 при ошибке сервиса List
func TestList_ServiceError(t *testing.T) {
	ms := &mockService{ListFn: func(limit, offset int) ([]model.Good, int, int, error) { return nil, 0, 0, errors.New("list fail") }}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/goods/list?limit=1&offset=0", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rq.Code)
	}
}

// TestReprioritize_Success проверяет корректную обработку пересортировки приоритетов через HTTP PATCH
func TestReprioritize_Success(t *testing.T) {
	ms := &mockService{}
	updates := []model.PriorityUpdate{{ID: 1, Priority: 2}}
	ms.ReprioritizeFn = func(projectID, id, newPriority int) ([]model.PriorityUpdate, error) {
		// Arrange: ожидаемые значения projectID, id и newPriority
		if projectID != 7 || id != 3 || newPriority != 5 {
			t.Fatal("args")
		}
		// Act: возврат ожидаемого обновления приоритета
		return updates, nil
	}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPatch, "/good/reprioritize?projectId=7&id=3", bytes.NewBufferString(`{"newPriority":5}`))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusOK {
		t.Fatalf("status = %d", rq.Code)
	}
	var resp map[string][]model.PriorityUpdate
	_ = json.Unmarshal(rq.Body.Bytes(), &resp)
	if !reflect.DeepEqual(resp["priorities"], updates) {
		t.Fatal("priorities")
	}
}

// TestReprioritize_InvalidParams проверяет возврат 400 при некорректных параметрах запроса приоритизации
func TestReprioritize_InvalidParams(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPatch, "/good/reprioritize?projectId=a&id=b", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rq.Code)
	}
}

// TestReprioritize_InvalidJSON проверяет возврат 400 при некорректном JSON в теле запроса приоритизации
func TestReprioritize_InvalidJSON(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodPatch, "/good/reprioritize?projectId=1&id=1", bytes.NewBufferString(`bad`))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rq.Code)
	}
}

// TestReprioritize_ServiceError проверяет возврат 500 при ошибке сервиса Reprioritize
func TestReprioritize_ServiceError(t *testing.T) {
	ms := &mockService{ReprioritizeFn: func(projectID, id, newPriority int) ([]model.PriorityUpdate, error) {
		return nil, errors.New("repr fail")
	}}
	h := NewHandler(ms)
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	body := `{"newPriority":1}`
	req := httptest.NewRequest(http.MethodPatch, "/good/reprioritize?projectId=1&id=1", bytes.NewBufferString(body))
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rq.Code)
	}
}

// TestHealthz проверяет корректный ответ эндпоинта /healthz
func TestHealthz(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rq.Code)
	}
	expected := `{"status":"ok"}`
	if strings.TrimSpace(rq.Body.String()) != expected {
		t.Fatalf("body = %s, want %s", rq.Body.String(), expected)
	}
}

// TestReadyz проверяет корректный ответ эндпоинта /readyz
func TestReadyz(t *testing.T) {
	h := NewHandler(&mockService{})
	r := mux.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rq := httptest.NewRecorder()
	r.ServeHTTP(rq, req)
	if rq.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rq.Code)
	}
	expected := `{"status":"ready"}`
	if strings.TrimSpace(rq.Body.String()) != expected {
		t.Fatalf("body = %s, want %s", rq.Body.String(), expected)
	}
}

// helper ptr используется для создания указателя на строку в тестовых данных
func ptr(s string) *string { return &s }

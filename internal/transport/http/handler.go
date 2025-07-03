package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"HezzlTestTask/internal/model"
	"HezzlTestTask/internal/repository"
)

// GoodsService задаёт интерфейс бизнес-логики для HTTP-слоя, используемый хендлером
// Методы соответствуют CRUD-операциям и управлению приоритетом
type GoodsService interface {
	Create(ctx context.Context, projectID int, name string, description *string) (*model.Good, error)
	Get(ctx context.Context, projectID, id int) (*model.Good, error)
	Update(ctx context.Context, projectID, id int, name string, description *string) (*model.Good, error)
	Remove(ctx context.Context, projectID, id int) error
	List(ctx context.Context, limit, offset int) ([]model.Good, int, int, error)
	Reprioritize(ctx context.Context, projectID, id, newPriority int) ([]model.PriorityUpdate, error)
}

// Handler содержит зависимости и реализует HTTP-эндпоинты для операций с товарами
type Handler struct {
	srv GoodsService
}

// NewHandler создаёт новый HTTP Handler
func NewHandler(srv GoodsService) *Handler {
	return &Handler{srv: srv}
}

// RegisterRoutes регистрирует маршруты API
func (h *Handler) RegisterRoutes(r *mux.Router) {
	// Эндпоинты для проверки здоровья и готовности сервиса
	r.HandleFunc("/healthz", h.Healthz).Methods("GET")
	r.HandleFunc("/readyz", h.Readyz).Methods("GET")
	r.HandleFunc("/good/create", h.Create).Methods("POST")
	r.HandleFunc("/good/update", h.Update).Methods("PATCH")
	r.HandleFunc("/good/remove", h.Remove).Methods("DELETE")
	r.HandleFunc("/good/get", h.Get).Methods("GET")
	r.HandleFunc("/goods/list", h.List).Methods("GET")
	r.HandleFunc("/good/reprioritize", h.Reprioritize).Methods("PATCH")
}

// ErrorResponse модель ошибки API
type ErrorResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details"`
}

func writeError(w http.ResponseWriter, status int, resp ErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// Create обрабатывает POST /good/create
// 1. Парсит projectId из query
// 2. Декодирует тело запроса в структуру с полями name и description
// 3. Вызывает метод сервиса Create
// 4. В случае ошибки возвращает соответствующий HTTP-статус
// 5. При успешном создании возвращает JSON созданного товара
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pidStr := r.URL.Query().Get("projectId")
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid projectId", map[string]interface{}{}})
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid request body", map[string]interface{}{}})
		return
	}
	good, err := h.srv.Create(r.Context(), pid, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrorResponse{1, err.Error(), map[string]interface{}{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(good)
}

// Update обрабатывает PATCH /good/update
// 1. Извлекает projectId и id через parseIDs
// 2. Декодирует тело в поля name и description
// 3. Вызывает сервис Update, обрабатывает ErrNotFound и другие ошибки
// 4. Возвращает JSON обновлённого товара или ошибку
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	pid, id, ok := parseIDs(r)
	if !ok {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid projectId or id", map[string]interface{}{}})
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid request body", map[string]interface{}{}})
		return
	}
	good, err := h.srv.Update(r.Context(), pid, id, req.Name, req.Description)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, ErrorResponse{3, "errors.common.notFound", map[string]interface{}{}})
		} else {
			writeError(w, http.StatusInternalServerError, ErrorResponse{1, err.Error(), map[string]interface{}{}})
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(good)
}

// Remove обрабатывает DELETE /good/remove
// 1. Извлекает projectId и id через parseIDs
// 2. Вызывает сервис Remove, обрабатывает ErrNotFound и другие ошибки
// 3. При успешном удалении возвращает JSON {id, campaignId, removed: true}
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	pid, id, ok := parseIDs(r)
	if !ok {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid projectId or id", map[string]interface{}{}})
		return
	}
	err := h.srv.Remove(r.Context(), pid, id)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, ErrorResponse{3, "errors.common.notFound", map[string]interface{}{}})
		} else {
			writeError(w, http.StatusInternalServerError, ErrorResponse{1, err.Error(), map[string]interface{}{}})
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "campaignId": pid, "removed": true})
}

// Get обрабатывает GET /good/get
// 1. Парсит id и projectId из query, валидирует
// 2. Вызывает сервис Get, обрабатывает ErrNotFound и другие ошибки
// 3. При успехе возвращает JSON товара
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid id", map[string]interface{}{}})
		return
	}
	pidStr := r.URL.Query().Get("projectId")
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid projectId", map[string]interface{}{}})
		return
	}
	good, err := h.srv.Get(r.Context(), pid, id)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, ErrorResponse{3, "errors.common.notFound", map[string]interface{}{}})
		} else {
			writeError(w, http.StatusInternalServerError, ErrorResponse{1, err.Error(), map[string]interface{}{}})
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(good)
}

// List обрабатывает GET /goods/list
// 1. Читает optional параметры limit, offset (по умолчанию 10 и 0)
// 2. Вызывает сервис List, обрабатывает ошибки
// 3. Возвращает JSON с полем meta (total, removed, limit, offset) и массив goods
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := 10, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			limit = i
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			offset = i
		}
	}
	goods, total, removed, err := h.srv.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrorResponse{1, err.Error(), map[string]interface{}{}})
		return
	}
	resp := struct {
		Meta struct {
			Total   int `json:"total"`
			Removed int `json:"removed"`
			Limit   int `json:"limit"`
			Offset  int `json:"offset"`
		} `json:"meta"`
		Goods []model.Good `json:"goods"`
	}{
		Meta: struct {
			Total   int `json:"total"`
			Removed int `json:"removed"`
			Limit   int `json:"limit"`
			Offset  int `json:"offset"`
		}{Total: total, Removed: removed, Limit: limit, Offset: offset},
		Goods: goods,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Reprioritize обрабатывает PATCH /good/reprioritize
// 1. Извлекает projectId и id через parseIDs
// 2. Декодирует тело запроса в поле newPriority
// 3. Вызывает сервис Reprioritize, обрабатывает ErrNotFound и другие ошибки
// 4. Возвращает JSON с полем priorities (массив обновлений)
func (h *Handler) Reprioritize(w http.ResponseWriter, r *http.Request) {
	pid, id, ok := parseIDs(r)
	if !ok {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid projectId or id", map[string]interface{}{}})
		return
	}
	var req struct {
		NewPriority int `json:"newPriority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrorResponse{1, "invalid request body", map[string]interface{}{}})
		return
	}
	updates, err := h.srv.Reprioritize(r.Context(), pid, id, req.NewPriority)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, ErrorResponse{3, "errors.common.notFound", map[string]interface{}{}})
		} else {
			writeError(w, http.StatusInternalServerError, ErrorResponse{1, err.Error(), map[string]interface{}{}})
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"priorities": updates})
}

// Healthz возвращает статус работы сервиса
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Readyz возвращает готовность сервиса
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

// parseIDs извлекает и валидирует projectId и id из query parameters
// Возвращает (projectId, id, ok)
// ok=false при ошибке парсинга или если значения <=0
func parseIDs(r *http.Request) (int, int, bool) {
	pid, err1 := strconv.Atoi(r.URL.Query().Get("projectId"))
	id, err2 := strconv.Atoi(r.URL.Query().Get("id"))
	if err1 != nil || err2 != nil || pid <= 0 || id <= 0 {
		return 0, 0, false
	}
	return pid, id, true
}

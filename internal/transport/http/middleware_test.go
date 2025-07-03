package http

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLoggingMiddleware_Success проверяет, что middleware логирует запрос без паники
func TestLoggingMiddleware_Success(t *testing.T) {
	// Перенаправляем вывод логов в буфер
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	// Простая цель-обработчик, возвращает 201 и тело
	handler := LoggingMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPut, "/test-path?x=1", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	// Проверяем код ответа и тело
	if rw.Code != http.StatusCreated {
		t.Fatalf("ожидался статус %d, получили %d", http.StatusCreated, rw.Code)
	}
	if got := rw.Body.String(); got != "ok" {
		t.Fatalf("ожидалось тело 'ok', получили '%s'", got)
	}

	// Проверяем, что в логах есть метод, путь и статус
	out := buf.String()
	if !strings.Contains(out, "PUT /test-path") {
		t.Errorf("ожидалось упоминание метода и пути, получили: %s", out)
	}
	if !strings.Contains(out, "201") {
		t.Errorf("ожидалось упоминание статуса 201, получили: %s", out)
	}
}

// TestLoggingMiddleware_Panic проверяет, что middleware логирует панику и пробрасывает её дальше
func TestLoggingMiddleware_Panic(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	h := LoggingMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom error")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rw := httptest.NewRecorder()

	// Ожидаем панику, чтобы middleware её повторно пробросил
	defer func() {
		if rec := recover(); rec == nil {
			t.Fatalf("ожидалась паника, но её не было")
		}
		out := buf.String()
		// Должна быть строка с "PANIC" и путь
		if !strings.Contains(out, "PANIC GET /panic") {
			t.Errorf("ожидалось логирование паники, получили: %s", out)
		}
	}()

	h.ServeHTTP(rw, req)
}

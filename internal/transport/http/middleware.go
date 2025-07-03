package http

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// statusResponseWriter обёртка для http.ResponseWriter, чтобы захватывать статус-код
// и передавать его дальше
type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader сохраняет статус и вызывает оригинальный WriteHeader
func (w *statusResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware выводит в стандартный лог информацию о каждом HTTP-запросе и панике
func LoggingMiddleware(_ interface{}) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			srw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
			// обработка паники
			defer func() {
				if rec := recover(); rec != nil {
					dur := time.Since(start).Milliseconds()
					log.Printf("PANIC %s %s 500 %dms: %v", r.Method, r.URL.Path, dur, rec)
					panic(rec)
				}
			}()
			next.ServeHTTP(srw, r)
			dur := time.Since(start).Milliseconds()
			log.Printf("%s %s %d %dms", r.Method, r.URL.Path, srw.status, dur)
		})
	}
}

package main

import (
	"HezzlTestTask/internal/repository"
	"HezzlTestTask/internal/service"
	externalHttp "HezzlTestTask/internal/transport/http"
	"HezzlTestTask/pkg/cache"
	"HezzlTestTask/pkg/logger"
	"context"
	"database/sql"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	nats "github.com/nats-io/nats.go"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// читаем переменные окружения
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		log.Printf("DB_NAME не задан, используем базу по умолчанию 'appdb'")
		dbName = "appdb"
	}
	natsURL := os.Getenv("NATS_URL")
	natsSubject := os.Getenv("NATS_SUBJECT")
	if natsSubject == "" {
		natsSubject = "goods"
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	// подключаем Postgres
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to Postgres: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping Postgres: %v", err)
	}

	// Применяем миграции Postgres с помощью golang-migrate
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("failed to create migrate driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations/postgres", "postgres", driver,
	)
	if err != nil {
		log.Fatalf("failed to create migrate instance: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("failed to apply migrations: %v", err)
	}

	// подключаем Redis
	rClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	cacheClient := cache.NewRedisClient(rClient.Options())
	// подключаем NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("failed to connect to NATS: %v", err)
	}
	loggerClient := logger.NewClient(nc, natsSubject)
	// создаем репозиторий и сервис
	repo := repository.NewGoodRepository(db)
	srv := service.NewGoodsService(repo, cacheClient, loggerClient)
	// настраиваем HTTP маршруты
	// подключаем middleware для логирования HTTP-запросов
	r := mux.NewRouter()
	r.Use(externalHttp.LoggingMiddleware(loggerClient))
	h := externalHttp.NewHandler(srv)
	h.RegisterRoutes(r)
	// запускаем HTTP сервер с поддержкой graceful shutdown
	addr := ":8080"
	srvHttp := &http.Server{Addr: addr, Handler: r}
	// запуск сервера в горутине
	go func() {
		log.Printf("starting server at %s", addr)
		if err := srvHttp.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()
	// ожидаем сигнал для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Printf("shutting down server...")
	// контекст с таймаутом для остановки
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srvHttp.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	log.Printf("server exited properly")
	// закрываем Redis-клиент
	if err := rClient.Close(); err != nil {
		log.Printf("failed to close Redis client: %v", err)
	}
	// корректно дренируем и закрываем NATS-соединение
	if err := nc.Drain(); err != nil {
		log.Printf("failed to drain NATS connection: %v", err)
	}
	nc.Close()
}

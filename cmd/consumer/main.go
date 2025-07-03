package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/nats-io/nats.go"

	"HezzlTestTask/internal/consumer"
	"HezzlTestTask/internal/repository"
	_ "github.com/ClickHouse/clickhouse-go"
)

func main() {
	// Читаем конфигурацию из окружения
	natsURL := os.Getenv("NATS_URL")
	subject := os.Getenv("NATS_SUBJECT")
	dsn := os.Getenv("CLICKHOUSE_DSN")
	batchSize := 10
	if v := os.Getenv("BATCH_SIZE"); v != "" {
		if bs, err := strconv.Atoi(v); err == nil {
			batchSize = bs
		} else {
			log.Fatalf("invalid BATCH_SIZE: %v", err)
		}
	}

	// Подключаемся к NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	// Подключаемся к ClickHouse (appdb должна быть создана SQL-скриптами)
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		log.Fatalf("failed to connect to ClickHouse: %v", err)
	}
	// закрываем соединение с ClickHouse
	defer func() { _ = db.Close() }()

	// Применяем миграции ClickHouse с помощью golang-migrate
	driver, err := clickhouse.WithInstance(db, &clickhouse.Config{})
	if err != nil {
		log.Fatalf("failed to create ClickHouse migrate driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations/clickhouse", "clickhouse", driver,
	)
	if err != nil {
		log.Fatalf("failed to create ClickHouse migrate instance: %v", err)
	}

	// Применяем все up миграции
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("failed to apply ClickHouse migrations: %v", err)
	}

	// Создаём репозиторий и консьюмера
	repo := repository.NewClickhouseRepo(db)
	cons := consumer.NewConsumer(repo, batchSize)

	// Запускаем HTTP-сервер для healthz и readyz
	port := os.Getenv("CONSUMER_PORT")
	if port == "" {
		port = "8081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	// создаем HTTP сервер для health
	healthSrv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		log.Printf("starting health server on :%s", port)
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("health server failed: %v", err)
		}
	}()

	// Подписываемся на тему NATS
	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		// Обрабатываем сообщение в контексте Background
		if err := cons.HandleMessage(context.Background(), msg.Data); err != nil {
			log.Printf("failed to handle message: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("failed to subscribe to subject %s: %v", subject, err)
	}
	// Ждём сигнала завершения
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Printf("shutting down consumer...")
	// контекст с таймаутом для остановки
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// останавливаем HTTP сервер
	if err := healthSrv.Shutdown(ctx); err != nil {
		log.Printf("health server shutdown failed: %v", err)
	}

	// Отписываемся и сбрасываем оставшиеся события
	if err := sub.Unsubscribe(); err != nil {
		log.Printf("failed to unsubscribe: %v", err)
	}
	if err := cons.Flush(context.Background()); err != nil {
		log.Printf("failed to flush consumer events: %v", err)
	}
}

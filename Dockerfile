# Dockerfile для API-сервиса (Go)
# сборка бинарника
FROM golang:1.24-alpine AS builder
WORKDIR /app
# установка зависимостей
COPY go.mod go.sum ./
RUN go mod download
# копируем исходники и собираем
COPY . .
RUN go build -o server cmd/app/main.go

# минимальный образ для запуска
FROM alpine:latest
WORKDIR /app
# копируем бинарник и необходимые файлы
COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations
# порты
EXPOSE 8080
# команда запуска
CMD ["./server"]

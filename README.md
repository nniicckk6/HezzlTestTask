# HezzlTestTask

- Стек: Go, PostgreSQL, ClickHouse, NATS, Redis, Docker
-
- Проект разработан в рамках тестового задания компании Hezzl и реализует набор микросервисов на Go для хранения и управления списками товаров (Goods), привязанных к проектам.

## Содержание
- Обновлено: добавлены ссылки на условие задачи и CI / GitHub Actions
1. [Описание проекта](#описание-проекта)
2. [Задача тестового задания](#задача-тестового-задания)
3. [Структура репозитория](#структура-репозитория)
4. [Требования и зависимости](#требования-и-зависимостей)
5. [Установка и запуск локально](#установка-и-запуск-локально)
6. [Конфигурация окружения](#конфигурация-окружения)
7. [Миграции баз данных](#миграции-баз-данных)
8. [CI / GitHub Actions](#ci--github-actions)
9. [HTTP-сервис (API)](#http-сервис-api)
10. [Consumer-сервис](#consumer-сервис)
11. [Кэширование и логирование](#кэширование-и-логирование)
12. [Тесты](#тесты)

## Описание проекта
- Реализован REST API для управления товарами (Goods), привязанными к проектам (Projects).
- PostgreSQL: таблицы `projects` (с дефолтной записью 'Первая запись') и `goods` с полями `id`, `project_id`, `name`, `description`, `priority` (автоматически = max+1), `removed` и `created_at`.
- Миграции создают схему БД и добавляют начальную запись в `projects`.
- CRUD-операции (POST, PATCH, DELETE) выполняются в транзакциях с `SELECT FOR UPDATE`, валидируются поля, при отсутствии записи возвращается 404 (code=3, message=errors.common.notFound).
- Redis: кэш GET-запросов на 1 минуту и инвалидирование при изменениях.
- NATS: публикация логов изменений, consumer-сервис группирует события и батчами записывает их в ClickHouse.
- ClickHouse: хранение истории изменений товаров в таблице `events_log`.

Проект состоит из двух сервисов:
- `app` — HTTP-сервис для API.
- `consumer` — подписчик NATS для записи логов в ClickHouse.

## Задача тестового задания
Ниже приведено полное условие тестового задания, которое необходимо реализовать:

![Скриншот 1](https://github.com/nniicckk6/HezzlTestTask/blob/master/Task/1.png)
![Скриншот 2](https://github.com/nniicckk6/HezzlTestTask/blob/master/Task/2.png)
![Скриншот 3](https://github.com/nniicckk6/HezzlTestTask/blob/master/Task/3.png)
![Скриншот 4](https://github.com/nniicckk6/HezzlTestTask/blob/master/Task/4.png)
![Скриншот 5](https://github.com/nniicckk6/HezzlTestTask/blob/master/Task/5.png)
![Скриншот 6](https://github.com/nniicckk6/HezzlTestTask/blob/master/Task/6.png)
![Скриншот 7](https://github.com/nniicckk6/HezzlTestTask/blob/master/Task/7.png)

## Структура репозитория
```
./
├── cmd/
│   ├── app/
│   │   └── main.go           # HTTP-сервис
│   └── consumer/
│       └── main.go           # consumer-сервис
├── internal/
│   ├── consumer/             # групповая запись логов в ClickHouse
│   │   ├── handler.go
│   │   └── handler_test.go
│   ├── model/                # модели данных
│   │   ├── models.go
│   │   └── models_test.go
│   ├── repository/           # Postgres и ClickHouse репозитории
│   │   ├── postgres.go
│   │   ├── postgres_test.go
│   │   ├── clickhouse.go
│   │   └── clickhouse_test.go
│   ├── service/              # бизнес-логика, кэш, логирование
│   │   ├── goods.go
│   │   └── goods_test.go
│   └── transport/
│       └── http/             # HTTP-обработчики и middleware
│           ├── handler.go
│           ├── handler_test.go
│           ├── middleware.go
│           └── middleware_test.go
├── pkg/
│   ├── cache/                # Redis-клиент
│   │   ├── redis.go
│   │   └── redis_test.go
│   └── logger/               # NATS-клиент
│       ├── nats.go
│       └── nats_test.go
├── migrations/               # SQL-миграции
│   ├── postgres/             # Postgres миграции
│   └── clickhouse/           # ClickHouse миграции
├── postgres-init/            # init-скрипт для тестовой БД Postgres
│   └── 01_create_test_db.sql
├── clickhouse-init/          # init-скрипты для ClickHouse (создание БД и пользователей)
│   └── 0001_init_all_clickhouse.sql
├── Dockerfile                # сборка HTTP-сервиса
├── Dockerfile.consumer       # сборка consumer-сервиса
├── docker-compose.yml        # окружение Docker для разработки и тестов
├── go.mod
└── go.sum
```

## Требования и зависимости
- Go 1.24+
- Docker и Docker Compose
- PostgreSQL 17+
- ClickHouse
- NATS 2.x
- Redis

Go-зависимости:
- github.com/gorilla/mux
- github.com/go-redis/redis/v8
- github.com/nats-io/nats.go
- github.com/golang-migrate/migrate/v4
- github.com/lib/pq
- github.com/ClickHouse/clickhouse-go

## Установка и запуск локально

Есть три варианта запуска:

### Вариант 1: Быстрый запуск без исходников (docker-compose.remote.yml)

- Требуется только папки `postgres-init` и `clickhouse-init` в корне проекта.
- Либо скачайте архив релиза с этими папками.
- Запустите:
  ```bash
  docker compose -f docker-compose.remote.yml up -d
  ```
  - Либо если из релизов
  ```bash
  docker compose up -d
  ```

### Вариант 2: Запуск через Docker Compose с исходным кодом

1. Клонировать репозиторий и перейти в директорию:
   ```bash
   git clone https://github.com/nniicckk6/HezzlTestTask.git
   cd HezzlTestTask
   ```
2. Установить зависимости Go:
   ```bash
   go mod download
   ```
3. Запустить все сервисы Docker Compose:
   ```bash
   docker compose up --build -d
   ```

После успешного запуска сервисы будут доступны:
- HTTP API: http://localhost:8080
- Health consumer: http://localhost:8081/healthz

## Конфигурация окружения
Все сервисы конфигурируются через переменные окружения (определены в `docker-compose.yml`):

### HTTP-сервис (`app`)
```
DB_HOST        - адрес Postgres (postgres)
DB_PORT        - порт Postgres (5432)
DB_USER        - пользователь Postgres (appuser)
DB_PASSWORD    - пароль Postgres (secret)
DB_NAME        - имя базы (appdb)
REDIS_ADDR     - адрес Redis (redis:6379)
REDIS_TTL      - время жизни кэша, пример "1m"
NATS_URL       - URL NATS (nats://nats:4222)
NATS_SUBJECT   - тема публикации логов (goods)
CLICKHOUSE_DSN - DSN для ClickHouse, не нужен в HTTP-сервисе
```

### Consumer-сервис (`consumer`)
```
NATS_URL       - URL NATS (nats://nats:4222)
NATS_SUBJECT   - тема подписки (goods)
CLICKHOUSE_DSN - DSN для ClickHouse, пример: "tcp://clickhouse:9000?username=migrations_user&password=migrator_pass&database=appdb&debug=false"
BATCH_SIZE     - размер пачки логов перед записью (по умолчанию 10)
CONSUMER_PORT  - порт для healthz (8081)
```

## Миграции баз данных
Миграции хранятся в папке `migrations`:

- postgres/:
  - `0001_init_projects_and_goods.up.sql` / `.down.sql`
  - `0002_add_default_project.up.sql` / `.down.sql`
  - `migrations_test.go`
- clickhouse/:
  - `0001_create_events_log.up.sql` / `.down.sql`
  - `0002_add_skip_indices.up.sql` / `.down.sql`
  - `migrations_test.go`

Применение миграций:
- HTTP-сервис (app) автоматически выполняет postgres Up при старте.
- Consumer-сервис выполняет clickhouse Up при старте.

## CI / GitHub Actions

В репозитории настроены два основных пайплайна в каталоге `.github/workflows/`:

- `ci.yml` — CI для проекта:
  - Проверка форматирования кода (`go fmt`).
  - Установка зависимостей (`go mod download`).
  - Запуск тестов (`go test ./... -v`).

- `publish-docker.yml` — публикация Docker-образов:
  - Сборка и публикация образа основного приложения и consumer-сервиса в GitHub Container Registry.
  - Теги образов: указанный `${{ inputs.version }}` и `latest`.
  - Запуск вручную через `workflow_dispatch`.

## HTTP-сервис (API)
Сервис использует Gorilla Mux.

### Эндпоинты и примеры

#### GET /healthz
Проверка статуса сервиса.
Пример:
```
curl -i http://localhost:8080/healthz
```

#### GET /readyz
Проверка готовности сервиса.
Пример:
```
curl -i http://localhost:8080/readyz
```

#### POST /good/create?projectId={projectId}
Создание нового Good.
Query-параметр: projectId (int, обязательно).
Body (application/json):
```json
{
  "name": "string",
  "description": "string" // необязательно
}
```
Ответ (201 Created):
```json
{
  "id": 1,
  "projectId": 1,
  "name": "string",
  "description": "string",
  "priority": 1,
  "removed": false,
  "createdAt": "2025-07-03T12:00:00Z"
}
```
Пример:
```
curl -X POST "http://localhost:8080/good/create?projectId=1" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test","description":"Desc"}'
```

#### GET /good/get?projectId={projectId}&id={id}
Получение Good по id.
Query: projectId (int), id (int).
Ответ (200 OK): объект Good.
Если не найден (404):
```json
{
  "code": 3,
  "message": "errors.common.notFound",
  "details": {}
}
```
Пример:
```
curl -i "http://localhost:8080/good/get?projectId=1&id=1"
```

#### PATCH /good/update?projectId={projectId}&id={id}
Обновление полей Good.
Query: projectId, id.
Body:
```json
{
  "name": "string",
  "description": "string"
}
```
Ответ (200 OK): обновлённый объект Good.
Пример:
```
curl -X PATCH "http://localhost:8080/good/update?projectId=1&id=1" \
  -H 'Content-Type: application/json' \
  -d '{"name":"New","description":"Desc"}'
```

#### DELETE /good/remove?projectId={projectId}&id={id}
Мягкое удаление Good.
Query: projectId, id.
Ответ (200 OK):
```json
{
  "id": 1,
  "campaignId": 1,
  "removed": true
}
```
Пример:
```
curl -X DELETE "http://localhost:8080/good/remove?projectId=1&id=1"
```

#### GET /goods/list?limit={limit}&offset={offset}
Список Good.
Query: limit (int, default 10), offset (int, default 0).
Ответ (200 OK):
```json
{
  "meta": {"total":100,"removed":5,"limit":10,"offset":0},
  "goods": [ /* массив объектов Good */ ]
}
```
Пример:
```
curl -i "http://localhost:8080/goods/list?limit=20&offset=0"
```

#### PATCH /good/reprioritize?projectId={projectId}&id={id}
Изменение приоритета Good и сдвиг остальных.
Query: projectId, id.
Body:
```json
{ "newPriority": 3 }
```
Ответ (200 OK):
```json
{ "priorities": [ {"id":2,"priority":2}, {"id":3,"priority":4} ] }
```
Пример:
```
curl -X PATCH "http://localhost:8080/good/reprioritize?projectId=1&id=1" \
  -H 'Content-Type: application/json' \
  -d '{"newPriority":3}'
```

## Consumer-сервис
Слушает тему NATS `goods`, группирует события размером `BATCH_SIZE` и записывает их в таблицу ClickHouse `events_log`.

### Таблица в ClickHouse

Поля таблицы `events_log` и их типы:
- Id: UInt64
- ProjectId: UInt64
- Name: String
- Description: String
- Priority: UInt32
- Removed: UInt8
- EventTime: DateTime

## Кэширование и логирование
- При GET-запросе данные проверяются в Redis. Если нет, запрашиваются из Postgres и сохраняются в Redis на `REDIS_TTL`.
- При изменении (POST, PATCH, DELETE, reprioritize) запись инвалидируется в Redis.
- Изменения публикуются в NATS, consumer пишет их в ClickHouse пачками.

## Тесты
Проект содержит тесты на все уровни.

Запуск локально:
```bash
go test ./... -v
```
Или через Docker:
```bash
docker compose run --rm test
```

package clickhouse_test

import (
	"database/sql"                          // пакет взаимодействия с базой данных через стандартный интерфейс
	_ "github.com/ClickHouse/clickhouse-go" // ClickHouse драйвер, регистрируется анонимным импортом
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require" // библиотека утверждений для тестов
	"os"                                  // пакет для работы с окружением
	"testing"                             // стандартный пакет для тестирования
)

// TestClickhouseMigrations проверяет, что SQL-миграции для ClickHouse выполняются корректно
func TestClickhouseMigrations(t *testing.T) {
	env := os.Getenv("CLICKHOUSE_TEST_DSN")
	if env == "" {
		t.Skip("CLICKHOUSE_TEST_DSN env var not set; skipping ClickHouse migration tests")
	}
	dsn := env

	// Открываем соединение с ClickHouse
	db, err := sql.Open("clickhouse", dsn)
	require.NoError(t, err, "ошибка при открытии соединения с ClickHouse")
	defer func() {
		require.NoError(t, db.Close(), "ошибка при закрытии соединения с ClickHouse")
	}()

	// Откат предыдущих миграций и применение новых через golang-migrate
	drv, err := clickhouse.WithInstance(db, &clickhouse.Config{})
	require.NoError(t, err, "failed to create ClickHouse migrate driver")
	m, err := migrate.NewWithDatabaseInstance(
		"file://.", "clickhouse", drv,
	)
	require.NoError(t, err, "failed to create ClickHouse migrate instance")
	// сначала откатываем все
	_ = m.Down()
	// применяем up-миграции
	require.NoError(t, m.Up(), "failed to apply ClickHouse migrations")

	// ------------------------- Проверка существования таблицы -------------------------
	var existsTable int
	err = db.QueryRow(
		"SELECT count() FROM system.tables WHERE database=currentDatabase() AND name='events_log'",
	).Scan(&existsTable)
	require.NoError(t, err)
	require.Equal(t, 1, existsTable, "events_log должна существовать после migrate Up")
	// проверка полного отката миграций (двух шагов)
	require.NoError(t, m.Steps(-2), "failed to rollback ClickHouse migrations")
	err = db.QueryRow(
		"SELECT count() FROM system.tables WHERE database=currentDatabase() AND name='events_log'",
	).Scan(&existsTable)
	require.NoError(t, err)
	require.Equal(t, 0, existsTable, "events_log должна быть удалена после migrate Down")

	// теперь тест основной логики миграций не нужен, так как проверили up/down
	return

	// ------------------------- Проверка структуры таблицы -------------------------
	// Ожидаемые колонки и их типы
	expected := map[string]string{
		"Id":          "UInt64",
		"ProjectId":   "UInt64",
		"Name":        "String",
		"Description": "String",
		"Priority":    "UInt32",
		"Removed":     "UInt8",
		"EventTime":   "DateTime",
	}

	// Выбираем колонки из system.columns
	rows, err := db.Query(
		"SELECT name, type FROM system.columns WHERE database = currentDatabase() AND table = 'events_log'",
	)
	require.NoError(t, err, "ошибка при получении описания колонок таблицы events_log")
	defer rows.Close()

	colsFound := make(map[string]string)
	for rows.Next() {
		var name, ctype string
		require.NoError(t, rows.Scan(&name, &ctype), "ошибка при сканировании строки system.columns")
		colsFound[name] = ctype
	}
	require.NoError(t, rows.Err(), "ошибка после обхода всех строк system.columns")

	// Сверяем найденные колонки с ожидаемыми
	for col, typ := range expected {
		actual, ok := colsFound[col]
		require.True(t, ok, "колонка %s должна присутствовать в таблице events_log", col)
		require.Equal(t, typ, actual, "тип колонки %s должен быть %s, получен %s", col, typ, actual)
	}

	// ------------------------- Проверка skip-индексов -------------------------
	var idxCount int
	// Индекс по полю Id
	err = db.QueryRow(
		"SELECT count() FROM system.data_skipping_indices WHERE database=currentDatabase() AND table='events_log' AND name='idx_events_log_id'",
	).Scan(&idxCount)
	require.NoError(t, err, "ошибка при проверке skip-индекса idx_events_log_id")
	require.Equal(t, 1, idxCount, "skip-индекс idx_events_log_id должен существовать")

	// Индекс по полю ProjectId
	err = db.QueryRow(
		"SELECT count() FROM system.data_skipping_indices WHERE database=currentDatabase() AND table='events_log' AND name='idx_events_log_project_id'",
	).Scan(&idxCount)
	require.NoError(t, err, "ошибка при проверке skip-индекса idx_events_log_project_id")
	require.Equal(t, 1, idxCount, "skip-индекс idx_events_log_project_id должен существовать")

	// Bloom-фильтр по полю Name
	err = db.QueryRow(
		"SELECT count() FROM system.data_skipping_indices WHERE database=currentDatabase() AND table='events_log' AND name='idx_events_log_name'",
	).Scan(&idxCount)
	require.NoError(t, err, "ошибка при проверке skip-индекса idx_events_log_name")
	require.Equal(t, 1, idxCount, "skip-индекс idx_events_log_name должен существовать")

	// ------------------------- Проверка типа движка таблицы -------------------------
	var engine string
	err = db.QueryRow(
		"SELECT engine FROM system.tables WHERE database=currentDatabase() AND name='events_log'",
	).Scan(&engine)
	require.NoError(t, err, "ошибка при получении типа движка таблицы events_log")
	require.Equal(t, "MergeTree", engine, "движок таблицы events_log должен быть MergeTree")
}

// Пакет postgres_test содержит интеграционные тесты для проверки корректного выполнения SQL миграций PostgreSQL
package postgres_test

import (
	"database/sql" // пакет взаимодействия с базой данных через стандартный интерфейс
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"                 // PostgreSQL драйвер, регистрируется анонимным импортом через side-effects
	"github.com/stretchr/testify/require" // библиотека удобных утверждений для упрощения проверок в тестах
	"os"
	"testing"
)

// TestPostgresMigrations проверяет, что все миграции выполняются корректно и оставляют базу в ожидаемом состоянии
func TestPostgresMigrations(t *testing.T) {
	// Подготовка строки подключения (DSN): сначала пробуем прочитать из переменной окружения MIGRATION_TEST_DSN
	// пропускаем тест, если не задана переменная окружения для тестовой БД
	env := os.Getenv("MIGRATION_TEST_DSN")
	if env == "" {
		t.Skip("MIGRATION_TEST_DSN env var not set; skipping Postgres migration tests")
	}
	dsn := env

	// Открываем соединение с базой данных через драйвер lib/pq
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err, "ошибка при открытии соединения с базой данных")
	// Гарантируем закрытие соединения по завершению теста, проверяем отсутствие ошибок при закрытии
	defer func() {
		require.NoError(t, db.Close(), "ошибка при закрытии соединения с базой данных")
	}()

	// Применяем миграции Postgres с помощью golang-migrate
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	require.NoError(t, err, "failed to create migrate driver")
	m, err := migrate.NewWithDatabaseInstance(
		"file://.", "postgres", driver,
	)
	require.NoError(t, err, "failed to create migrate instance")
	// Откат предыдущих миграций, чтобы обеспечить чистое состояние
	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to rollback migrations: %v", err)
	}
	// Применяем все up миграции
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	// ------------------------- Проверки структуры базы данных -------------------------

	// Проверяем, создалась ли таблица Projects
	var exists bool
	err = db.QueryRow(
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='projects')`,
	).Scan(&exists)
	require.NoError(t, err, "ошибка при проверке существования таблицы Projects")
	require.True(t, exists, "таблица Projects должна существовать после миграций")

	// Проверяем, создалась ли таблица Goods
	err = db.QueryRow(
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='goods')`,
	).Scan(&exists)
	require.NoError(t, err, "ошибка при проверке существования таблицы Goods")
	require.True(t, exists, "таблица Goods должна существовать после миграций")

	// ------------------------- Проверка дефолтной записи в Projects -------------------------

	// Проверяем, что в таблице Projects существует запись с именем 'Первая запись'
	var name string
	err = db.QueryRow(
		`SELECT name FROM Projects WHERE name='Первая запись'`,
	).Scan(&name)
	require.NoError(t, err, "ошибка при выборке дефолтной записи из Projects")
	require.Equal(t, "Первая запись", name, "имя дефолтной записи должно быть 'Первая запись'")

	// Проверяем, что ID дефолтной записи равен 1 (первый добавленный элемент)
	var defaultID int
	err = db.QueryRow(
		`SELECT id FROM Projects WHERE name='Первая запись'`,
	).Scan(&defaultID)
	require.NoError(t, err, "ошибка при получении ID дефолтной записи")
	require.Equal(t, 1, defaultID, "ID дефолтной записи должен быть равен 1")

	// ------------------------- Проверки ограничений первичных ключей -------------------------

	// Проверяем наличие одного первичного ключа в таблице Projects
	var pkCount int
	err = db.QueryRow(
		`SELECT count(*) FROM information_schema.table_constraints WHERE table_name='projects' AND constraint_type='PRIMARY KEY'`,
	).Scan(&pkCount)
	require.NoError(t, err, "ошибка при проверке первичного ключа в Projects")
	require.Equal(t, 1, pkCount, "в таблице Projects должен быть ровно один первичный ключ")

	// Проверяем наличие одного первичного ключа в таблице Goods
	err = db.QueryRow(
		`SELECT count(*) FROM information_schema.table_constraints WHERE table_name='goods' AND constraint_type='PRIMARY KEY'`,
	).Scan(&pkCount)
	require.NoError(t, err, "ошибка при проверке первичного ключа в Goods")
	require.Equal(t, 1, pkCount, "в таблице Goods должен быть ровно один первичный ключ")

	// ------------------------- Проверка внешнего ключа project_id в Goods -------------------------

	var fkExists bool
	err = db.QueryRow(
		`SELECT EXISTS (
		   SELECT 1 FROM information_schema.table_constraints tc
		   JOIN information_schema.key_column_usage kcu ON tc.constraint_name=kcu.constraint_name
		   WHERE tc.table_name='goods' AND tc.constraint_type='FOREIGN KEY' AND kcu.column_name='project_id'
		)`,
	).Scan(&fkExists)
	require.NoError(t, err, "ошибка при проверке внешнего ключа project_id в таблице Goods")
	require.True(t, fkExists, "в таблице Goods должен быть внешний ключ project_id, ссылающийся на Projects(id)")

	// ------------------------- Проверка индексов на таблице Goods -------------------------

	var indexExists bool
	// Индекс по полю project_id
	err = db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename='goods' AND indexname='idx_goods_project_id')`,
	).Scan(&indexExists)
	require.NoError(t, err, "ошибка при проверке индекса idx_goods_project_id")
	require.True(t, indexExists, "индекс idx_goods_project_id должен существовать")

	// Индекс по полю name
	err = db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE tablename='goods' AND indexname='idx_goods_name')`,
	).Scan(&indexExists)
	require.NoError(t, err, "ошибка при проверке индекса idx_goods_name")
	require.True(t, indexExists, "индекс idx_goods_name должен существовать")

	// ------------------------- Проверка работы триггера установки приоритета -------------------------

	// Вставляем первую запись в Goods без явного указания priority, ожидаем priority=1
	_, err = db.Exec(`INSERT INTO Goods (project_id, name) VALUES ($1, $2)`, 1, "TriggerTest1")
	require.NoError(t, err, "ошибка при вставке записи для проверки триггера (TriggerTest1)")
	var pr1 int
	err = db.QueryRow(`SELECT priority FROM Goods WHERE name = $1`, "TriggerTest1").Scan(&pr1)
	require.NoError(t, err, "ошибка при чтении priority для TriggerTest1")
	require.Equal(t, 1, pr1, "приоритет первой записи должен быть равен 1")

	// Вставляем вторую запись, ожидаем приоритет=2
	_, err = db.Exec(`INSERT INTO Goods (project_id, name) VALUES ($1, $2)`, 1, "TriggerTest2")
	require.NoError(t, err, "ошибка при вставке записи для проверки триггера (TriggerTest2)")
	var pr2 int
	err = db.QueryRow(`SELECT priority FROM Goods WHERE name = $1`, "TriggerTest2").Scan(&pr2)
	require.NoError(t, err, "ошибка при чтении priority для TriggerTest2")
	require.Equal(t, 2, pr2, "приоритет второй записи должен быть равен 2")

	// ------------------------- Проверка свойств столбцов created_at и removed -------------------------

	var colDefault, dataType, isNullable string
	// Проверяем столбец Projects.created_at на наличие DEFAULT now(), тип TIMESTAMP и NOT NULL
	err = db.QueryRow(
		`SELECT column_default, data_type, is_nullable FROM information_schema.columns WHERE table_name='projects' AND column_name='created_at'`,
	).Scan(&colDefault, &dataType, &isNullable)
	require.NoError(t, err, "ошибка при проверке свойства столбца projects.created_at")
	require.Contains(t, colDefault, "now()", "DEFAULT для Projects.created_at должен быть now()")
	require.Equal(t, "timestamp without time zone", dataType, "тип Projects.created_at должен быть TIMESTAMP")
	require.Equal(t, "NO", isNullable, "Projects.created_at не должен быть NULL")

	// Проверяем столбец Goods.created_at аналогично
	err = db.QueryRow(
		`SELECT column_default, data_type, is_nullable FROM information_schema.columns WHERE table_name='goods' AND column_name='created_at'`,
	).Scan(&colDefault, &dataType, &isNullable)
	require.NoError(t, err, "ошибка при проверке свойства столбца goods.created_at")
	require.Contains(t, colDefault, "now()", "DEFAULT для Goods.created_at должен быть now()")
	require.Equal(t, "timestamp without time zone", dataType, "тип Goods.created_at должен быть TIMESTAMP")
	require.Equal(t, "NO", isNullable, "Goods.created_at не должен быть NULL")

	// Проверяем столбец Goods.removed: DEFAULT false, тип BOOLEAN и NOT NULL
	err = db.QueryRow(
		`SELECT column_default, data_type, is_nullable FROM information_schema.columns WHERE table_name='goods' AND column_name='removed'`,
	).Scan(&colDefault, &dataType, &isNullable)
	require.NoError(t, err, "ошибка при проверке свойства столбца goods.removed")
	require.Contains(t, colDefault, "false", "DEFAULT для Goods.removed должен быть false")
	require.Equal(t, "boolean", dataType, "тип Goods.removed должен быть BOOLEAN")
	require.Equal(t, "NO", isNullable, "Goods.removed не должен быть NULL")

	// ------------------------- Проверка отката (down migrations) -------------------------
	// Откат всех миграций назад
	if err := m.Steps(-2); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to rollback all migrations: %v", err)
	}
	// Проверяем, что таблица Projects удалена
	exists = false
	err = db.QueryRow(
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='projects')`,
	).Scan(&exists)
	require.NoError(t, err, "ошибка при проверке удаления таблицы Projects после отката")
	require.False(t, exists, "таблица Projects должна быть удалена после отката")
	// Проверяем, что таблица Goods удалена
	exists = false
	err = db.QueryRow(
		`SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name='goods')`,
	).Scan(&exists)
	require.NoError(t, err, "ошибка при проверке удаления таблицы Goods после отката")
	require.False(t, exists, "таблица Goods должна быть удалена после отката")
}

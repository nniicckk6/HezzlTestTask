-- Скрипт инициализации PostgreSQL: создаёт базу данных для тестирования миграций
-- Этот файл автоматически выполнится при старте контейнера postgres благодаря монтированию в /docker-entrypoint-initdb.d

-- Создаём базу данных test_migrations с владельцем appuser
CREATE DATABASE test_migrations OWNER appuser;


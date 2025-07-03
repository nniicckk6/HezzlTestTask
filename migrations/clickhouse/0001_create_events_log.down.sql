-- Миграция 0001 (down): удаление таблицы events_log в ClickHouse
DROP TABLE IF EXISTS events_log;


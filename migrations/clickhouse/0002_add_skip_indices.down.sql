-- Миграция 0002 (down): удаление skip-индексов из таблицы events_log
ALTER TABLE events_log
    DROP INDEX idx_events_log_id,
    DROP INDEX idx_events_log_project_id,
    DROP INDEX idx_events_log_name;

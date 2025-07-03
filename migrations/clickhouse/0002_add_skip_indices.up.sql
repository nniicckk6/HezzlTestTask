-- Миграция 0002 (up): добавление skip-индексов в таблицу events_log
ALTER TABLE events_log
    ADD INDEX IF NOT EXISTS idx_events_log_id (Id) TYPE minmax GRANULARITY 1,
    ADD INDEX IF NOT EXISTS idx_events_log_project_id (ProjectId) TYPE minmax GRANULARITY 1,
    ADD INDEX IF NOT EXISTS idx_events_log_name (Name) TYPE bloom_filter(0.01) GRANULARITY 1;
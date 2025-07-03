-- 0001_create_events_log.sql
-- Миграция: создание таблицы логов событий в ClickHouse
-- Таблица хранит логи изменений объектов Goods для последующей аналитики

CREATE TABLE IF NOT EXISTS events_log (
    `Id` UInt64,            -- уникальный идентификатор лога (устанавливается приложением)
    `ProjectId` UInt64,     -- идентификатор проекта, из которого пришел лог
    `Name` String,          -- название сущности
    `Description` String,   -- описание сущности
    `Priority` UInt32,      -- приоритет сущности из Postgres
    `Removed` UInt8,        -- флаг удаления сущности (0 - не удалена, 1 - удалена)
    `EventTime` DateTime    -- время события (должно устанавливаться приложением)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(EventTime)
ORDER BY (ProjectId, EventTime);

-- Индексы пропусков (skip indices) для ускоренной фильтрации по полям
ALTER TABLE events_log
    ADD INDEX IF NOT EXISTS idx_events_log_id (Id) TYPE minmax() GRANULARITY 1;
ALTER TABLE events_log
    ADD INDEX IF NOT EXISTS idx_events_log_project_id (ProjectId) TYPE minmax() GRANULARITY 1;
ALTER TABLE events_log
    ADD INDEX IF NOT EXISTS idx_events_log_name (Name) TYPE bloom_filter(0.01) GRANULARITY 1;

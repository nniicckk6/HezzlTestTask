-- Миграция 0001 (up): создание таблицы events_log в ClickHouse
-- Таблица хранит логи изменений объектов Goods для аналитики
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
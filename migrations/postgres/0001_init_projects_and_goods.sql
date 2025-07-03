-- DEPRECATED: заменён на 0001_init_projects_and_goods.up.sql/.down.sql

-- Миграция 0001: создание таблиц Projects и Goods с ключами, индексами и триггером для приоритета
-- Подробное описание: создаем структуру БД для хранения проектов и связанных с ними записей (Goods),
-- включая автоматическое назначение приоритета новой записи на основе существующих.

CREATE TABLE IF NOT EXISTS Projects (
    id SERIAL PRIMARY KEY,                       -- уникальный идентификатор проекта, автоинкрементируемый целочисленный тип
    name TEXT NOT NULL,                          -- название проекта, обязательное текстовое поле
    created_at TIMESTAMP NOT NULL DEFAULT now()  -- дата и время создания проекта, по умолчанию текущий момент
);

-- Создание таблицы Goods для хранения записей, связанных с проектами
CREATE TABLE IF NOT EXISTS Goods (
    id SERIAL PRIMARY KEY,            -- уникальный идентификатор записи Goods, автоинкремент
    project_id INT NOT NULL,          -- внешний ключ на таблицу Projects.id, обеспечивает связь записи с проектом
    name TEXT NOT NULL,               -- название записи (товара или задачи), обязательное текстовое поле
    description TEXT,                 -- подробное описание записи, необязательное текстовое поле
    priority INT NOT NULL,            -- числовое значение приоритета записи внутри проекта
    removed BOOLEAN NOT NULL DEFAULT false,  -- логический флаг удаления записи, по умолчанию false
    created_at TIMESTAMP NOT NULL DEFAULT now(),  -- дата и время создания записи, по умолчанию текущий момент
    FOREIGN KEY (project_id) REFERENCES Projects(id)  -- настраиваем внешний ключ, чтобы гарантировать целостность ссылок. -- Если вы хотите, чтобы при удалении проекта автоматически удалялись все связанные записи Goods, можно расширить до FOREIGN KEY (project_id) REFERENCES Projects(id) ON DELETE CASCADE
);

-- Индексы для ускорения поиска по ключевым полям
CREATE INDEX IF NOT EXISTS idx_goods_project_id ON Goods(project_id);  -- индекс по полю project_id для быстрого поиска записей конкретного проекта
CREATE INDEX IF NOT EXISTS idx_goods_name ON Goods(name);              -- индекс по названию записи для быстрого поиска по тексту

-- Функция set_goods_priority: автоматически устанавливает приоритет новой записи как (max(priority)+1)
CREATE OR REPLACE FUNCTION set_goods_priority() RETURNS TRIGGER AS $$
BEGIN
    -- если приоритет не задан (NULL или 0), вычисляем следующий свободный приоритет
    IF NEW.priority IS NULL OR NEW.priority = 0 THEN
        SELECT COALESCE(MAX(priority), 0) + 1 INTO NEW.priority
        FROM Goods
        WHERE project_id = NEW.project_id;  -- учитываем только записи внутри того же проекта
    END IF;
    RETURN NEW;  -- возвращаем модифицированную запись для вставки
END;
$$ LANGUAGE plpgsql;

-- Триггер для вызова функции set_goods_priority перед вставкой в таблицу Goods
CREATE TRIGGER trg_set_goods_priority
BEFORE INSERT ON Goods
FOR EACH ROW EXECUTE FUNCTION set_goods_priority();

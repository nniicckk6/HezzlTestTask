-- Миграция 0001 (down): удаление таблиц и триггера

-- Удаляем триггер и функцию установки приоритета
DROP TRIGGER IF EXISTS trg_set_goods_priority ON Goods;
DROP FUNCTION IF EXISTS set_goods_priority();

-- Удаляем таблицы Goods и Projects
DROP TABLE IF EXISTS Goods;
DROP TABLE IF EXISTS Projects;


-- Миграция 0002 (down): удаление дефолтного проекта 'Первая запись'

DELETE FROM Goods WHERE project_id = (
    SELECT id FROM Projects WHERE name = 'Первая запись'
);
DELETE FROM Projects WHERE name = 'Первая запись';

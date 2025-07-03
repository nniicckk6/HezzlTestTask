-- Инициализация ClickHouse: создание тестовой и рабочей баз, пользователя migrations_user и выдача прав

-- Создание тестовой базы
CREATE DATABASE IF NOT EXISTS migrations_test;

-- Создание основной базы
CREATE DATABASE IF NOT EXISTS appdb;

-- Создание пользователя для миграций
CREATE USER IF NOT EXISTS migrations_user IDENTIFIED WITH plaintext_password BY 'migrator_pass';

-- Выдача прав на базы
GRANT ALL ON migrations_test.* TO migrations_user;
GRANT ALL ON appdb.* TO migrations_user;


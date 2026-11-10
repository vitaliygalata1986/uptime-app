-- +goose Up
ALTER TABLE users ADD COLUMN name VARCHAR(100) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN name;

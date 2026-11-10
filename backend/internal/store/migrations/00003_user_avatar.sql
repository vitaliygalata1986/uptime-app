-- +goose Up
ALTER TABLE users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN avatar_url;

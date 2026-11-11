-- +goose Up
CREATE TABLE monitors (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url TEXT NOT NULL CHECK (length(url) BETWEEN 1 AND 2048),
    interval_seconds INTEGER NOT NULL CHECK (interval_seconds > 0),
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX monitors_user_created_idx ON monitors(user_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE monitors;

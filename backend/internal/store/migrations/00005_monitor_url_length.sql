-- +goose Up
ALTER TABLE monitors
    DROP CONSTRAINT monitors_url_check,
    ADD CONSTRAINT monitors_url_check CHECK (length(url) BETWEEN 1 AND 4096);

-- +goose Down
ALTER TABLE monitors
    DROP CONSTRAINT monitors_url_check,
    ADD CONSTRAINT monitors_url_check CHECK (length(url) BETWEEN 1 AND 2048);

-- +goose Up
CREATE TABLE IF NOT EXISTS metrics (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    gauge_value DOUBLE PRECISION,
    counter_value BIGINT
);

-- +goose Down
DROP TABLE IF EXISTS metrics;

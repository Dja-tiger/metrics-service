-- +goose Up
CREATE TABLE metric_receipts (
    request_id TEXT PRIMARY KEY,
    fingerprint TEXT NOT NULL
);

-- +goose Down
DROP TABLE metric_receipts;

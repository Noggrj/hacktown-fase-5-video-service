-- Idempotency ledger for Kafka consumers (video.processed, video.failed).
-- See internal/platform/idempotency.PostgresStore.
CREATE TABLE IF NOT EXISTS processed_events (
    event_id    UUID NOT NULL,
    consumer    VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, consumer)
);

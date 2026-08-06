-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS payments_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT,
    processed_at TIMESTAMPTZ,
    claimed_by TEXT,
    claim_until TIMESTAMPTZ,

    operation_id VARCHAR(255) NOT NULL UNIQUE,  -- например: deposit:payment:pay-123

    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,

    type VARCHAR(30) NOT NULL CHECK (
        type IN ('deposit', 'refund')
    ),

    amount INT NOT NULL CHECK (amount > 0),

    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'processing', 'processed', 'failed')
    ),


    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- готовое событие для Kafka
    event_key VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_payments_outbox_status_created
ON payments_outbox (status, next_attempt_at,created_at);

-- +goose Down
DROP TABLE IF EXISTS payments_outbox;

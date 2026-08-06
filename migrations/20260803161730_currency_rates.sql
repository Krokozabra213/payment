-- +goose Up
SELECT 'up SQL query';

CREATE TABLE currency_rates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(10) NOT NULL, -- 'USD', 'XTR' (звезды)
    rub_rate    BIGINT NOT NULL CHECK (rub_rate > 0),
    source_name VARCHAR(50) NOT NULL,
    source_at   TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (code, source_name, source_at)
);

-- +goose Down
SELECT 'down SQL query';

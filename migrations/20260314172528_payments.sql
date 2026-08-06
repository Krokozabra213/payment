-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    idempotency_key VARCHAR(127) NOT NULL UNIQUE,

    -- Финансы
    amount INT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,

    -- Статус
    status VARCHAR(20) NOT NULL DEFAULT 'started',

    -- Провайдер
    provider_name VARCHAR(50) NOT NULL,
    provider_payment_id VARCHAR(255),          -- ID платежа у провайдера
    provider_user_id BIGINT,
    payment_url TEXT,

    -- Мета
    description TEXT,

    -- Время
    expires_at TIMESTAMP WITH TIME ZONE,       -- срок жизни ссылки
    paid_at TIMESTAMP WITH TIME ZONE,          -- когда реально оплачен
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Ограничения
    CONSTRAINT valid_status CHECK (
        status IN ('pending', 'completed', 'failed', 'expired', 'refunded', 'cancelled', 'started', 'processed', 'cancelling', 'refunding')
    ),

    CONSTRAINT valid_provider_name CHECK (
        provider_name IN ('tbank_form', 'tg_stars')
    )
);

CREATE UNIQUE INDEX unique_provider_payment
ON payments (provider_name, provider_payment_id)
WHERE provider_payment_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS payments;

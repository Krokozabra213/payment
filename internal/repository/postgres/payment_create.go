package pgRepo

import (
	"context"
	"fmt"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreatePaymentReturnID(ctx context.Context, params domain.CreatePaymentParams) (uuid.UUID, error) {
	ctx, cancel := context.WithTimeout(ctx, ctxTimeout)
	defer cancel()

	query := `
        INSERT INTO payments (
            user_id, idempotency_key, amount, currency, status,
            provider_name, provider_payment_id, payment_url, description, expires_at
        ) VALUES (
            @user_id, @idempotency_key, @amount, @currency, 'pending',
            @provider_name, @provider_payment_id, @payment_url, @description, @expires_at
        )
        RETURNING id`

	args := pgx.NamedArgs{
		"user_id":             params.UserID,
		"idempotency_key":     params.IdempotencyKey,
		"amount":              params.Amount,
		"currency":            params.Currency,
		"provider_name":       params.ProviderName,
		"provider_payment_id": params.ProviderPaymentID,
		"payment_url":         params.PaymentURL,
		"description":         params.Description,
		"expires_at":          params.ExpiresAt,
	}

	var id uuid.UUID
	err := r.QueryRow(ctx, query, args).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("repository.Create: %w", err)
	}
	return id, nil
}

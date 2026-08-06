package pgRepo

import (
	"context"
	"errors"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) PaymentReserve(
	ctx context.Context,
	params domain.CreatePaymentParams,
) (*domain.Payment, bool, error) {
	const insertQuery = `
		INSERT INTO payments (
			user_id,
			idempotency_key,
			amount,
			currency,
			status,
			provider_name,
            provider_payment_id,
            payment_url,
			description,
			expires_at
		)
		VALUES (
			@user_id, @idempotency_key, @amount, @currency, @status,
            @provider_name, @provider_payment_id, @payment_url, @description, @expires_at
		)
        ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING
			id, user_id, idempotency_key, amount, currency, status,
			provider_name, provider_payment_id, payment_url,
			description, provider_user_id, expires_at, paid_at, created_at, updated_at;
	`

	rows, err := r.Query(ctx, insertQuery, pgx.NamedArgs{
		"user_id":             params.UserID,
		"idempotency_key":     params.IdempotencyKey,
		"amount":              params.Amount,
		"currency":            params.Currency,
		"status":              string(domain.PaymentStatusStarted),
		"provider_name":       params.ProviderName,
		"provider_payment_id": params.ProviderPaymentID,
		"payment_url":         params.PaymentURL,
		"description":         params.Description,
		"expires_at":          params.ExpiresAt,
	})
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	rowInsert, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[paymentRow])
	if err == nil {
		payment, err := rowInsert.toDomain()
		if err != nil {
			return nil, true, err
		}
		return payment, true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	const selectQuery = `
		SELECT id, user_id, idempotency_key, amount, currency, status,
			   provider_name, provider_payment_id, provider_user_id, payment_url,
			   description, expires_at, paid_at, created_at, updated_at
		FROM payments
		WHERE idempotency_key = @idempotency_key;
	`

	rowSelect, err := r.Query(ctx, selectQuery, pgx.NamedArgs{
		"provider_name":   params.ProviderName,
		"user_id":         params.UserID,
		"idempotency_key": params.IdempotencyKey,
	})
	if err != nil {
		return nil, false, err
	}
	defer rowSelect.Close()

	pRow, err := pgx.CollectExactlyOneRow(rowSelect, pgx.RowToStructByName[paymentRow])
	if err != nil {
		return nil, false, err
	}

	payment, err := pRow.toDomain()
	return payment, false, err
}

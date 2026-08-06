package pgRepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) PaymentCancelLock(
	ctx context.Context,
	params domain.PaymentCancelLockParams,
) (*domain.Payment, error) {
	tx, err := r.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `
		SELECT
			id, user_id, idempotency_key, amount, currency, status,
			provider_name, provider_payment_id, provider_user_id, payment_url,
			description, expires_at, paid_at, created_at, updated_at
		FROM payments
		WHERE id = @id
		FOR UPDATE;
	`

	rows, err := tx.Query(ctx, selectQuery, pgx.NamedArgs{
		"id": params.PaymentID,
	})
	if err != nil {
		return nil, fmt.Errorf("select for update: %w", err)
	}
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[paymentRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	payment, err := row.toDomain()
	if err != nil {
		return nil, err
	}

	if payment.Status == params.LockStatus {
		if time.Since(payment.UpdatedAt) < 30*time.Second {
			return nil, ErrPaymentProcessed
		}
	} else if payment.Status != params.CurrentStatus {
		return nil, ErrInvalidPaymentState
	}

	const updateQuery = `
		UPDATE payments
		SET status = @status, updated_at = now()
		WHERE id = @id;
	`

	_, err = tx.Exec(ctx, updateQuery, pgx.NamedArgs{
		"id":     payment.ID,
		"status": string(params.LockStatus),
	})
	if err != nil {
		return nil, fmt.Errorf("update lock status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return payment, nil
}

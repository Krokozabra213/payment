package pgRepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/jackc/pgx/v5"
)

const staleProcessingTimeout = 30 * time.Second

func (r *Repository) TGPrecheckoutApprove(
	ctx context.Context,
	params domain.TGPrecheckoutApproveParams,
) error {
	tx, err := r.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `
		SELECT
			id, user_id, idempotency_key, amount, currency, status,
			provider_name, provider_payment_id, provider_user_id, payment_url,
			description, expires_at, paid_at, created_at, updated_at
		FROM payments
		WHERE idempotency_key = @idempotency_key
		FOR UPDATE;
	`

	rows, err := tx.Query(ctx, selectQuery, pgx.NamedArgs{
		"idempotency_key": params.IdempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("select for update: %w", err)
	}
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[paymentRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	payment, err := row.toDomain()
	if err != nil {
		return err
	}

	if err := validateTGPrecheckout(payment, params); err != nil {
		return err
	}

	const updateQuery = `
		UPDATE payments
		SET
			status = @status,
			provider_user_id = @provider_user_id,
			updated_at = now()
		WHERE id = @id;
	`

	cmd, err := tx.Exec(ctx, updateQuery, pgx.NamedArgs{
		"id":               payment.ID,
		"status":           string(domain.PaymentStatusProcessed),
		"provider_user_id": params.TGUserID,
	})
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	if cmd.RowsAffected() != 1 {
		return ErrInvalidPaymentState
	}

	return tx.Commit(ctx)
}

func validateTGPrecheckout(payment *domain.Payment, p domain.TGPrecheckoutApproveParams) error {
	if payment.ProviderName != string(domain.PaymentTGStars) {
		return ErrProviderMismatch
	}

	switch payment.Status {
	case domain.PaymentStatusPending:

	case domain.PaymentStatusProcessed:
		if time.Since(payment.UpdatedAt) < staleProcessingTimeout {
			return ErrPaymentProcessed
		}

	default:
		return ErrInvalidPaymentStatus
	}

	if payment.Amount != p.Amount {
		return ErrAmountMismatch
	}

	if payment.Currency != p.Currency {
		return ErrCurrencyMismatch
	}

	return nil
}

package pgRepo

import (
	"context"
	"errors"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) TbankUpdateStatus(
	ctx context.Context,
	params domain.TbankUpdateStatusParams,
) error {
	const query = `
		UPDATE payments
		SET
			status = @new_status,
			updated_at = now()
		WHERE idempotency_key = @idempotency_key
		  AND provider_name = @provider_name
		  AND status = @current_status
		RETURNING id;
	`

	var id uuid.UUID
	err := r.QueryRow(ctx, query, pgx.NamedArgs{
		"idempotency_key": params.IdempotencyKey,
		"provider_name":   string(domain.PaymentTbankForm),
		"new_status":      string(params.NewStatus),
		"current_status":  string(params.CurrentStatus),
	}).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidPaymentState
		}
		return err
	}

	return nil
}

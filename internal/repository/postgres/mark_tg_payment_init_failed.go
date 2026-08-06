package pgRepo

import (
	"context"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) MarkTGPaymentInitFailed(ctx context.Context, paymentID uuid.UUID) error {
	const query = `
		UPDATE payments
		SET
			status = @status,
			updated_at = now()
		WHERE id = @id
		  AND status = @current_status;
	`

	cmd, err := r.Exec(ctx, query, pgx.NamedArgs{
		"id":             paymentID,
		"status":         string(domain.PaymentStatusFailed),
		"current_status": string(domain.PaymentStatusStarted),
	})
	if err != nil {
		return err
	}

	if cmd.RowsAffected() != 1 {
		return ErrInvalidPaymentState
	}

	return nil
}

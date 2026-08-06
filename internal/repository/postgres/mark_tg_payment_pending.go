package pgRepo

import (
	"context"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) MarkTGPaymentPending(ctx context.Context, paymentID uuid.UUID, paymentURL string) error {
	const query = `
		UPDATE payments
		SET
			status = @status,
			payment_url = @payment_url,
			updated_at = now()
		WHERE id = @id
		  AND status = @current_status;
	`

	cmd, err := r.Exec(ctx, query, pgx.NamedArgs{
		"id":             paymentID,
		"status":         string(domain.PaymentStatusPending),
		"payment_url":    paymentURL,
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

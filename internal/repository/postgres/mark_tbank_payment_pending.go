package pgRepo

import (
	"context"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) MarkTbankPaymentPending(
	ctx context.Context,
	paymentID uuid.UUID,
	paymentURL string,
	providerPaymentID string,
) error {
	const query = `
		UPDATE payments
		SET
			status = @status,
			payment_url = @payment_url,
			provider_payment_id = @provider_payment_id,
			updated_at = now()
		WHERE id = @id
		  AND status = @current_status;
	`

	cmd, err := r.Exec(ctx, query, pgx.NamedArgs{
		"id":                  paymentID,
		"status":              string(domain.PaymentStatusPending),
		"payment_url":         paymentURL,
		"provider_payment_id": providerPaymentID,
		"current_status":      string(domain.PaymentStatusStarted),
	})
	if err != nil {
		return err
	}

	if cmd.RowsAffected() != 1 {
		return ErrInvalidPaymentState
	}

	return nil
}

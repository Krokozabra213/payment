package pgRepo

import (
	"context"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) PaymentRevertLock(
	ctx context.Context,
	paymentID uuid.UUID,
	lockStatus domain.PaymentStatus,
	revertStatus domain.PaymentStatus,
) error {
	const query = `
		UPDATE payments
		SET status = @revert_status, updated_at = now()
		WHERE id = @id AND status = @lock_status;
	`

	cmd, err := r.Exec(ctx, query, pgx.NamedArgs{
		"id":            paymentID,
		"lock_status":   string(lockStatus),
		"revert_status": string(revertStatus),
	})
	if err != nil {
		return err
	}

	if cmd.RowsAffected() != 1 {
		return ErrInvalidPaymentState
	}

	return nil
}

package pgRepo

import (
	"context"
	"errors"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetPaymentByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	const query = `
		SELECT
			id, user_id, idempotency_key, amount, currency, status,
			provider_name, provider_payment_id, provider_user_id, payment_url,
			description, expires_at, paid_at, created_at, updated_at
		FROM payments
		WHERE id = @id;
	`

	rows, err := r.Query(ctx, query, pgx.NamedArgs{
		"id": id,
	})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[paymentRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return row.toDomain()
}

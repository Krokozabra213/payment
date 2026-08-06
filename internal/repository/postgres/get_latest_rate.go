package pgRepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetLatestRate(ctx context.Context, code domain.CurrencyCode) (int64, error) {
	const query = `
		SELECT rub_rate
		FROM currency_rates
		WHERE code = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var rate int64
	err := r.QueryRow(ctx, query, string(code)).Scan(&rate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("%w: %s", ErrNotFound, code)
		}
		return 0, fmt.Errorf("get latest rate for %s: %w", code, err)
	}

	return rate, nil
}

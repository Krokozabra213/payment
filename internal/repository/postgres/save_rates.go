package pgRepo

import (
	"context"
	"fmt"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) SaveRates(ctx context.Context, rates []domain.CurrencyRate) error {
	batch := &pgx.Batch{}

	for _, rate := range rates {
		batch.Queue(`
			INSERT INTO currency_rates (code, rub_rate, source_name, source_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code, source_name, source_at) DO NOTHING
		`, string(rate.Code), rate.RubRate, rate.SourceName, rate.SourceAt)
	}

	br := r.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(rates); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("batch exec: %w", err)
		}
	}

	return nil
}

package pgRepo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) ClaimBatch(ctx context.Context, batchSize int, workerID string, lease time.Duration) ([]PaymentsOutbox, error) {
	claimUntil := time.Now().Add(lease)

	const query = `
		WITH picked AS (
			SELECT id
			FROM payments_outbox
			WHERE
				(status = 'pending' AND next_attempt_at <= NOW())
				OR
				(status = 'processing' AND claim_until IS NOT NULL AND claim_until < NOW())
			ORDER BY created_at
			LIMIT @batch_size
			FOR UPDATE SKIP LOCKED
		)
		UPDATE payments_outbox o
		SET status = 'processing',
            claimed_by = @claimed_by,
			claim_until = @claim_until,
			updated_at = NOW()
		FROM picked
		WHERE o.id = picked.id
		RETURNING o.*;
	`

	rows, err := r.Query(ctx, query, pgx.NamedArgs{
		"batch_size":  batchSize,
		"claim_until": claimUntil,
		"claimed_by":  workerID,
	})
	if err != nil {
		return nil, fmt.Errorf("claim batch query: %w", err)
	}
	defer rows.Close()

	result, err := pgx.CollectRows(rows, pgx.RowToStructByName[PaymentsOutbox])
	if err != nil {
		return nil, fmt.Errorf("collect claimed rows: %w", err)
	}

	return result, nil
}

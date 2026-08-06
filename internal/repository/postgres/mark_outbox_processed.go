package pgRepo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) MarkOutboxProcessed(ctx context.Context, row *PaymentsOutbox, workerID string) error {
	const query = `
		UPDATE payments_outbox
		SET status = 'processed',
			processed_at = NOW(),
			last_error = NULL,
			claimed_by = NULL,
			claim_until = NULL,
			updated_at = NOW()
		WHERE id = @id
		  AND status = 'processing'
		  AND claimed_by = @worker_id
	`

	tag, err := r.Exec(ctx, query, pgx.NamedArgs{
		"id":        row.ID,
		"worker_id": workerID,
	})
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark processed: row not updated (claim lost or already changed)")
	}

	return nil
}

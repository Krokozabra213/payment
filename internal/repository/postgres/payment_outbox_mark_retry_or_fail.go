package pgRepo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) MarkRetryOrFail(ctx context.Context, row *PaymentsOutbox, cause error, maxAttempts int, workerID string) error {
	nextAttempts := row.Attempts + 1
	status := "pending"
	if nextAttempts >= maxAttempts {
		status = "failed"
	}

	nextAttemptAt := time.Now().Add(retryBackoff(nextAttempts))

	const query = `
		UPDATE payments_outbox
		SET status = @status,
			attempts = attempts + 1,
			next_attempt_at = @next_attempt_at,
			last_error = @last_error,
			claimed_by = NULL,
			claim_until = NULL,
			updated_at = NOW()
		WHERE id = @id
		  AND status = 'processing'
		  AND claimed_by = @worker_id
	`

	tag, err := r.Exec(ctx, query, pgx.NamedArgs{
		"id":              row.ID,
		"worker_id":       workerID,
		"status":          status,
		"next_attempt_at": nextAttemptAt,
		"last_error":      truncate(cause.Error(), 2000),
	})
	if err != nil {
		return fmt.Errorf("mark retry/fail: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark retry/fail: row not updated (claim lost or already changed)")
	}

	return nil
}

func retryBackoff(attempt int) time.Duration {
	// 1s, 2s, 4s, 8s, 16s, 32s, max 1m
	sec := 1 << min(attempt-1, 5)
	d := time.Duration(sec) * time.Second
	if d > time.Minute {
		return time.Minute
	}
	return d
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

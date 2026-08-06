//go:build integration

package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	testutil "github.com/Krokozabra213/gargantua_common/pkg/testutils"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedClaimedOutboxRow(t *testing.T, testRepo *pgRepo.Repository, workerID string, attempts int) pgRepo.PaymentsOutbox {
	t.Helper()

	payment := seedPendingPayment(t, testRepo)

	id := uuid.New()
	operationID := "op:" + id.String()
	claimUntil := time.Now().Add(5 * time.Minute)

	_, err := testRepo.Exec(context.Background(), `
		INSERT INTO payments_outbox (
			id, operation_id, payment_id, type, amount, event_key,
			payload, status, attempts, next_attempt_at,
			claimed_by, claim_until, created_at, updated_at
		) VALUES (
			@id, @operation_id, @payment_id, @type, @amount, @event_key,
			@payload, 'processing', @attempts, NOW(),
			@claimed_by, @claim_until, NOW(), NOW()
		)
	`, pgx.NamedArgs{
		"id":           id,
		"operation_id": operationID,
		"payment_id":   payment.ID,
		"type":         "deposit",
		"amount":       payment.Amount,
		"event_key":    "test_key",
		"payload":      []byte(`{}`),
		"attempts":     attempts,
		"claimed_by":   workerID,
		"claim_until":  claimUntil,
	})
	require.NoError(t, err)

	return pgRepo.PaymentsOutbox{
		ID:          id,
		OperationID: operationID,
		PaymentID:   payment.ID,
		Type:        "deposit",
		Amount:      payment.Amount,
		EventKey:    "test_key",
		Payload:     []byte(`{}`),
		Status:      "processing",
		Attempts:    attempts,
		ClaimedBy:   strPtr(workerID),
		ClaimUntil:  timePtr(claimUntil),
	}
}

func getOutboxRow(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) pgRepo.PaymentsOutbox {
	t.Helper()

	rows, err := pool.Query(context.Background(), `
		SELECT
			id, operation_id, payment_id, type, amount, event_key,
			payload, status, attempts, next_attempt_at,
			last_error, claimed_by, claim_until, processed_at,
			created_at, updated_at
		FROM payments_outbox
		WHERE id = @id
	`, pgx.NamedArgs{"id": id})
	require.NoError(t, err)
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[pgRepo.PaymentsOutbox])
	require.NoError(t, err)

	return row
}

func TestMarkRetryOrFail_Retry(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	maxAttempts := 5
	row := seedClaimedOutboxRow(t, repo, workerID, 1)

	beforeMark := time.Now()

	err := repo.MarkRetryOrFail(context.Background(), &row, fmt.Errorf("connection timeout"), maxAttempts, workerID)
	require.NoError(t, err)

	updated := getOutboxRow(t, pool, row.ID)

	assert.Equal(t, "pending", updated.Status)
	assert.Equal(t, 2, updated.Attempts)
	assert.NotNil(t, updated.LastError)
	assert.Equal(t, "connection timeout", *updated.LastError)
	assert.Nil(t, updated.ClaimedBy)
	assert.Nil(t, updated.ClaimUntil)
	assert.True(t, updated.NextAttemptAt.After(beforeMark))
	assert.True(t, updated.UpdatedAt.After(beforeMark) || updated.UpdatedAt.Equal(beforeMark))
}

func TestMarkRetryOrFail_FailOnMaxAttempts(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	maxAttempts := 3
	row := seedClaimedOutboxRow(t, repo, workerID, 2)

	err := repo.MarkRetryOrFail(context.Background(), &row, fmt.Errorf("provider unavailable"), maxAttempts, workerID)
	require.NoError(t, err)

	updated := getOutboxRow(t, pool, row.ID)

	assert.Equal(t, "failed", updated.Status)
	assert.Equal(t, 3, updated.Attempts)
	assert.NotNil(t, updated.LastError)
	assert.Equal(t, "provider unavailable", *updated.LastError)
	assert.Nil(t, updated.ClaimedBy)
	assert.Nil(t, updated.ClaimUntil)
}

func TestMarkRetryOrFail_FailOnExceedMaxAttempts(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	maxAttempts := 3
	row := seedClaimedOutboxRow(t, repo, workerID, 5)

	err := repo.MarkRetryOrFail(context.Background(), &row, fmt.Errorf("still broken"), maxAttempts, workerID)
	require.NoError(t, err)

	updated := getOutboxRow(t, pool, row.ID)

	assert.Equal(t, "failed", updated.Status)
	assert.Equal(t, 6, updated.Attempts)
}

func TestMarkRetryOrFail_ClaimLost_DifferentWorker(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	row := seedClaimedOutboxRow(t, repo, "worker-A", 0)

	err := repo.MarkRetryOrFail(context.Background(), &row, fmt.Errorf("some error"), 5, "worker-B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row not updated")

	unchanged := getOutboxRow(t, pool, row.ID)
	assert.Equal(t, "processing", unchanged.Status)
	assert.Equal(t, 0, unchanged.Attempts)
	assert.NotNil(t, unchanged.ClaimedBy)
	assert.Equal(t, "worker-A", *unchanged.ClaimedBy)
}

func TestMarkRetryOrFail_StatusAlreadyChanged(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	row := seedClaimedOutboxRow(t, repo, workerID, 0)

	_, err := pool.Exec(context.Background(), `
		UPDATE payments_outbox SET status = 'processed' WHERE id = @id
	`, pgx.NamedArgs{"id": row.ID})
	require.NoError(t, err)

	err = repo.MarkRetryOrFail(context.Background(), &row, fmt.Errorf("some error"), 5, workerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row not updated")

	unchanged := getOutboxRow(t, pool, row.ID)
	assert.Equal(t, "processed", unchanged.Status)
}

func TestMarkRetryOrFail_RowNotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	nonExistent := pgRepo.PaymentsOutbox{
		ID:       uuid.New(),
		Attempts: 0,
	}

	err := repo.MarkRetryOrFail(context.Background(), &nonExistent, fmt.Errorf("some error"), 5, "worker-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row not updated")
}

func TestMarkRetryOrFail_LongErrorTruncated(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	row := seedClaimedOutboxRow(t, repo, workerID, 0)

	longMsg := strings.Repeat("x", 3000)

	err := repo.MarkRetryOrFail(context.Background(), &row, fmt.Errorf("%s", longMsg), 5, workerID)
	require.NoError(t, err)

	updated := getOutboxRow(t, pool, row.ID)
	require.NotNil(t, updated.LastError)
	assert.Len(t, *updated.LastError, 2000)
}

func TestMarkRetryOrFail_RetryBackoffIncreases(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"

	row1 := seedClaimedOutboxRow(t, repo, workerID, 0)
	err := repo.MarkRetryOrFail(context.Background(), &row1, fmt.Errorf("err"), 10, workerID)
	require.NoError(t, err)
	updated1 := getOutboxRow(t, pool, row1.ID)

	row2 := seedClaimedOutboxRow(t, repo, workerID, 3)
	err = repo.MarkRetryOrFail(context.Background(), &row2, fmt.Errorf("err"), 10, workerID)
	require.NoError(t, err)
	updated2 := getOutboxRow(t, pool, row2.ID)

	delay1 := updated1.NextAttemptAt.Sub(time.Now())
	delay2 := updated2.NextAttemptAt.Sub(time.Now())

	assert.True(t, delay2 > delay1, "backoff at attempt 4 (%v) should be greater than at attempt 1 (%v)", delay2, delay1)
}

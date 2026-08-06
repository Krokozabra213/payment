//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	testutil "github.com/Krokozabra213/gargantua_common/pkg/testutils"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkOutboxProcessed_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	row := seedClaimedOutboxRow(t, repo, workerID, 0)

	beforeMark := time.Now()

	err := repo.MarkOutboxProcessed(context.Background(), &row, workerID)
	require.NoError(t, err)

	updated := getOutboxRow(t, pool, row.ID)

	assert.Equal(t, "processed", updated.Status)
	assert.NotNil(t, updated.ProcessedAt)
	assert.True(t, updated.ProcessedAt.After(beforeMark) || updated.ProcessedAt.Equal(beforeMark))
	assert.Nil(t, updated.LastError)
	assert.Nil(t, updated.ClaimedBy)
	assert.Nil(t, updated.ClaimUntil)
	assert.True(t, updated.UpdatedAt.After(beforeMark) || updated.UpdatedAt.Equal(beforeMark))
}

func TestMarkOutboxProcessed_ClearsLastError(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	row := seedClaimedOutboxRow(t, repo, workerID, 2)

	_, err := pool.Exec(context.Background(), `
		UPDATE payments_outbox
		SET last_error = @last_error
		WHERE id = @id
	`, pgx.NamedArgs{
		"id":         row.ID,
		"last_error": "previous connection timeout",
	})
	require.NoError(t, err)

	before := getOutboxRow(t, pool, row.ID)
	require.NotNil(t, before.LastError)
	assert.Equal(t, "previous connection timeout", *before.LastError)

	err = repo.MarkOutboxProcessed(context.Background(), &row, workerID)
	require.NoError(t, err)

	updated := getOutboxRow(t, pool, row.ID)

	assert.Equal(t, "processed", updated.Status)
	assert.Nil(t, updated.LastError)
}

func TestMarkOutboxProcessed_ClaimLost_DifferentWorker(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	row := seedClaimedOutboxRow(t, repo, "worker-A", 0)

	err := repo.MarkOutboxProcessed(context.Background(), &row, "worker-B")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row not updated")

	unchanged := getOutboxRow(t, pool, row.ID)
	assert.Equal(t, "processing", unchanged.Status)
	assert.Nil(t, unchanged.ProcessedAt)
	assert.NotNil(t, unchanged.ClaimedBy)
	assert.Equal(t, "worker-A", *unchanged.ClaimedBy)
}

func TestMarkOutboxProcessed_StatusAlreadyChanged(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	row := seedClaimedOutboxRow(t, repo, workerID, 0)

	_, err := pool.Exec(context.Background(), `
		UPDATE payments_outbox
		SET status = 'failed',
		    claimed_by = NULL,
		    claim_until = NULL
		WHERE id = @id
	`, pgx.NamedArgs{"id": row.ID})
	require.NoError(t, err)

	err = repo.MarkOutboxProcessed(context.Background(), &row, workerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row not updated")

	unchanged := getOutboxRow(t, pool, row.ID)
	assert.Equal(t, "failed", unchanged.Status)
	assert.Nil(t, unchanged.ProcessedAt)
}

func TestMarkOutboxProcessed_RowNotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	nonExistent := pgRepo.PaymentsOutbox{
		ID: uuid.New(),
	}

	err := repo.MarkOutboxProcessed(context.Background(), &nonExistent, "worker-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row not updated")
}

func TestMarkOutboxProcessed_DoubleCall_Fails(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	workerID := "worker-1"
	row := seedClaimedOutboxRow(t, repo, workerID, 0)

	err := repo.MarkOutboxProcessed(context.Background(), &row, workerID)
	require.NoError(t, err)

	updated := getOutboxRow(t, pool, row.ID)
	assert.Equal(t, "processed", updated.Status)

	err = repo.MarkOutboxProcessed(context.Background(), &row, workerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row not updated")

	stillSame := getOutboxRow(t, pool, row.ID)
	assert.Equal(t, "processed", stillSame.Status)
	assert.Equal(t, updated.ProcessedAt, stillSame.ProcessedAt)
}

//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	testutil "github.com/Krokozabra213/gargantua_common/pkg/testutils"
	"github.com/GargantuaLabs/payment/migrations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
)

type seedOutboxParams struct {
	Status        string
	NextAttemptAt time.Time
	ClaimedBy     *string
	ClaimUntil    *time.Time
	Attempts      int
}

func seedOutboxRow(t *testing.T, testRepo *pgRepo.Repository, p seedOutboxParams) uuid.UUID {
	t.Helper()

	payment := seedPendingPayment(t, testRepo)

	id := uuid.New()
	operationID := "op:" + id.String()

	_, err := testRepo.Exec(context.Background(), `
		INSERT INTO payments_outbox (
			id, operation_id, payment_id, type, amount, event_key,
			payload, status, attempts, next_attempt_at,
			claimed_by, claim_until, created_at, updated_at
		) VALUES (
			@id, @operation_id, @payment_id, @type, @amount, @event_key,
			@payload, @status, @attempts, @next_attempt_at,
			@claimed_by, @claim_until, NOW(), NOW()
		)
	`, pgx.NamedArgs{
		"id":              id,
		"operation_id":    operationID,
		"payment_id":      payment.ID,
		"type":            "deposit",
		"amount":          payment.Amount,
		"event_key":       "test_key",
		"payload":         []byte(`{}`),
		"status":          p.Status,
		"attempts":        p.Attempts,
		"next_attempt_at": p.NextAttemptAt,
		"claimed_by":      p.ClaimedBy,
		"claim_until":     p.ClaimUntil,
	})
	require.NoError(t, err)

	return id
}

func strPtr(s string) *string        { return &s }
func timePtr(t time.Time) *time.Time { return &t }

func TestClaimBatch_EmptyTable(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	result, err := repo.ClaimBatch(context.Background(), 10, "worker-1", 5*time.Minute)
	require.NoError(t, err)

	assert.Empty(t, result)
}

func TestClaimBatch_PicksPendingReady(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	readyID := seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "pending",
		NextAttemptAt: time.Now().Add(-1 * time.Minute),
	})

	result, err := repo.ClaimBatch(context.Background(), 10, "worker-1", 5*time.Minute)
	require.NoError(t, err)

	require.Len(t, result, 1)
	assert.Equal(t, readyID, result[0].ID)
	assert.Equal(t, "processing", result[0].Status)
	assert.NotNil(t, result[0].ClaimedBy)
	assert.Equal(t, "worker-1", *result[0].ClaimedBy)
	assert.NotNil(t, result[0].ClaimUntil)
	assert.True(t, result[0].ClaimUntil.After(time.Now()))
}

func TestClaimBatch_SkipsPendingNotReady(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "pending",
		NextAttemptAt: time.Now().Add(10 * time.Minute),
	})

	result, err := repo.ClaimBatch(context.Background(), 10, "worker-1", 5*time.Minute)
	require.NoError(t, err)

	assert.Empty(t, result)
}

func TestClaimBatch_PicksExpiredProcessing(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	expiredClaimUntil := time.Now().Add(-1 * time.Minute)

	stuckID := seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "processing",
		NextAttemptAt: time.Now().Add(-5 * time.Minute),
		ClaimedBy:     strPtr("dead-worker"),
		ClaimUntil:    timePtr(expiredClaimUntil),
	})

	result, err := repo.ClaimBatch(context.Background(), 10, "worker-2", 5*time.Minute)
	require.NoError(t, err)

	require.Len(t, result, 1)
	assert.Equal(t, stuckID, result[0].ID)
	assert.Equal(t, "processing", result[0].Status)
	require.NotNil(t, result[0].ClaimedBy)
	assert.Equal(t, "worker-2", *result[0].ClaimedBy)
}

func TestClaimBatch_SkipsActiveProcessing(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "processing",
		NextAttemptAt: time.Now().Add(-5 * time.Minute),
		ClaimedBy:     strPtr("active-worker"),
		ClaimUntil:    timePtr(time.Now().Add(5 * time.Minute)),
	})

	result, err := repo.ClaimBatch(context.Background(), 10, "worker-2", 5*time.Minute)
	require.NoError(t, err)

	assert.Empty(t, result)
}

func TestClaimBatch_RespectsLimit(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	for i := 0; i < 5; i++ {
		seedOutboxRow(t, repo, seedOutboxParams{
			Status:        "pending",
			NextAttemptAt: time.Now().Add(-1 * time.Minute),
		})
	}

	result, err := repo.ClaimBatch(context.Background(), 3, "worker-1", 5*time.Minute)
	require.NoError(t, err)

	assert.Len(t, result, 3)

	var pendingCount int
	err = pool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM payments_outbox WHERE status = 'pending'`,
	).Scan(&pendingCount)
	require.NoError(t, err)

	assert.Equal(t, 2, pendingCount)
}

func TestClaimBatch_SkipsTerminalStatuses(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	for _, status := range []string{"processed", "failed"} {
		seedOutboxRow(t, repo, seedOutboxParams{
			Status:        status,
			NextAttemptAt: time.Now().Add(-1 * time.Minute),
		})
	}

	result, err := repo.ClaimBatch(context.Background(), 10, "worker-1", 5*time.Minute)
	require.NoError(t, err)

	assert.Empty(t, result)
}

func TestClaimBatch_FieldsUpdatedCorrectly(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	beforeClaim := time.Now()

	id := seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "pending",
		NextAttemptAt: time.Now().Add(-1 * time.Minute),
	})

	lease := 7 * time.Minute
	result, err := repo.ClaimBatch(context.Background(), 10, "worker-42", lease)
	require.NoError(t, err)

	require.Len(t, result, 1)
	row := result[0]

	assert.Equal(t, id, row.ID)
	assert.Equal(t, "processing", row.Status)

	require.NotNil(t, row.ClaimedBy)
	assert.Equal(t, "worker-42", *row.ClaimedBy)

	require.NotNil(t, row.ClaimUntil)
	assert.WithinDuration(t, beforeClaim.Add(lease), *row.ClaimUntil, 5*time.Second)

	assert.True(t, row.UpdatedAt.After(beforeClaim) || row.UpdatedAt.Equal(beforeClaim))
}

func TestClaimBatch_ConcurrentWorkersNoOverlap(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	ids := make(map[uuid.UUID]bool)
	for i := 0; i < 4; i++ {
		id := seedOutboxRow(t, repo, seedOutboxParams{
			Status:        "pending",
			NextAttemptAt: time.Now().Add(-1 * time.Minute),
		})
		ids[id] = true
	}

	result1, err := repo.ClaimBatch(context.Background(), 3, "worker-A", 5*time.Minute)
	require.NoError(t, err)

	result2, err := repo.ClaimBatch(context.Background(), 3, "worker-B", 5*time.Minute)
	require.NoError(t, err)

	assert.Len(t, result1, 3)
	assert.Len(t, result2, 1)

	claimedIDs := make(map[uuid.UUID]string)
	for _, r := range result1 {
		claimedIDs[r.ID] = *r.ClaimedBy
	}
	for _, r := range result2 {
		_, exists := claimedIDs[r.ID]
		assert.False(t, exists, "worker-B захватил запись, уже захваченную worker-A: %s", r.ID)
		claimedIDs[r.ID] = *r.ClaimedBy
	}

	assert.Len(t, claimedIDs, 4)
}

func TestClaimBatch_MixedStatuses(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	repo := pgRepo.New(pool)

	pendingReadyID := seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "pending",
		NextAttemptAt: time.Now().Add(-1 * time.Minute),
	})

	seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "pending",
		NextAttemptAt: time.Now().Add(10 * time.Minute),
	})

	expiredID := seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "processing",
		NextAttemptAt: time.Now().Add(-5 * time.Minute),
		ClaimedBy:     strPtr("dead-worker"),
		ClaimUntil:    timePtr(time.Now().Add(-1 * time.Minute)),
	})

	seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "processing",
		NextAttemptAt: time.Now().Add(-5 * time.Minute),
		ClaimedBy:     strPtr("alive-worker"),
		ClaimUntil:    timePtr(time.Now().Add(5 * time.Minute)),
	})

	seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "processed",
		NextAttemptAt: time.Now().Add(-1 * time.Minute),
	})

	seedOutboxRow(t, repo, seedOutboxParams{
		Status:        "failed",
		NextAttemptAt: time.Now().Add(-1 * time.Minute),
	})

	result, err := repo.ClaimBatch(context.Background(), 10, "worker-1", 5*time.Minute)
	require.NoError(t, err)

	require.Len(t, result, 2)

	claimedIDs := make(map[uuid.UUID]bool)
	for _, r := range result {
		claimedIDs[r.ID] = true
	}

	assert.True(t, claimedIDs[pendingReadyID])
	assert.True(t, claimedIDs[expiredID])
}

//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	testutil "github.com/Krokozabra213/gargantua_common/pkg/testutils"
	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedReservedPayment(t *testing.T, testRepo *pgRepo.Repository, overrides ...func(*domain.CreatePaymentParams)) *domain.Payment {
	t.Helper()
	params := createReservePayment(t, overrides...)
	payment, inserted, err := testRepo.PaymentReserve(context.Background(), params)
	require.NoError(t, err)
	require.True(t, inserted)
	return payment
}

func seedPendingPayment(t *testing.T, testRepo *pgRepo.Repository, overrides ...func(*domain.CreatePaymentParams)) *domain.Payment {
	t.Helper()
	payment := seedReservedPayment(t, testRepo, overrides...)
	err := testRepo.MarkTGPaymentPending(context.Background(), payment.ID, "https://t.me/invoice/test")
	require.NoError(t, err)
	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	return updated
}

func seedProcessingPayment(t *testing.T, testRepo *pgRepo.Repository, overrides ...func(*domain.CreatePaymentParams)) *domain.Payment {
	t.Helper()
	payment := seedPendingPayment(t, testRepo, overrides...)
	err := testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
	})
	require.NoError(t, err)
	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	return updated
}

func TestTGPrecheckoutApprove_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedPendingPayment(t, testRepo)

	err := testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusProcessed, updated.Status)
	assert.Equal(t, int64(123456), updated.ProviderUserID)
	assert.True(t, updated.UpdatedAt.After(payment.UpdatedAt) || updated.UpdatedAt.Equal(payment.UpdatedAt))
}

func TestTGPrecheckoutApprove_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: "nonexistent-key",
		TGUserID:       123456,
		Amount:         100,
		Currency:       "XTR",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrNotFound)
}

func TestTGPrecheckoutApprove_WrongAmount(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedPendingPayment(t, testRepo)

	err := testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         999999,
		Currency:       payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "amount mismatch")

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
}

func TestTGPrecheckoutApprove_WrongCurrency(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedPendingPayment(t, testRepo)

	err := testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         payment.Amount,
		Currency:       "RUB",
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "currency mismatch")

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
}

func TestTGPrecheckoutApprove_WrongStatus_Started(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo)

	err := testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid payment status")
}

func TestTGPrecheckoutApprove_WrongStatus_Failed(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo)
	err := testRepo.MarkTGPaymentInitFailed(context.Background(), payment.ID)
	require.NoError(t, err)

	err = testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid payment status")
}

func TestTGPrecheckoutApprove_Processing_Fresh_Rejects(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedProcessingPayment(t, testRepo)

	err := testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "payment is processing")
}

func TestTGPrecheckoutApprove_Processing_Stale_Recovers(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedProcessingPayment(t, testRepo)

	_, err := pool.Exec(context.Background(),
		`UPDATE payments SET updated_at = now() - interval '60 seconds' WHERE id = $1`,
		payment.ID,
	)
	require.NoError(t, err)

	err = testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       789012,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusProcessed, updated.Status)
	assert.Equal(t, int64(789012), updated.ProviderUserID)
	assert.True(t, time.Since(updated.UpdatedAt) < 5*time.Second)
}

func TestTGPrecheckoutApprove_Concurrent_OnlyOneWins(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedPendingPayment(t, testRepo)

	params := domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
	}

	type result struct {
		err error
	}

	start := make(chan struct{})
	results := make(chan result, 2)

	for range 2 {
		go func() {
			<-start
			err := testRepo.TGPrecheckoutApprove(context.Background(), params)
			results <- result{err: err}
		}()
	}

	close(start)

	r1 := <-results
	r2 := <-results

	successCount := 0
	failCount := 0

	for _, r := range []result{r1, r2} {
		if r.err == nil {
			successCount++
		} else {
			failCount++
			assert.ErrorContains(t, r.err, "payment is processing")
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, failCount)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusProcessed, updated.Status)
}

func TestTGPrecheckoutApprove_StatusNotChanged_OnValidationError(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedPendingPayment(t, testRepo)

	err := testRepo.TGPrecheckoutApprove(context.Background(), domain.TGPrecheckoutApproveParams{
		IdempotencyKey: payment.IdempotencyKey,
		TGUserID:       123456,
		Amount:         999999,
		Currency:       "WRONG",
	})
	require.Error(t, err)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
	assert.Equal(t, payment.UpdatedAt, unchanged.UpdatedAt)
}

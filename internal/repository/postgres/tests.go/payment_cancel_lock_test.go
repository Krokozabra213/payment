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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentCancelLock_Success_FromPending(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	locked, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)
	require.NotNil(t, locked)

	assert.Equal(t, payment.ID, locked.ID)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusCancelling, updated.Status)
}

func TestPaymentCancelLock_Success_FromPaid(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPaidPayment(t, testRepo)

	locked, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusCompleted,
		LockStatus:    domain.PaymentStatusRefunding,
	})
	require.NoError(t, err)
	require.NotNil(t, locked)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusRefunding, updated.Status)
}

func TestPaymentCancelLock_Success_FromStarted(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
		p.Currency = "RUB"
	})

	locked, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusStarted,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)
	require.NotNil(t, locked)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusCancelling, updated.Status)
}

func TestPaymentCancelLock_WrongStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
		p.Currency = "RUB"
	})

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusStarted, unchanged.Status)
}

func TestPaymentCancelLock_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     uuid.New(),
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrNotFound)
}

func TestPaymentCancelLock_AlreadyLocked_Fresh_Rejects(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	_, err = testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrPaymentProcessed)
}

func TestPaymentCancelLock_AlreadyLocked_Stale_Recovers(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	_, err = pool.Exec(context.Background(),
		`UPDATE payments SET updated_at = now() - interval '60 seconds' WHERE id = $1`,
		payment.ID,
	)
	require.NoError(t, err)

	locked, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)
	require.NotNil(t, locked)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusCancelling, updated.Status)
	assert.True(t, time.Since(updated.UpdatedAt) < 5*time.Second)
}

func TestPaymentCancelLock_Concurrent_OnlyOneWins(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	params := domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	}

	type result struct {
		payment *domain.Payment
		err     error
	}

	start := make(chan struct{})
	results := make(chan result, 2)

	for range 2 {
		go func() {
			<-start
			p, err := testRepo.PaymentCancelLock(context.Background(), params)
			results <- result{payment: p, err: err}
		}()
	}

	close(start)

	r1 := <-results
	r2 := <-results

	successCount := 0
	rejectCount := 0

	for _, r := range []result{r1, r2} {
		if r.err == nil {
			successCount++
		} else {
			rejectCount++
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, rejectCount)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusCancelling, updated.Status)
}

//go:build integration

package tests

import (
	"context"
	"testing"

	testutil "github.com/Krokozabra213/gargantua_common/pkg/testutils"
	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentRevertLock_Success_CancellingToPending(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	err = testRepo.PaymentRevertLock(context.Background(),
		payment.ID,
		domain.PaymentStatusCancelling,
		domain.PaymentStatusPending,
	)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusPending, updated.Status)
}

func TestPaymentRevertLock_Success_RefundingToPaid(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankRefundingPayment(t, testRepo)

	err := testRepo.PaymentRevertLock(context.Background(),
		payment.ID,
		domain.PaymentStatusRefunding,
		domain.PaymentStatusCompleted,
	)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusCompleted, updated.Status)
}

func TestPaymentRevertLock_WrongLockStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.PaymentRevertLock(context.Background(),
		payment.ID,
		domain.PaymentStatusCancelling,
		domain.PaymentStatusPending,
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
}

func TestPaymentRevertLock_NonExistentPayment(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.PaymentRevertLock(context.Background(),
		uuid.New(),
		domain.PaymentStatusCancelling,
		domain.PaymentStatusPending,
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestPaymentRevertLock_AlreadyReverted(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	err = testRepo.PaymentRevertLock(context.Background(),
		payment.ID,
		domain.PaymentStatusCancelling,
		domain.PaymentStatusPending,
	)
	require.NoError(t, err)

	// Второй раз — уже не cancelling, а pending
	err = testRepo.PaymentRevertLock(context.Background(),
		payment.ID,
		domain.PaymentStatusCancelling,
		domain.PaymentStatusPending,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestPaymentRevertLock_CanRelockAfterRevert(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	// Lock
	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	// Revert
	err = testRepo.PaymentRevertLock(context.Background(),
		payment.ID,
		domain.PaymentStatusCancelling,
		domain.PaymentStatusPending,
	)
	require.NoError(t, err)

	// Lock again
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
}

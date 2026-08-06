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

func TestPaymentFinishCancel_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	err = testRepo.PaymentFinishCancel(context.Background(), payment.ID)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusCancelled, updated.Status)
}

func TestPaymentFinishCancel_WrongStatus_Pending(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.PaymentFinishCancel(context.Background(), payment.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
}

func TestPaymentFinishCancel_WrongStatus_Paid(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPaidPayment(t, testRepo)

	err := testRepo.PaymentFinishCancel(context.Background(), payment.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestPaymentFinishCancel_AlreadyCancelled(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	err = testRepo.PaymentFinishCancel(context.Background(), payment.ID)
	require.NoError(t, err)

	err = testRepo.PaymentFinishCancel(context.Background(), payment.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestPaymentFinishCancel_NonExistentPayment(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.PaymentFinishCancel(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestPaymentFinishCancel_NoOutboxCreated(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	err = testRepo.PaymentFinishCancel(context.Background(), payment.ID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM payments_outbox WHERE payment_id = $1`,
		payment.ID,
	).Scan(&count)
	require.NoError(t, err)

	assert.Equal(t, 0, count)
}

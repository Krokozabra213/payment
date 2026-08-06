//go:build integration

package tests

import (
	"context"
	"testing"

	testutil "github.com/Krokozabra213/gargantua_common/pkg/testutils"
	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTbankUpdateStatus_Success_Failed(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankUpdateStatus(context.Background(), domain.TbankUpdateStatusParams{
		IdempotencyKey: payment.IdempotencyKey,
		NewStatus:      domain.PaymentStatusFailed,
		CurrentStatus:  domain.PaymentStatusPending,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusFailed, updated.Status)
}

func TestTbankUpdateStatus_Success_Expired(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankUpdateStatus(context.Background(), domain.TbankUpdateStatusParams{
		IdempotencyKey: payment.IdempotencyKey,
		NewStatus:      domain.PaymentStatusExpired,
		CurrentStatus:  domain.PaymentStatusPending,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusExpired, updated.Status)
}

func TestTbankUpdateStatus_Success_Cancelled(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankUpdateStatus(context.Background(), domain.TbankUpdateStatusParams{
		IdempotencyKey: payment.IdempotencyKey,
		NewStatus:      domain.PaymentStatusCancelled,
		CurrentStatus:  domain.PaymentStatusPending,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusCancelled, updated.Status)
}

func TestTbankUpdateStatus_WrongCurrentStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
		p.Currency = "RUB"
	})

	err := testRepo.TbankUpdateStatus(context.Background(), domain.TbankUpdateStatusParams{
		IdempotencyKey: payment.IdempotencyKey,
		NewStatus:      domain.PaymentStatusFailed,
		CurrentStatus:  domain.PaymentStatusPending,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestTbankUpdateStatus_AlreadyInTargetStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankUpdateStatus(context.Background(), domain.TbankUpdateStatusParams{
		IdempotencyKey: payment.IdempotencyKey,
		NewStatus:      domain.PaymentStatusFailed,
		CurrentStatus:  domain.PaymentStatusPending,
	})
	require.NoError(t, err)

	err = testRepo.TbankUpdateStatus(context.Background(), domain.TbankUpdateStatusParams{
		IdempotencyKey: payment.IdempotencyKey,
		NewStatus:      domain.PaymentStatusFailed,
		CurrentStatus:  domain.PaymentStatusPending,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestTbankUpdateStatus_NonExistentPayment(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.TbankUpdateStatus(context.Background(), domain.TbankUpdateStatusParams{
		IdempotencyKey: "nonexistent-key",
		NewStatus:      domain.PaymentStatusFailed,
		CurrentStatus:  domain.PaymentStatusPending,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestTbankUpdateStatus_NoOutboxCreated(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankUpdateStatus(context.Background(), domain.TbankUpdateStatusParams{
		IdempotencyKey: payment.IdempotencyKey,
		NewStatus:      domain.PaymentStatusFailed,
		CurrentStatus:  domain.PaymentStatusPending,
	})
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

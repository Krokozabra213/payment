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

func TestMarkTGPaymentInitFailed_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	params := createReservePayment(t)
	payment, inserted, err := testRepo.PaymentReserve(context.Background(), params)
	require.NoError(t, err)
	require.True(t, inserted)

	err = testRepo.MarkTGPaymentInitFailed(context.Background(), payment.ID)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusFailed, updated.Status)
	assert.True(t, updated.UpdatedAt.After(payment.UpdatedAt) || updated.UpdatedAt.Equal(payment.UpdatedAt))
}

func TestMarkTGPaymentInitFailed_WrongStatus_Pending(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	id := seedPaymentReturnID(t, testRepo) // создаёт со статусом pending

	err := testRepo.MarkTGPaymentInitFailed(context.Background(), id)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestMarkTGPaymentInitFailed_WrongStatus_AlreadyFailed(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	params := createReservePayment(t)
	payment, inserted, err := testRepo.PaymentReserve(context.Background(), params)
	require.NoError(t, err)
	require.True(t, inserted)

	err = testRepo.MarkTGPaymentInitFailed(context.Background(), payment.ID)
	require.NoError(t, err)

	// повторный вызов — уже не started, а failed
	err = testRepo.MarkTGPaymentInitFailed(context.Background(), payment.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestMarkTGPaymentInitFailed_NonExistentPayment(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.MarkTGPaymentInitFailed(context.Background(), uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

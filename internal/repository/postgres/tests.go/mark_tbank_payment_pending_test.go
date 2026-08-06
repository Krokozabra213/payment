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

func TestMarkTbankPaymentPending_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
	})

	err := testRepo.MarkTbankPaymentPending(
		context.Background(),
		payment.ID,
		"https://pay.tbank.ru/xyz",
		"tbank-payment-123",
	)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusPending, updated.Status)
	assert.Equal(t, "https://pay.tbank.ru/xyz", updated.PaymentURL)
	assert.Equal(t, "tbank-payment-123", updated.ProviderPaymentID)
	assert.True(t, updated.UpdatedAt.After(payment.UpdatedAt) || updated.UpdatedAt.Equal(payment.UpdatedAt))
}

func TestMarkTbankPaymentPending_WrongStatus_Pending(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
	})

	err := testRepo.MarkTbankPaymentPending(context.Background(), payment.ID, "https://pay.tbank.ru/first", "id-1")
	require.NoError(t, err)

	err = testRepo.MarkTbankPaymentPending(context.Background(), payment.ID, "https://pay.tbank.ru/second", "id-2")
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://pay.tbank.ru/first", unchanged.PaymentURL)
	assert.Equal(t, "id-1", unchanged.ProviderPaymentID)
}

func TestMarkTbankPaymentPending_WrongStatus_Failed(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
	})

	err := testRepo.MarkTGPaymentInitFailed(context.Background(), payment.ID)
	require.NoError(t, err)

	err = testRepo.MarkTbankPaymentPending(context.Background(), payment.ID, "https://pay.tbank.ru/xyz", "id-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestMarkTbankPaymentPending_NonExistentPayment(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.MarkTbankPaymentPending(context.Background(), uuid.New(), "https://pay.tbank.ru/xyz", "id-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestMarkTbankPaymentPending_SavesProviderPaymentID(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
	})

	providerID := "tbank-unique-id-" + time.Now().Format("150405")

	err := testRepo.MarkTbankPaymentPending(
		context.Background(),
		payment.ID,
		"https://pay.tbank.ru/test",
		providerID,
	)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, providerID, updated.ProviderPaymentID)
	assert.Equal(t, domain.PaymentStatusPending, updated.Status)
}

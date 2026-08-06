//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func createTestPayment(t *testing.T, overrides ...func(*domain.CreatePaymentParams)) domain.CreatePaymentParams {
	t.Helper()

	idempotencyKey := "test-" + time.Now().Format("20060102-150405.000")
	providerPaymentID := uuid.New().String()

	p := domain.CreatePaymentParams{
		UserID:            uuid.New(),
		IdempotencyKey:    idempotencyKey,
		Amount:            10000,
		Currency:          "RUB",
		ProviderName:      string(domain.PaymentTbankForm),
		ProviderPaymentID: ptr(providerPaymentID),
		PaymentURL:        ptr("https://pay.tbank.ru/xyz"),
		Description:       ptr("test payment"),
		ExpiresAt:         ptr(time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)),
	}

	for _, fn := range overrides {
		fn(&p)
	}

	return p
}

func seedPayment(t *testing.T, testRepo *pgRepo.Repository, overrides ...func(*domain.CreatePaymentParams)) domain.CreatePaymentParams {
	t.Helper()
	p := createTestPayment(t, overrides...)
	_, err := testRepo.CreatePaymentReturnID(context.Background(), p)
	require.NoError(t, err)
	return p
}

func seedPaymentReturnID(t *testing.T, testRepo *pgRepo.Repository, overrides ...func(*domain.CreatePaymentParams)) uuid.UUID {
	t.Helper()
	p := createTestPayment(t, overrides...)
	id, err := testRepo.CreatePaymentReturnID(context.Background(), p)
	require.NoError(t, err)
	return id
}

func seedUSDRate(t *testing.T, testRepo *pgRepo.Repository) {
	t.Helper()
	err := testRepo.SaveRates(context.Background(), []domain.CurrencyRate{{
		Code:       domain.CurrencyUSD,
		RubRate:    9000,
		SourceName: domain.CurrencySourceCBR,
		SourceAt:   time.Now().UTC().Truncate(time.Second),
	}})
	require.NoError(t, err)
}

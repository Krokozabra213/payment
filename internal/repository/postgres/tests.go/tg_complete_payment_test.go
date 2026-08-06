//go:build integration

package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"

	testutil "github.com/Krokozabra213/gargantua_common/pkg/testutils"
	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedProcessingPaymentForComplete(
	t *testing.T,
	testRepo *pgRepo.Repository,
	overrides ...func(*domain.CreatePaymentParams),
) *domain.Payment {
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

func TestTGCompletePayment_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedProcessingPaymentForComplete(t, testRepo)
	seedUSDRate(t, testRepo)

	err := testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_123",
		Amount:                  payment.Amount,
		Currency:                payment.Currency,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusCompleted, updated.Status)
	assert.Equal(t, "tg_charge_123", updated.ProviderPaymentID)
	assert.NotNil(t, updated.PaidAt)
	assert.True(t, updated.UpdatedAt.After(payment.UpdatedAt) || updated.UpdatedAt.Equal(payment.UpdatedAt))
}

func TestTGCompletePayment_OutboxCreated(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedProcessingPaymentForComplete(t, testRepo)
	seedUSDRate(t, testRepo)

	err := testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_456",
		Amount:                  payment.Amount,
		Currency:                payment.Currency,
	})
	require.NoError(t, err)

	expectedOperationID := fmt.Sprintf("deposit:telegram:%s", payment.ID.String())

	var count int
	err = pool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM payments_outbox WHERE operation_id = $1`,
		expectedOperationID,
	).Scan(&count)
	require.NoError(t, err)

	assert.Equal(t, 1, count)

	var opType string
	var amount int
	var paymentID uuid.UUID
	err = pool.QueryRow(
		context.Background(),
		`SELECT payment_id, type, amount FROM payments_outbox WHERE operation_id = $1`,
		expectedOperationID,
	).Scan(&paymentID, &opType, &amount)
	require.NoError(t, err)

	assert.Equal(t, payment.ID, paymentID)
	assert.Equal(t, string(domain.OpTypeDeposit), opType)
	assert.Equal(t, payment.Amount, amount)
}

func TestTGCompletePayment_AlreadyPaid_Idempotent(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedProcessingPaymentForComplete(t, testRepo)
	seedUSDRate(t, testRepo)

	params := domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_789",
		Amount:                  payment.Amount,
		Currency:                payment.Currency,
	}

	err := testRepo.TGCompletePayment(context.Background(), params)
	require.NoError(t, err)

	err = testRepo.TGCompletePayment(context.Background(), params)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrAlreadyPaid)

	expectedOperationID := fmt.Sprintf("deposit:telegram:%s", payment.ID.String())
	var count int
	err = pool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM payments_outbox WHERE operation_id = $1`,
		expectedOperationID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestTGCompletePayment_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          "nonexistent-key",
		TelegramPaymentChargeID: "tg_charge_000",
		Amount:                  100,
		Currency:                "XTR",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrNotFound)
}

func TestTGCompletePayment_WrongStatus_Pending(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedPendingPayment(t, testRepo)

	err := testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_111",
		Amount:                  payment.Amount,
		Currency:                payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentStatus)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
}

func TestTGCompletePayment_WrongStatus_Started(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo)

	err := testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_222",
		Amount:                  payment.Amount,
		Currency:                payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentStatus)
}

func TestTGCompletePayment_WrongStatus_Failed(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo)
	err := testRepo.MarkTGPaymentInitFailed(context.Background(), payment.ID)
	require.NoError(t, err)

	err = testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_333",
		Amount:                  payment.Amount,
		Currency:                payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentStatus)
}

func TestTGCompletePayment_WrongAmount(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedProcessingPaymentForComplete(t, testRepo)

	err := testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_444",
		Amount:                  999999,
		Currency:                payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrAmountMismatch)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusProcessed, unchanged.Status)
}

func TestTGCompletePayment_WrongCurrency(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedProcessingPaymentForComplete(t, testRepo)

	err := testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_555",
		Amount:                  payment.Amount,
		Currency:                "RUB",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrCurrencyMismatch)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusProcessed, unchanged.Status)
}

func TestTGCompletePayment_Concurrent_OnlyOneCompletes(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedProcessingPaymentForComplete(t, testRepo)
	seedUSDRate(t, testRepo)

	params := domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_concurrent",
		Amount:                  payment.Amount,
		Currency:                payment.Currency,
	}

	type result struct {
		err error
	}

	start := make(chan struct{})
	results := make(chan result, 2)

	for range 2 {
		go func() {
			<-start
			err := testRepo.TGCompletePayment(context.Background(), params)
			results <- result{err: err}
		}()
	}

	close(start)

	r1 := <-results
	r2 := <-results

	successCount := 0
	alreadyPaidCount := 0

	for _, r := range []result{r1, r2} {
		if r.err == nil {
			successCount++
		} else if errors.Is(r.err, pgRepo.ErrAlreadyPaid) {
			alreadyPaidCount++
		} else {
			t.Errorf("unexpected error: %v", r.err)
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, alreadyPaidCount)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusCompleted, updated.Status)

	expectedOperationID := fmt.Sprintf("deposit:telegram:%s", payment.ID.String())
	var count int
	err = pool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM payments_outbox WHERE operation_id = $1`,
		expectedOperationID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestTGCompletePayment_ProviderMismatch(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
	})

	err := testRepo.MarkTGPaymentPending(context.Background(), payment.ID, "https://pay.tbank.ru/xyz")
	require.NoError(t, err)

	err = testRepo.TGCompletePayment(context.Background(), domain.TGCompletePaymentParams{
		IdempotencyKey:          payment.IdempotencyKey,
		TelegramPaymentChargeID: "tg_charge_666",
		Amount:                  payment.Amount,
		Currency:                payment.Currency,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrProviderMismatch)
}

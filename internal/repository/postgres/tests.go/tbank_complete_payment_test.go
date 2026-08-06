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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedTbankPendingPayment(
	t *testing.T,
	testRepo *pgRepo.Repository,
	overrides ...func(*domain.CreatePaymentParams),
) *domain.Payment {
	t.Helper()

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
		p.Currency = "RUB"
	})

	for _, fn := range overrides {
		params := domain.CreatePaymentParams{}
		fn(&params)
		_ = params
	}

	err := testRepo.MarkTbankPaymentPending(
		context.Background(),
		payment.ID,
		"https://pay.tbank.ru/test",
		"tbank-pid-123",
	)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	return updated
}

func seedTbankPaidPayment(
	t *testing.T,
	testRepo *pgRepo.Repository,
) *domain.Payment {
	t.Helper()

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: payment.ProviderPaymentID,
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	return updated
}

func TestTbankCompletePayment_Success_Paid(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: payment.ProviderPaymentID,
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusCompleted, updated.Status)
	assert.NotNil(t, updated.PaidAt)
}

func TestTbankCompletePayment_Success_Refunded(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPaidPayment(t, testRepo)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: payment.ProviderPaymentID,
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusRefunded,
		CurrentStatus:     domain.PaymentStatusCompleted,
		OpType:            domain.OpTypeRefund,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusRefunded, updated.Status)
}

func TestTbankCompletePayment_OutboxCreated_Deposit(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: payment.ProviderPaymentID,
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})
	require.NoError(t, err)

	expectedOpID := fmt.Sprintf("deposit:tbank:%s", payment.ID.String())

	var opType string
	var amount int
	err = pool.QueryRow(
		context.Background(),
		`SELECT type, amount FROM payments_outbox WHERE operation_id = $1`,
		expectedOpID,
	).Scan(&opType, &amount)
	require.NoError(t, err)

	assert.Equal(t, string(domain.OpTypeDeposit), opType)
	assert.Equal(t, payment.Amount, amount)
}

func TestTbankCompletePayment_OutboxCreated_Refund(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPaidPayment(t, testRepo)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: payment.ProviderPaymentID,
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusRefunded,
		CurrentStatus:     domain.PaymentStatusCompleted,
		OpType:            domain.OpTypeRefund,
	})
	require.NoError(t, err)

	expectedOpID := fmt.Sprintf("refund:tbank:%s", payment.ID.String())

	var opType string
	var amount int
	err = pool.QueryRow(
		context.Background(),
		`SELECT type, amount FROM payments_outbox WHERE operation_id = $1`,
		expectedOpID,
	).Scan(&opType, &amount)
	require.NoError(t, err)

	assert.Equal(t, string(domain.OpTypeRefund), opType)
	assert.Equal(t, payment.Amount, amount)
}

func TestTbankCompletePayment_AlreadyPaid_Idempotent(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPaidPayment(t, testRepo)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: payment.ProviderPaymentID,
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrAlreadyProcessed)
}

func TestTbankCompletePayment_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    "nonexistent-key",
		ProviderPaymentID: "pid-000",
		Amount:            100,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrNotFound)
}

func TestTbankCompletePayment_WrongStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedReservedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTbankForm)
		p.Currency = "RUB"
	})

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: "pid-111",
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestTbankCompletePayment_AmountMismatch(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: payment.ProviderPaymentID,
		Amount:            999999,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrAmountMismatch)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
}

func TestTbankCompletePayment_ProviderPaymentIDMismatch(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.TbankCompletePayment(context.Background(), domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: "wrong-provider-id",
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrProviderPaymentIDMismatch)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
}

func TestTbankCompletePayment_Concurrent_OnlyOneCompletes(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	params := domain.TbankCompleteParams{
		IdempotencyKey:    payment.IdempotencyKey,
		ProviderPaymentID: payment.ProviderPaymentID,
		Amount:            payment.Amount,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	}

	type result struct {
		err error
	}

	start := make(chan struct{})
	results := make(chan result, 2)

	for range 2 {
		go func() {
			<-start
			err := testRepo.TbankCompletePayment(context.Background(), params)
			results <- result{err: err}
		}()
	}

	close(start)

	r1 := <-results
	r2 := <-results

	successCount := 0
	alreadyCount := 0

	for _, r := range []result{r1, r2} {
		if r.err == nil {
			successCount++
		} else if errors.Is(r.err, pgRepo.ErrAlreadyProcessed) {
			alreadyCount++
		} else {
			t.Errorf("unexpected error: %v", r.err)
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, alreadyCount)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusCompleted, updated.Status)

	expectedOpID := fmt.Sprintf("deposit:tbank:%s", payment.ID.String())
	var count int
	err = pool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM payments_outbox WHERE operation_id = $1`,
		expectedOpID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

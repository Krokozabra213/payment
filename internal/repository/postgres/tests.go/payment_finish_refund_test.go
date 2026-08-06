//go:build integration


package tests

import (
	"context"
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

func seedTbankRefundingPayment(
	t *testing.T,
	testRepo *pgRepo.Repository,
) *domain.Payment {
	t.Helper()

	payment := seedTbankPaidPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusCompleted,
		LockStatus:    domain.PaymentStatusRefunding,
	})
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	return updated
}

func TestPaymentFinishRefund_Success(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankRefundingPayment(t, testRepo)

	err := testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.PaymentStatusRefunded, updated.Status)
}

func TestPaymentFinishRefund_OutboxCreated(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankRefundingPayment(t, testRepo)

	err := testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.NoError(t, err)

	expectedOpID := fmt.Sprintf("refund:%s", payment.ID.String())

	var opType string
	var amount int
	var paymentID uuid.UUID
	err = pool.QueryRow(
		context.Background(),
		`SELECT payment_id, type, amount FROM payments_outbox WHERE operation_id = $1`,
		expectedOpID,
	).Scan(&paymentID, &opType, &amount)
	require.NoError(t, err)

	assert.Equal(t, payment.ID, paymentID)
	assert.Equal(t, string(domain.OpTypeRefund), opType)
	assert.Equal(t, payment.Amount, amount)
}

func TestPaymentFinishRefund_OutboxNotDuplicated(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankRefundingPayment(t, testRepo)

	err := testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.NoError(t, err)

	// Вручную вернём в refunding чтобы попробовать ещё раз
	_, err = pool.Exec(context.Background(),
		`UPDATE payments SET status = 'refunding' WHERE id = $1`,
		payment.ID,
	)
	require.NoError(t, err)

	err = testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.Error(t, err) // outbox duplicate

	expectedOpID := fmt.Sprintf("refund:%s", payment.ID.String())
	var count int
	err = pool.QueryRow(
		context.Background(),
		`SELECT count(*) FROM payments_outbox WHERE operation_id = $1`,
		expectedOpID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPaymentFinishRefund_WrongStatus_Pending(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	err := testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, unchanged.Status)
}

func TestPaymentFinishRefund_WrongStatus_Paid(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPaidPayment(t, testRepo)

	err := testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)

	unchanged, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusCompleted, unchanged.Status)
}

func TestPaymentFinishRefund_WrongStatus_Cancelling(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankPendingPayment(t, testRepo)

	_, err := testRepo.PaymentCancelLock(context.Background(), domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	require.NoError(t, err)

	err = testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestPaymentFinishRefund_AlreadyRefunded(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	payment := seedTbankRefundingPayment(t, testRepo)

	err := testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.NoError(t, err)

	updated, err := testRepo.GetPaymentByID(context.Background(), payment.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusRefunded, updated.Status)

	err = testRepo.PaymentFinishRefund(context.Background(), payment.ID, payment.Amount)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

func TestPaymentFinishRefund_NonExistentPayment(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	err := testRepo.PaymentFinishRefund(context.Background(), uuid.New(), 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, pgRepo.ErrInvalidPaymentState)
}

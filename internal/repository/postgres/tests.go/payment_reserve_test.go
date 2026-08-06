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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createReservePayment(t *testing.T, overrides ...func(*domain.CreatePaymentParams)) domain.CreatePaymentParams {
	t.Helper()

	p := createTestPayment(t, func(p *domain.CreatePaymentParams) {
		p.ProviderName = string(domain.PaymentTGStars)
		p.ProviderPaymentID = nil
		p.PaymentURL = nil
		p.Currency = "XTR"
	})

	for _, fn := range overrides {
		fn(&p)
	}

	return p
}

func countPaymentsByScope(
	t *testing.T,
	pool *pgxpool.Pool,
	providerName string,
	userID uuid.UUID,
	idempotencyKey string,
) int {
	t.Helper()

	var count int
	err := pool.QueryRow(
		context.Background(),
		`
			SELECT count(*)
			FROM payments
			WHERE idempotency_key = $1
		`,
		idempotencyKey,
	).Scan(&count)
	require.NoError(t, err)

	return count
}

func TestPaymentReserve_InsertNewPayment(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	params := createReservePayment(t)

	payment, inserted, err := testRepo.PaymentReserve(context.Background(), params)
	require.NoError(t, err)

	require.True(t, inserted)
	require.NotNil(t, payment)

	assert.Equal(t, params.UserID, payment.UserID)
	assert.Equal(t, params.IdempotencyKey, payment.IdempotencyKey)
	assert.Equal(t, params.Amount, payment.Amount)
	assert.Equal(t, params.Currency, payment.Currency)
	assert.Equal(t, params.ProviderName, payment.ProviderName)
	assert.Equal(t, domain.PaymentStatusStarted, payment.Status)

	assert.Equal(t, 1, countPaymentsByScope(
		t,
		pool,
		params.ProviderName,
		params.UserID,
		params.IdempotencyKey,
	))
}

func TestPaymentReserve_ReturnsExistingPayment_OnDuplicate(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	params := createReservePayment(t)

	first, inserted, err := testRepo.PaymentReserve(context.Background(), params)
	require.NoError(t, err)
	require.True(t, inserted)
	require.NotNil(t, first)

	second, inserted, err := testRepo.PaymentReserve(context.Background(), params)
	require.NoError(t, err)
	require.False(t, inserted)
	require.NotNil(t, second)

	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, first.UserID, second.UserID)
	assert.Equal(t, first.IdempotencyKey, second.IdempotencyKey)
	assert.Equal(t, first.ProviderName, second.ProviderName)
	assert.Equal(t, first.Status, second.Status)

	assert.Equal(t, 1, countPaymentsByScope(
		t,
		pool,
		params.ProviderName,
		params.UserID,
		params.IdempotencyKey,
	))
}

func TestPaymentReserve_NilOptionalFields(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	params := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.ProviderPaymentID = nil
		p.PaymentURL = nil
		p.Description = nil
		p.ExpiresAt = nil
	})

	payment, inserted, err := testRepo.PaymentReserve(context.Background(), params)
	require.NoError(t, err)
	require.True(t, inserted)
	require.NotNil(t, payment)

	assert.Equal(t, domain.PaymentStatusStarted, payment.Status)
}

func TestPaymentReserve_SameIdempotencyKeyDifferentUser_InsertsNewRow(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	idempotencyKey := "same-key-" + time.Now().Format("20060102-150405.000")

	params1 := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
	})
	params2 := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = uuid.New()
	})

	p1, inserted1, err := testRepo.PaymentReserve(context.Background(), params1)
	require.NoError(t, err)
	require.True(t, inserted1)

	p2, inserted2, err := testRepo.PaymentReserve(context.Background(), params2)
	require.NoError(t, err)
	require.False(t, inserted2)

	assert.Equal(t, p1.ID, p2.ID)
}

func TestPaymentReserve_SameIdempotencyKeyDifferentProvider_InsertsNewRow(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	idempotencyKey := "same-key-" + time.Now().Format("20060102-150405.000")
	userID := uuid.New()

	params1 := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = userID
		p.ProviderName = string(domain.PaymentTGStars)
	})
	params2 := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = userID
		p.ProviderName = string(domain.PaymentTbankForm)
	})

	p1, inserted1, err := testRepo.PaymentReserve(context.Background(), params1)
	require.NoError(t, err)
	require.True(t, inserted1)

	p2, inserted2, err := testRepo.PaymentReserve(context.Background(), params2)
	require.NoError(t, err)
	require.False(t, inserted2)

	assert.Equal(t, p1.ID, p2.ID)
}

func TestPaymentReserve_ReturnsAlreadyExistingPendingPayment(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	idempotencyKey := "same-key-" + time.Now().Format("20060102-150405.000")
	userID := uuid.New()
	provider := string(domain.PaymentTGStars)

	seedPayment(t, testRepo, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = userID
		p.ProviderName = provider
	})

	params := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = userID
		p.ProviderName = provider
	})

	payment, inserted, err := testRepo.PaymentReserve(context.Background(), params)
	require.NoError(t, err)
	require.False(t, inserted)
	require.NotNil(t, payment)

	assert.Equal(t, domain.PaymentStatusPending, payment.Status)
	assert.Equal(t, idempotencyKey, payment.IdempotencyKey)
	assert.Equal(t, provider, payment.ProviderName)
	assert.Equal(t, userID, payment.UserID)
}

func TestPaymentReserve_DuplicateWithDifferentPayload_ReturnsExistingRow(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	idempotencyKey := "same-key-" + time.Now().Format("20060102-150405.000")
	userID := uuid.New()
	provider := string(domain.PaymentTGStars)

	firstParams := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = userID
		p.ProviderName = provider
		p.Amount = 100
		p.Currency = "XTR"
	})

	first, inserted, err := testRepo.PaymentReserve(context.Background(), firstParams)
	require.NoError(t, err)
	require.True(t, inserted)

	secondParams := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = userID
		p.ProviderName = provider
		p.Amount = 999999
		p.Currency = "RUB"
	})

	second, inserted, err := testRepo.PaymentReserve(context.Background(), secondParams)
	require.NoError(t, err)
	require.False(t, inserted)

	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, first.Amount, second.Amount)
	assert.Equal(t, first.Currency, second.Currency)
}

func TestPaymentReserve_ConcurrentCalls_OnlyOneInsert(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	params := createReservePayment(t)

	type result struct {
		payment  *domain.Payment
		inserted bool
		err      error
	}

	start := make(chan struct{})
	results := make(chan result, 2)

	for range 2 {
		go func() {
			<-start
			payment, inserted, err := testRepo.PaymentReserve(context.Background(), params)
			results <- result{
				payment:  payment,
				inserted: inserted,
				err:      err,
			}
		}()
	}

	close(start)

	r1 := <-results
	r2 := <-results

	require.NoError(t, r1.err)
	require.NoError(t, r2.err)
	require.NotNil(t, r1.payment)
	require.NotNil(t, r2.payment)

	insertedCount := 0
	if r1.inserted {
		insertedCount++
	}
	if r2.inserted {
		insertedCount++
	}

	assert.Equal(t, 1, insertedCount)
	assert.Equal(t, r1.payment.ID, r2.payment.ID)

	assert.Equal(t, 1, countPaymentsByScope(
		t,
		pool,
		params.ProviderName,
		params.UserID,
		params.IdempotencyKey,
	))
}

func TestPaymentReserve_SameIdempotencyKeyDifferentUser_ReturnsExistingRow(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	idempotencyKey := "same-key-" + time.Now().Format("20060102-150405.000")

	params1 := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
	})
	params2 := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = uuid.New()
	})

	first, inserted, err := testRepo.PaymentReserve(context.Background(), params1)
	require.NoError(t, err)
	require.True(t, inserted)

	second, inserted, err := testRepo.PaymentReserve(context.Background(), params2)
	require.NoError(t, err)
	require.False(t, inserted)

	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, first.UserID, second.UserID)
	assert.Equal(t, first.ProviderName, second.ProviderName)

	assert.Equal(t, 1, countPaymentsByScope(
		t,
		pool,
		first.ProviderName,
		first.UserID,
		first.IdempotencyKey,
	))
}

func TestPaymentReserve_SameIdempotencyKeyDifferentProvider_ReturnsExistingRow(t *testing.T) {
	pool := testutil.SetupTestDB(t, migrations.Files)
	testRepo := pgRepo.New(pool)

	idempotencyKey := "same-key-" + time.Now().Format("20060102-150405.000")
	userID := uuid.New()

	params1 := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = userID
		p.ProviderName = string(domain.PaymentTGStars)
	})
	params2 := createReservePayment(t, func(p *domain.CreatePaymentParams) {
		p.IdempotencyKey = idempotencyKey
		p.UserID = userID
		p.ProviderName = string(domain.PaymentTbankForm)
	})

	first, inserted, err := testRepo.PaymentReserve(context.Background(), params1)
	require.NoError(t, err)
	require.True(t, inserted)

	second, inserted, err := testRepo.PaymentReserve(context.Background(), params2)
	require.NoError(t, err)
	require.False(t, inserted)

	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, first.UserID, second.UserID)
	assert.Equal(t, first.ProviderName, second.ProviderName)

	assert.Equal(t, 1, countPaymentsByScope(
		t,
		pool,
		first.ProviderName,
		first.UserID,
		first.IdempotencyKey,
	))
}

//go:build e2e

package tbank_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/GargantuaLabs/payment/internal/providers/tbank"
	"github.com/Krokozabra213/common/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testBaseURL     = "https://securepay.tinkoff.ru/v2"
	testTerminalKey string
	testPassword    string
)

func TestMain(m *testing.M) {
	testTerminalKey = os.Getenv("TBANK_TERMINAL_KEY")
	testPassword = os.Getenv("TBANK_PASSWORD")

	if testTerminalKey == "" {
		println("ERROR: TBANK_TERMINAL_KEY environment variable is not set")
		os.Exit(1)
	}
	if testPassword == "" {
		println("ERROR: TBANK_PASSWORD environment variable is not set")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func newIntegrationProvider(t *testing.T) *tbank.Provider {
	t.Helper()

	log := slog.Default()

	return tbank.NewProvider(&config.TinkoffConfig{
		TerminalKey:         testTerminalKey,
		Password:            testPassword,
		BaseURL:             testBaseURL,
		Timeout:             10 * time.Second,
		MaxIdleConns:        5,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	}, log)
}

func TestIntegration_FullPaymentCycle(t *testing.T) {
	p := newIntegrationProvider(t)
	ctx := context.Background()

	providerOrderID := "test-" + time.Now().Format("20060102-150405.000")
	userID := uuid.New()
	amount, _ := types.NewPositiveInt(10_000)

	payload := domain.TbankInit{
		UserID:          userID,
		Amount:          amount,
		Desc:            "test desc",
		NotificationURL: "192.168.0.0",
	}

	initResult, err := p.Init(ctx, &payload, providerOrderID)

	require.NoError(t, err)

	assert.NotEmpty(t, initResult.PaymentID)
	assert.NotEmpty(t, initResult.PaymentURL)
	assert.Contains(t, initResult.PaymentURL.Value(), "pay.tbank.ru")

	time.Sleep(2 * time.Second)
	status, err := p.Status(ctx, initResult.PaymentID)

	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusPending, *status)

	res, err := p.Cancel(ctx, initResult.PaymentID)

	require.NoError(t, err)
	assert.Equal(t, initResult.PaymentID, res.PaymentID)
	assert.Equal(t, providerOrderID, res.ProviderOrderID.Value())

	statusAfterCancel, err := p.Status(ctx, initResult.PaymentID)

	require.NoError(t, err)
	assert.Equal(t, domain.PaymentStatusCancelled, *statusAfterCancel)
}

func TestIntegration_CancelPayment_NonExistent(t *testing.T) {
	p := newIntegrationProvider(t)
	ctx := context.Background()

	paymentID, _ := types.NewNonEmptyString("9999999999")

	res, err := p.Cancel(ctx, paymentID)

	require.Error(t, err)
	assert.Nil(t, res)
}

func TestIntegration_GetStatus_NonExistent(t *testing.T) {
	p := newIntegrationProvider(t)
	ctx := context.Background()

	paymentID, _ := types.NewNonEmptyString("9999999999")

	_, err := p.Status(ctx, paymentID)

	require.Error(t, err)
}

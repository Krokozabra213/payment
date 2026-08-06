//go:build e2e

package tg_test

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/GargantuaLabs/payment/internal/domain"
	tg "github.com/GargantuaLabs/payment/internal/providers/telegram"
	"github.com/Krokozabra213/common/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBaseURL = "https://api.telegram.org/bot"
)

func generateLongString(minLength int) (string, error) {
	if minLength < 1 {
		minLength = 1
	}
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetLen := big.NewInt(int64(len(charset)))

	b := make([]byte, minLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

func newIntegrationProvider(t *testing.T) (*tg.Provider, error) {
	t.Helper()

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN environment variable not set")
	}

	log := slog.Default()

	return tg.NewProvider(&config.TelegramConfig{
		BaseURL:             testBaseURL,
		BotToken:            botToken,
		Timeout:             10 * time.Second,
		MaxIdleConns:        5,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	}, log), nil
}

func TestInitPayment_Success(t *testing.T) {
	p, err := newIntegrationProvider(t)
	if err != nil {
		t.Fatalf("failed to create integration provider: %v", err)
	}
	ctx := context.Background()

	providerOrderID := "test-" + time.Now().Format("20060102-150405.000")
	orderID := uuid.New()

	payload, _ := domain.NewTGInitPayload(orderID.String(), 1, string(domain.CurrencyTypeTGStars), "Test Description", "test title")

	result, err := p.Init(ctx, payload, providerOrderID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.PaymentURL)
	assert.Contains(t, result.PaymentURL.Value(), "t.me")
}

func TestInitPayment_ContextTimeout(t *testing.T) {
	p, err := newIntegrationProvider(t)
	if err != nil {
		t.Fatalf("failed to create integration provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	providerOrderID := "test-" + time.Now().Format("20060102-150405.000")

	payload, _ := domain.NewTGInitPayload(uuid.New().String(), 1, string(domain.CurrencyTypeTGStars), "Test Description", "test title")

	result, err := p.Init(ctx, payload, providerOrderID)

	require.Error(t, err)
	require.Nil(t, result)
}

func TestInitValidationFailed_Payload(t *testing.T) {
	p, err := newIntegrationProvider(t)
	if err != nil {
		t.Fatalf("failed to create integration provider: %v", err)
	}
	ctx := context.Background()

	orderID := uuid.New()
	invalidPayload, err := generateLongString(1000)
	require.NoError(t, err, "failed generate invalid payload: %v", err)

	payload, _ := domain.NewTGInitPayload(orderID.String(), 1, string(domain.CurrencyTypeTGStars), "Test Description", "test title")

	result, err := p.Init(ctx, payload, invalidPayload)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "code=400")
}

func TestInitValidationFailed_Currency(t *testing.T) {
	p, err := newIntegrationProvider(t)
	if err != nil {
		t.Fatalf("failed to create integration provider: %v", err)
	}
	ctx := context.Background()

	providerOrderID := "test-" + time.Now().Format("20060102-150405.000")

	amount, _ := types.NewPositiveInt(1)
	desc, _ := types.NewNonEmptyString("Test")
	payload := &domain.TGInit{
		UserID:   uuid.New(),
		Amount:   amount,
		Currency: domain.CurrencyType("INVALID"),
		Desc:     desc,
		Title:    desc,
	}

	result, err := p.Init(ctx, payload, providerOrderID)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "code=400")
}

func TestInitValidationFailed_Title(t *testing.T) {
	p, err := newIntegrationProvider(t)
	if err != nil {
		t.Fatalf("failed to create integration provider: %v", err)
	}
	ctx := context.Background()

	providerOrderID := "test-" + time.Now().Format("20060102-150405.000")
	amount, _ := types.NewPositiveInt(1)
	desc, _ := types.NewNonEmptyString("Test")

	payload := &domain.TGInit{
		UserID:   uuid.New(),
		Amount:   amount,
		Currency: domain.CurrencyTypeTGStars,
		Desc:     desc,
	}

	result, err := p.Init(ctx, payload, providerOrderID)

	require.Error(t, err)
	require.Nil(t, result)
}

func TestInitPayment_SpecialCharactersInTitle(t *testing.T) {
	p, err := newIntegrationProvider(t)
	if err != nil {
		t.Fatalf("failed to create integration provider: %v", err)
	}
	ctx := context.Background()

	providerOrderID := "test-" + time.Now().Format("20060102-150405.000")

	payload, _ := domain.NewTGInitPayload(uuid.New().String(), 1, string(domain.CurrencyTypeTGStars), "Test with emoji 🎉 and <html>", "Title: спецсимволы & emoji 🚀")

	result, err := p.Init(ctx, payload, providerOrderID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.PaymentURL)
}

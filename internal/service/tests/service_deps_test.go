package test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/GargantuaLabs/payment/internal/config"
	svc "github.com/GargantuaLabs/payment/internal/service"
	"github.com/GargantuaLabs/payment/internal/service/mocks"
	"go.uber.org/mock/gomock"
)

func ptr[T any](v T) *T { return &v }

type testServiceDeps struct {
	dbRepo    *mocks.MockDBProvider
	tbank     *mocks.MockTbankProvider
	tg        *mocks.MockTGProvider
	fixedTime time.Time
	fixedID   string
}

func newTestService(t *testing.T) (*svc.Service, *testServiceDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)

	deps := &testServiceDeps{
		dbRepo:    mocks.NewMockDBProvider(ctrl),
		tbank:     mocks.NewMockTbankProvider(ctrl),
		tg:        mocks.NewMockTGProvider(ctrl),
		fixedTime: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
		fixedID:   "test-order-id-123",
	}

	svc := svc.New(
		&config.Config{},
		slog.Default(),
		deps.tbank,
		deps.tg,
		deps.dbRepo,
	)

	return svc, deps
}

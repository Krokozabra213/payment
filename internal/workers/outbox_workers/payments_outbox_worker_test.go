package outboxworkers

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/GargantuaLabs/payment/internal/config"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/internal/workers/outbox_workers/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testConfig() *config.ProducerConfig {
	return &config.ProducerConfig{
		BatchSize:    100,
		Lease:        time.Minute,
		SendTimeout:  time.Second,
		DBTimeout:    time.Second,
		MaxAttempts:  10,
		PollInterval: time.Second,
		ErrorBackoff: time.Second,
	}
}

func testRow() *pgRepo.PaymentsOutbox {
	return &pgRepo.PaymentsOutbox{
		ID:          uuid.New(),
		PaymentID:   uuid.New(),
		OperationID: "op-123",
		EventKey:    "event-key",
		Payload:     []byte(`{"operation_id":"op-123"}`),
	}
}

func TestProcessOne_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockDB(ctrl)
	producer := mocks.NewMockBalanceProducer(ctrl)

	cfg := testConfig()

	worker := NewPaymentWorker(
		testLogger(),
		db,
		producer,
		cfg,
	)

	row := testRow()

	producer.EXPECT().
		PublishBalance(
			gomock.Any(),
			gomock.Any(),
		).
		Return(nil).
		Times(1)

	db.EXPECT().
		MarkOutboxProcessed(
			gomock.Any(),
			row,
			worker.workerID,
		).
		Return(nil).
		Times(1)

	worker.processOne(context.Background(), row)
}

func TestProcessOne_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockDB(ctrl)
	producer := mocks.NewMockBalanceProducer(ctrl)

	cfg := testConfig()

	worker := NewPaymentWorker(
		testLogger(),
		db,
		producer,
		cfg,
	)

	row := testRow()

	publishErr := assert.AnError

	producer.EXPECT().
		PublishBalance(
			gomock.Any(),
			gomock.Any(),
		).
		Return(publishErr).
		Times(1)

	db.EXPECT().
		MarkRetryOrFail(
			gomock.Any(),
			row,
			publishErr,
			cfg.MaxAttempts,
			worker.workerID,
		).
		Return(nil).
		Times(1)

	worker.processOne(context.Background(), row)
}

func TestProcessOne_MarkProcessedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockDB(ctrl)
	producer := mocks.NewMockBalanceProducer(ctrl)

	cfg := testConfig()

	worker := NewPaymentWorker(
		testLogger(),
		db,
		producer,
		cfg,
	)

	row := testRow()

	producer.EXPECT().
		PublishBalance(
			gomock.Any(),
			gomock.Any(),
		).
		Return(nil).
		Times(1)

	db.EXPECT().
		MarkOutboxProcessed(
			gomock.Any(),
			row,
			worker.workerID,
		).
		Return(assert.AnError).
		Times(1)

	require.NotPanics(t, func() {
		worker.processOne(context.Background(), row)
	})
}

func TestRunOnce_ClaimBatchError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockDB(ctrl)
	producer := mocks.NewMockBalanceProducer(ctrl)

	cfg := testConfig()

	worker := NewPaymentWorker(
		testLogger(),
		db,
		producer,
		cfg,
	)

	db.EXPECT().
		ClaimBatch(
			gomock.Any(),
			cfg.BatchSize,
			worker.workerID,
			cfg.Lease,
		).
		Return(nil, assert.AnError)

	n, err := worker.runOnce(context.Background())

	require.Error(t, err)
	require.Equal(t, 0, n)
}

func TestRunOnce_EmptyBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockDB(ctrl)
	producer := mocks.NewMockBalanceProducer(ctrl)

	cfg := testConfig()

	worker := NewPaymentWorker(
		testLogger(),
		db,
		producer,
		cfg,
	)

	db.EXPECT().
		ClaimBatch(
			gomock.Any(),
			cfg.BatchSize,
			worker.workerID,
			cfg.Lease,
		).
		Return([]pgRepo.PaymentsOutbox{}, nil)

	n, err := worker.runOnce(context.Background())

	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestRunOnce_OneRow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mocks.NewMockDB(ctrl)
	producer := mocks.NewMockBalanceProducer(ctrl)

	cfg := testConfig()

	worker := NewPaymentWorker(
		testLogger(),
		db,
		producer,
		cfg,
	)

	row := *testRow()

	db.EXPECT().
		ClaimBatch(
			gomock.Any(),
			cfg.BatchSize,
			worker.workerID,
			cfg.Lease,
		).
		Return([]pgRepo.PaymentsOutbox{row}, nil)

	producer.EXPECT().
		PublishBalance(
			gomock.Any(),
			gomock.Any(),
		).
		Return(nil)

	db.EXPECT().
		MarkOutboxProcessed(
			gomock.Any(),
			gomock.Any(),
			worker.workerID,
		).
		Return(nil)

	n, err := worker.runOnce(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, n)
}

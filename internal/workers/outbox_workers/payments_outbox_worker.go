package outboxworkers

import (
	"context"
	"log/slog"
	"time"

	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/google/uuid"
)

//go:generate mockgen -source=payments_outbox_worker.go -destination=mocks/mock_worker.go -package=mocks

type DB interface {
	ClaimBatch(ctx context.Context, batchSize int, workerID string, lease time.Duration) ([]pgRepo.PaymentsOutbox, error)
	MarkRetryOrFail(ctx context.Context, row *pgRepo.PaymentsOutbox, cause error, maxAttempts int, workerID string) error
	MarkOutboxProcessed(ctx context.Context, row *pgRepo.PaymentsOutbox, workerID string) error
}

type BalanceProducer interface {
	PublishBalance(ctx context.Context, balance *domain.BalanceEvent) error
}

type PaymentsWorker struct {
	logger   *slog.Logger
	db       DB
	producer BalanceProducer
	cfg      *config.ProducerConfig
	workerID string
}

func NewPaymentWorker(
	logger *slog.Logger,
	db DB,
	producer BalanceProducer,
	cfg *config.ProducerConfig,
) *PaymentsWorker {
	return &PaymentsWorker{
		db:       db,
		producer: producer,
		logger:   logger,
		cfg:      cfg,
		workerID: uuid.NewString(),
	}
}

func (w *PaymentsWorker) Run(ctx context.Context) error {
	w.logger.Info("paymentoutbox worker started",
		"topic", w.cfg.PaymentTopic,
		"batch_size", w.cfg.BatchSize,
		"lease", w.cfg.Lease,
	)

	for {
		if ctx.Err() != nil {
			w.logger.Info("outbox worker stopped")
			return nil
		}

		n, err := w.runOnce(ctx)
		if err != nil {
			w.logger.Error("outbox runOnce failed", "error", err)

			if err := sleep(ctx, w.cfg.ErrorBackoff); err != nil {
				return nil
			}
			continue
		}

		if n == 0 {
			if err := sleep(ctx, w.cfg.PollInterval); err != nil {
				return nil
			}
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (w *PaymentsWorker) runOnce(ctx context.Context) (int, error) {
	dbCtx, cancel := context.WithTimeout(ctx, w.cfg.DBTimeout)
	defer cancel()

	rows, err := w.db.ClaimBatch(dbCtx, w.cfg.BatchSize, w.workerID, w.cfg.Lease)
	if err != nil {
		return 0, err
	}

	for i := range rows {
		if ctx.Err() != nil {
			return len(rows), nil
		}
		w.processOne(ctx, &rows[i])
	}

	return len(rows), nil
}

func (w *PaymentsWorker) processOne(ctx context.Context, row *pgRepo.PaymentsOutbox) {
	logger := w.logger.With(
		"outbox_id", row.ID.String(),
		"operation_id", row.OperationID,
		"payment_id", row.PaymentID.String(),
		"attempts", row.Attempts,
	)

	sendCtx, cancel := context.WithTimeout(ctx, w.cfg.SendTimeout)
	err := w.producer.PublishBalance(
		sendCtx,
		row.ToBalanceEvent(),
	)
	cancel()

	if err != nil {
		logger.Warn("kafka publish failed", "error", err)

		dbCtx, cancel := context.WithTimeout(ctx, w.cfg.DBTimeout)
		defer cancel()

		if updErr := w.db.MarkRetryOrFail(dbCtx, row, err, w.cfg.MaxAttempts, w.workerID); updErr != nil {
			logger.Error(
				"failed to persist retry state; row remains processing until lease expires, then will be reclaimed",
				"update_error", updErr,
				"original_error", err,
			)
		}
		return
	}

	dbCtx, cancel := context.WithTimeout(ctx, w.cfg.DBTimeout)
	defer cancel()

	if err := w.db.MarkOutboxProcessed(dbCtx, row, w.workerID); err != nil {
		logger.Error(
			"kafka publish likely succeeded, but failed to mark processed; after lease expiration row may be published again",
			"error", err,
		)
		return
	}

	logger.Debug("outbox record processed")
}

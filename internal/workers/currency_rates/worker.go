package currency_rates

import (
	"context"
	"fmt"
	"time"

	"github.com/GargantuaLabs/payment/internal/domain"
)

type Source interface {
	Fetch(ctx context.Context, codes []domain.CurrencyCode) ([]domain.CurrencyRate, error)
}

type Repository interface {
	SaveRates(ctx context.Context, rates []domain.CurrencyRate) error
}

type Worker struct {
	repo   Repository
	source Source
	codes  []domain.CurrencyCode
}

func NewWorker(
	repo Repository,
	source Source,
	codes []domain.CurrencyCode,
) *Worker {
	return &Worker{
		repo:   repo,
		source: source,
		codes:  codes,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	return w.sync(ctx)
}

func (w *Worker) sync(ctx context.Context) error {
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	rates, err := w.source.Fetch(runCtx, w.codes)
	if err != nil {
		return fmt.Errorf("failed to fetch currency rates: %w", err)
	}

	if err := w.repo.SaveRates(runCtx, rates); err != nil {
		return fmt.Errorf("failed to save currency rates: %w", err)
	}

	return nil
}

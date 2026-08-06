package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Krokozabra213/gargantua_common/pkg/clients"
	"github.com/Krokozabra213/gargantua_common/pkg/logger"
	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	currency "github.com/GargantuaLabs/payment/internal/workers/currency_rates"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Init()
	if err != nil {
		return err
	}
	log := logger.Init(&cfg.Logger, nil)

	pool, err := clients.NewPostgresClient(&cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	cbr := currency.NewCBRSource(nil)
	repo := pgRepo.New(pool)

	log.Info("starting currency sync")

	fetchCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	codes := []domain.CurrencyCode{domain.CurrencyUSD, domain.CurrencyEUR}

	currencyWorker := currency.NewWorker(repo, cbr, codes)
	err = currencyWorker.Run(fetchCtx)
	if err != nil {
		return err
	}

	log.Info("currency sync successful")
	return nil
}

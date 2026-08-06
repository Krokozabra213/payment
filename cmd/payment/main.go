package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Krokozabra213/gargantua_common/pkg/clients"
	"github.com/Krokozabra213/gargantua_common/pkg/logger"
	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/GargantuaLabs/payment/internal/kafka/producer"
	"github.com/GargantuaLabs/payment/internal/providers/tbank"
	tg "github.com/GargantuaLabs/payment/internal/providers/telegram"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/internal/server/grpc/app"
	"github.com/GargantuaLabs/payment/internal/server/grpc/handlers"
	svc "github.com/GargantuaLabs/payment/internal/service"
	outboxworkers "github.com/GargantuaLabs/payment/internal/workers/outbox_workers"
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

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	telemetry, err := setupTelemetry(ctx, cfg)
	if err != nil {
		return err
	}
	defer telemetry.Shutdown(ctx)

	log := logger.Init(&cfg.Logger, telemetry.Handler)

	pool, err := clients.NewPostgresClient(&cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	kafkaClient, err := producer.NewSaramaSyncProducer(log, cfg.Producer)
	if err != nil {
		return err
	}
	defer kafkaClient.Close()

	balanceProducer := producer.NewBalanceProducer(kafkaClient, cfg.Producer.PaymentTopic)

	tbankProvider := tbank.NewProvider(&cfg.Tinkoff, log)
	tgProvider := tg.NewProvider(&cfg.Telegram, log)

	repo := pgRepo.New(pool)
	paymentWorker := outboxworkers.NewPaymentWorker(log, repo, balanceProducer, &cfg.Producer)

	go func() {
		if err := paymentWorker.Run(ctx); err != nil {
			log.Error("payment worker failed", "error", err)
			stop()
		}
	}()

	service := svc.New(cfg, log, tbankProvider, tgProvider, repo)
	handler := handlers.New(cfg, service)
	grpcServer := app.New(&cfg.GRPC, log, handler)
	go grpcServer.MustRun()

	<-ctx.Done()

	log.Info("shutting down application")

	grpcServer.Stop()

	log.Info("application stopped")

	return nil
}

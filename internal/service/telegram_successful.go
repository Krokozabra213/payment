package svc

import (
	"context"
	"errors"

	comtel "github.com/Krokozabra213/gargantua_common/pkg/telemetry"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const tgSuccessfulOp = "payment.telegram.successful"

func (s *Service) TGSuccessful(ctx context.Context, sp domain.SuccessfulPayment) error {

	ctx, span := telemetry.ServiceTrace.Start(ctx, tgSuccessfulOp,
		trace.WithAttributes(
			attribute.Int("payment.amount", sp.TotalAmount),
			attribute.String("payment.currency", sp.Currency),
			attribute.String("payment.provider", "telegram"),
			attribute.String("payment.idempotency_key", sp.InvoicePayload),
		),
	)
	defer span.End()

	s.metrics.PaymentInProcess.Add(ctx, 1)
	defer s.metrics.PaymentInProcess.Add(ctx, -1)

	s.log.InfoContext(ctx, "processing telegram successful payment")

	err := s.dbRepo.TGCompletePayment(ctx, domain.TGCompletePaymentParams{
		IdempotencyKey:          sp.InvoicePayload,
		TelegramPaymentChargeID: sp.TelegramPaymentChargeID,
		Amount:                  sp.TotalAmount,
		Currency:                sp.Currency,
	})
	if err != nil {
		if errors.Is(err, pgRepo.ErrAlreadyPaid) {
			span.AddEvent("payment.already_completed")
			s.log.InfoContext(ctx, "telegram payment already completed")
			return nil
		}

		comtel.HandleError(span, "failed completed telegram payment", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "telegram"),
			attribute.String("error.type", comtel.DBAttr),
		))

		return apperror.NewAppErr(apperror.CodeInternal, "repository.TGCompletePayment",
			"failed completed telegram payment", err, apperror.LevelError, nil)
	}

	span.AddEvent("payment.completed")
	s.metrics.PaymentCompleted.Add(ctx, 1, metric.WithAttributes(
		attribute.String("payment.provider", "telegram"),
	))
	s.metrics.PaymentSucceeded.Add(ctx, 1, metric.WithAttributes(
		attribute.String("payment.provider", "telegram"),
	))

	s.log.InfoContext(ctx, "telegram payment completed")

	return nil
}

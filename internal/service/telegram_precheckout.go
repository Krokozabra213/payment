package svc

import (
	"context"
	"errors"

	comtel "github.com/Krokozabra213/gargantua_common/pkg/telemetry"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/GargantuaLabs/payment/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const tgPrecheckoutOp = "payment.telegram.precheckout"

func (s *Service) TGPrecheckout(ctx context.Context, query *domain.PreCheckoutQuery) error {

	ctx, span := telemetry.ServiceTrace.Start(ctx, tgPrecheckoutOp,
		trace.WithAttributes(
			attribute.Int("payment.amount", query.TotalAmount),
			attribute.String("payment.currency", query.Currency),
			attribute.String("payment.provider", "telegram"),
			attribute.String("payment.idempotency_key", query.InvoicePayload),
		),
	)
	defer span.End()

	s.metrics.PaymentInProcess.Add(ctx, 1)
	defer s.metrics.PaymentInProcess.Add(ctx, -1)

	s.log.InfoContext(ctx, "processing telegram precheckout")

	payload := domain.PreCheckoutPayload{
		PreCheckoutQueryID: query.ID,
	}

	err := s.dbRepo.TGPrecheckoutApprove(ctx, domain.TGPrecheckoutApproveParams{
		IdempotencyKey: query.InvoicePayload,
		TGUserID:       query.From.ID,
		Amount:         query.TotalAmount,
		Currency:       query.Currency,
	})
	if err != nil {
		comtel.HandleError(span, "failed precheckout approve process", err)

		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "telegram"),
			attribute.String("error.type", comtel.DBAttr),
		))

		payload.ErrorMsg = "Произошла ошибка, попробуйте чуть позже"
		if ansErr := s.tgProvider.AnswerPreCheckout(ctx, payload); ansErr != nil {
			comtel.HandleError(span, "failed answer telegram precheckout error", ansErr)
			return apperror.NewAppErr(apperror.CodeInternal, "provider.AnswerPreCheckout", "failed answer telegram precheckout error", errors.Join(err, ansErr), apperror.LevelError, nil)
		}
		return apperror.NewAppErr(apperror.CodeInternal, "repository.TGPrecheckoutApprove",
			"failed precheckout approve process", err, apperror.LevelError, nil)
	}

	span.AddEvent("precheckout.approved")

	payload.Ok = true
	if err := s.tgProvider.AnswerPreCheckout(ctx, payload); err != nil {
		comtel.HandleError(span, "failed answer telegram precheckout", err)

		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "telegram"),
			attribute.String("error.type", comtel.ExternalAttr),
		))

		return err
	}

	span.AddEvent("precheckout.answered")
	s.log.InfoContext(ctx, "telegram precheckout approved")

	return nil
}

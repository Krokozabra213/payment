package svc

import (
	"context"
	"errors"
	"fmt"

	comtel "github.com/Krokozabra213/gargantua_common/pkg/telemetry"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/GargantuaLabs/payment/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const tbankWebhookOp = "Service.TbankWebhook"

func (s *Service) TbankWebhook(ctx context.Context, n *domain.TBankPaymentNotification) error {
	ctx, span := telemetry.ServiceTrace.Start(ctx, tbankWebhookOp,
		trace.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("payment.provider_status", n.Status),
			attribute.Bool("payment.success", n.Success),
			attribute.Int("payment.amount", n.Amount),
			attribute.String("payment.currency", "RUB(копейки)"),
		),
	)
	defer span.End()

	s.metrics.PaymentInProcess.Add(ctx, 1)
	defer s.metrics.PaymentInProcess.Add(ctx, -1)

	span.SetAttributes(
		attribute.String("payment.idempotency_key", n.OrderID),
		attribute.String("payment.provider_payment_id", fmt.Sprint(n.PaymentID)),
	)

	s.log.InfoContext(ctx, "processing tbank webhook")

	if !s.tbankProvider.VerifyWebhook(n.ToMap(), n.Token) {
		comtel.HandleError(span, "invalid tbank webhook token", ErrTbankInvalidToken)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("error.type", comtel.ValidationAttr),
		))

		return apperror.NewAppErr(apperror.CodeValidation, tbankWebhookOp,
			ErrTbankInvalidToken.Error(), nil, apperror.LevelError, nil)
	}

	span.AddEvent("webhook.verified")

	providerStatus, err := domain.NewTBankProviderStatus(n.Status)
	if err != nil {
		comtel.HandleError(span, "invalid tbank provider status", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("error.type", comtel.ValidationAttr),
		))

		return apperror.NewAppErr(apperror.CodeValidation, tbankWebhookOp,
			err.Error(), nil, apperror.LevelError, nil)
	}

	newStatus, err := providerStatus.ToPaymentStatus()
	if err != nil {
		span.AddEvent("payment.status.ignored")
		return nil // NEW, FORM_SHOWED, AUTHORIZED — игнорируем
	}

	span.SetAttributes(attribute.String("payment.status", string(newStatus)))

	if err := validateTbankNotification(n.Success, newStatus); err != nil {
		comtel.HandleError(span, "invalid tbank notification state", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("error.type", comtel.ValidationAttr),
		))
		return apperror.NewAppErr(apperror.CodeValidation, tbankWebhookOp, err.Error(), nil, apperror.LevelError, nil)
	}

	switch newStatus {
	case domain.PaymentStatusCompleted:
		if err = s.handleTbankComplete(ctx, n, newStatus); err != nil {
			comtel.HandleError(span, "failed handle tbank complete payment", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", "tbank"),
				attribute.String("error.type", comtel.DBAttr),
			))

			return apperror.NewAppErr(apperror.CodeInternal, "repository.TbankCompletePayment",
				"failed handle tbank complete payment", err, apperror.LevelError, nil)
		}

		span.AddEvent("payment.completed")
		s.metrics.PaymentCompleted.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
		))

	case domain.PaymentStatusRefunded:
		if err = s.handleTbankRefund(ctx, n, newStatus); err != nil {
			comtel.HandleError(span, "failed handle tbank refund payment", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", "tbank"),
				attribute.String("error.type", comtel.DBAttr),
			))

			return apperror.NewAppErr(apperror.CodeInternal, "repository.TbankCompletePayment",
				"failed handle tbank refund payment", err, apperror.LevelError, nil)
		}

		span.AddEvent("payment.refunded")
		s.metrics.PaymentRefunded.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
		))

	case domain.PaymentStatusFailed:
		if err := s.handleTbankTerminal(ctx, n, newStatus); err != nil {
			comtel.HandleError(span, "failed handle tbank terminal payment", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", "tbank"),
				attribute.String("kind", "system"),
				attribute.String("error.type", comtel.DBAttr),
			))

			return apperror.NewAppErr(apperror.CodeInternal, "repository.TbankUpdateStatus", "failed handle tbank terminal payment", err, apperror.LevelError, nil)
		}

		span.AddEvent("payment.failed")
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("kind", "business"),
			attribute.String("payment.status", string(newStatus)),
		))

	case domain.PaymentStatusExpired, domain.PaymentStatusCancelled:
		if err := s.handleTbankTerminal(ctx, n, newStatus); err != nil {
			comtel.HandleError(span, "failed handle tbank terminal payment", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", "tbank"),
				attribute.String("kind", "system"),
				attribute.String("error.type", comtel.DBAttr),
			))

			return apperror.NewAppErr(apperror.CodeInternal, "repository.TbankUpdateStatus", "failed handle tbank terminal payment", err, apperror.LevelError, nil)
		}

		span.AddEvent("payment.cancelled", trace.WithAttributes(
			attribute.String("payment.status", string(newStatus)),
		))
		s.metrics.PaymentCancelled.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("payment.status", string(newStatus)),
		))

	default:
		msg := fmt.Sprintf("unexpected mapped status: %s", string(newStatus))
		comtel.HandleError(span, "unexpected mapped status", errors.New(msg))

		return apperror.NewAppErr(apperror.CodeValidation, tbankWebhookOp, msg, nil, apperror.LevelError, nil)
	}

	s.log.InfoContext(ctx, "tbank webhook processed")
	return nil
}

func (s *Service) handleTbankComplete(ctx context.Context, n *domain.TBankPaymentNotification, newStatus domain.PaymentStatus) error {
	err := s.dbRepo.TbankCompletePayment(ctx, domain.TbankCompleteParams{
		IdempotencyKey:    n.OrderID,
		ProviderPaymentID: n.PaymentID,
		Amount:            n.Amount,
		NewStatus:         domain.PaymentStatusCompleted,
		CurrentStatus:     domain.PaymentStatusPending,
		OpType:            domain.OpTypeDeposit,
	})
	if err != nil {
		if errors.Is(err, pgRepo.ErrAlreadyProcessed) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Service) handleTbankRefund(ctx context.Context, n *domain.TBankPaymentNotification, newStatus domain.PaymentStatus) error {
	err := s.dbRepo.TbankCompletePayment(ctx, domain.TbankCompleteParams{
		IdempotencyKey:    n.OrderID,
		ProviderPaymentID: n.PaymentID,
		Amount:            n.Amount,
		NewStatus:         domain.PaymentStatusRefunded,
		CurrentStatus:     domain.PaymentStatusCompleted,
		OpType:            domain.OpTypeRefund,
	})
	if err != nil {
		if errors.Is(err, pgRepo.ErrAlreadyProcessed) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Service) handleTbankTerminal(ctx context.Context, n *domain.TBankPaymentNotification, newStatus domain.PaymentStatus) error {
	err := s.dbRepo.TbankUpdateStatus(ctx, domain.TbankUpdateStatusParams{
		IdempotencyKey: n.OrderID,
		NewStatus:      newStatus,
		CurrentStatus:  domain.PaymentStatusPending,
	})
	if err != nil {
		if errors.Is(err, pgRepo.ErrInvalidPaymentState) {
			return nil
		}
		return err
	}
	return nil
}

func validateTbankNotification(success bool, newStatus domain.PaymentStatus) error {
	switch newStatus {
	case domain.PaymentStatusCompleted, domain.PaymentStatusRefunded:
		if !success {
			return ErrUnexpectedNotificationState
		}

	case domain.PaymentStatusFailed, domain.PaymentStatusExpired, domain.PaymentStatusCancelled:
		if success {
			return ErrUnexpectedNotificationState
		}
	}

	return nil
}

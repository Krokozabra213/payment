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
	"github.com/Krokozabra213/common/types"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const paymentCancelOp = "payment.cancel"

func (s *Service) PaymentCancel(ctx context.Context, paymentID uuid.UUID) error {
	baseFields := apperror.Fields{
		apperror.F("payment_id", paymentID.String()),
	}

	ctx, span := telemetry.ServiceTrace.Start(ctx, paymentCancelOp,
		trace.WithAttributes(
			attribute.String("payment.id", paymentID.String()),
		),
	)
	defer span.End()

	s.metrics.PaymentInProcess.Add(ctx, 1)
	defer s.metrics.PaymentInProcess.Add(ctx, -1)

	s.log.InfoContext(ctx, "processing payment cancel")

	payment, err := s.dbRepo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		comtel.HandleError(span, "failed select payment by id", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("error.type", comtel.DBAttr),
		))

		return apperror.NewAppErr(apperror.CodeNotFound, "repository.GetPaymentByID",
			"failed select payment by id", err, apperror.LevelError, baseFields)
	}

	span.SetAttributes(
		attribute.String("payment.status", string(payment.Status)),
		attribute.String("payment.provider", payment.ProviderName),
	)

	switch payment.Status {
	case domain.PaymentStatusStarted:
		return s.cancelStarted(ctx, span, payment, baseFields)
	case domain.PaymentStatusPending, domain.PaymentStatusCancelling:
		return s.cancelPending(ctx, span, payment, baseFields)

	case domain.PaymentStatusCompleted, domain.PaymentStatusRefunding:
		return s.refundPaid(ctx, span, payment, baseFields)

	case domain.PaymentStatusCancelled:
		return nil

	case domain.PaymentStatusRefunded:
		return nil

	default:
		return apperror.NewAppErr(apperror.CodeValidation, "Service.PaymentCancel",
			ErrInvalidPaymentStatus.Error(), nil, apperror.LevelError, baseFields)
	}
}

func (s *Service) cancelStarted(ctx context.Context, span trace.Span, payment *domain.Payment, baseFields apperror.Fields) error {
	cancelStartedFields := apperror.Fields{
		apperror.F("current_status", string(domain.PaymentStatusStarted)),
		apperror.F("lock_status", string(domain.PaymentStatusCancelling)),
	}

	baseFields = append(baseFields, cancelStartedFields...)

	_, err := s.dbRepo.PaymentCancelLock(ctx, domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusStarted,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	if err != nil {
		if errors.Is(err, pgRepo.ErrPaymentProcessed) {
			span.AddEvent("payment.already_processed")
			return nil
		}

		comtel.HandleError(span, "failed lock payment", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", payment.ProviderName),
			attribute.String("error.type", comtel.DBAttr),
		))

		return apperror.NewAppErr(apperror.CodeInternal, "repository.PaymentCancelLock",
			"failed lock payment", err, apperror.LevelError, baseFields)
	}

	span.AddEvent("payment.locked")

	if err := s.dbRepo.PaymentFinishCancel(ctx, payment.ID); err != nil {
		comtel.HandleError(span, "failed finish payment", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", payment.ProviderName),
			attribute.String("error.type", comtel.DBAttr),
		))

		return apperror.NewAppErr(apperror.CodeInternal, "repository.PaymentFinishCancel",
			"failed finish payment", err, apperror.LevelError, baseFields)
	}

	span.AddEvent("payment.cancelled")
	s.metrics.PaymentCancelled.Add(ctx, 1, metric.WithAttributes(
		attribute.String("payment.provider", payment.ProviderName),
	))

	return nil
}

func (s *Service) cancelPending(ctx context.Context, span trace.Span, payment *domain.Payment, baseFields apperror.Fields) error {
	cancelPendingFields := apperror.Fields{
		apperror.F("current_status", string(domain.PaymentStatusPending)),
		apperror.F("lock_status", string(domain.PaymentStatusCancelling)),
	}

	baseFields = append(baseFields, cancelPendingFields...)

	payment, err := s.dbRepo.PaymentCancelLock(ctx, domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusPending,
		LockStatus:    domain.PaymentStatusCancelling,
	})
	if err != nil {
		if errors.Is(err, pgRepo.ErrPaymentProcessed) {
			span.AddEvent("payment.already_processed")
			return nil
		}

		comtel.HandleError(span, "failed lock payment", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", payment.ProviderName),
			attribute.String("error.type", comtel.DBAttr),
		))

		return apperror.NewAppErr(apperror.CodeInternal, "repository.PaymentCancelLock",
			"failed lock payment", err, apperror.LevelError, baseFields)
	}

	span.AddEvent("payment.locked")
	baseFields = append(baseFields, apperror.F("provider", payment.ProviderName))

	if payment.ProviderName == string(domain.PaymentTbankForm) {
		providerPaymentID, err := types.NewNonEmptyString(payment.ProviderPaymentID)
		if err != nil {
			comtel.HandleError(span, "failed parse provider_payment_id", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", payment.ProviderName),
				attribute.String("error.type", comtel.ValidationAttr),
			))
			return apperror.NewAppErr(apperror.CodeInternal, "Service.CancelPending",
				"failed parse provider_payment_id", err, apperror.LevelError, baseFields)
		}

		if _, err := s.tbankProvider.Cancel(ctx, providerPaymentID); err != nil {
			comtel.HandleError(span, "failed cancel in tbank provider", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", payment.ProviderName),
				attribute.String("error.type", comtel.ExternalAttr),
			))

			_ = s.dbRepo.PaymentRevertLock(ctx, payment.ID,
				domain.PaymentStatusCancelling, domain.PaymentStatusPending)
			return err
		}

		span.AddEvent("provider.cancelled")
	}

	if err := s.dbRepo.PaymentFinishCancel(ctx, payment.ID); err != nil {
		comtel.HandleError(span, "failed payment finish cancel", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", payment.ProviderName),
			attribute.String("error.type", comtel.DBAttr),
		))

		return apperror.NewAppErr(apperror.CodeInternal, "repository.PaymentFinishCancel",
			"failed payment finish cancel", err, apperror.LevelError, baseFields)
	}

	span.AddEvent("payment.cancelled")
	s.metrics.PaymentCancelled.Add(ctx, 1, metric.WithAttributes(
		attribute.String("payment.provider", payment.ProviderName),
	))

	return nil
}

func (s *Service) refundPaid(ctx context.Context, span trace.Span, payment *domain.Payment, baseFields apperror.Fields) error {
	refundPaidFields := apperror.Fields{
		apperror.F("current_status", string(domain.PaymentStatusCompleted)),
		apperror.F("lock_status", string(domain.PaymentStatusRefunding)),
	}

	baseFields = append(baseFields, refundPaidFields...)

	payment, err := s.dbRepo.PaymentCancelLock(ctx, domain.PaymentCancelLockParams{
		PaymentID:     payment.ID,
		CurrentStatus: domain.PaymentStatusCompleted,
		LockStatus:    domain.PaymentStatusRefunding,
	})
	if err != nil {
		if errors.Is(err, pgRepo.ErrPaymentProcessed) {
			span.AddEvent("payment.already_processed")
			return nil
		}

		comtel.HandleError(span, "failed lock payment", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", payment.ProviderName),
			attribute.String("error.type", comtel.DBAttr),
		))

		return apperror.NewAppErr(apperror.CodeInternal, "repository.PaymentCancelLock",
			"failed lock payment", err, apperror.LevelError, baseFields)
	}

	span.AddEvent("payment.locked")

	providerPaymentID, err := types.NewNonEmptyString(payment.ProviderPaymentID)
	if err != nil {
		comtel.HandleError(span, "failed parse provider_payment_id", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", payment.ProviderName),
			attribute.String("error.type", comtel.ValidationAttr),
		))

		_ = s.dbRepo.PaymentRevertLock(ctx, payment.ID,
			domain.PaymentStatusRefunding, domain.PaymentStatusCompleted)
		return apperror.NewAppErr(apperror.CodeInternal, "Service.CancelPending",
			"failed parse provider_payment_id", err, apperror.LevelError, baseFields)
	}

	switch payment.ProviderName {
	case string(domain.PaymentTbankForm):
		if _, err := s.tbankProvider.Cancel(ctx, providerPaymentID); err != nil {
			comtel.HandleError(span, "failed refund in tbank provider", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", payment.ProviderName),
				attribute.String("error.type", comtel.ExternalAttr),
			))

			_ = s.dbRepo.PaymentRevertLock(ctx, payment.ID,
				domain.PaymentStatusRefunding, domain.PaymentStatusCompleted)
			return err
		}
		span.AddEvent("provider.refunded")

	case string(domain.PaymentTGStars):
		providerUserID, err := types.NewPositiveInt(payment.ProviderUserID)
		if err != nil {
			comtel.HandleError(span, "failed parse provider_user_id", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", payment.ProviderName),
				attribute.String("error.type", comtel.ValidationAttr),
			))

			_ = s.dbRepo.PaymentRevertLock(ctx, payment.ID,
				domain.PaymentStatusRefunding, domain.PaymentStatusCompleted)
			return apperror.NewAppErr(apperror.CodeInternal, "Service.CancelPending",
				"failed parse provider_user_id", err, apperror.LevelError, baseFields)
		}

		if err := s.tgProvider.Cancel(ctx, providerUserID, providerPaymentID); err != nil {
			comtel.HandleError(span, "failed refund in telegram provider", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", payment.ProviderName),
				attribute.String("error.type", comtel.ExternalAttr),
			))

			_ = s.dbRepo.PaymentRevertLock(ctx, payment.ID,
				domain.PaymentStatusRefunding, domain.PaymentStatusCompleted)
			return err
		}
		span.AddEvent("provider.refunded")

	default:
		comtel.HandleError(span, "unsupported provider", fmt.Errorf("unsupported provider: %s", payment.ProviderName))
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", payment.ProviderName),
			attribute.String("error.type", comtel.ValidationAttr),
		))

		_ = s.dbRepo.PaymentRevertLock(ctx, payment.ID,
			domain.PaymentStatusRefunding, domain.PaymentStatusCompleted)
		return fmt.Errorf("unsupported provider: %s", payment.ProviderName)
	}

	if err := s.dbRepo.PaymentFinishRefund(ctx, payment.ID, payment.Amount); err != nil {
		comtel.HandleError(span, "failed finish refund", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", payment.ProviderName),
			attribute.String("error.type", comtel.DBAttr),
		))

		return fmt.Errorf("finish refund: %w", err)
	}

	span.AddEvent("payment.refunded")
	s.metrics.PaymentCancelled.Add(ctx, 1, metric.WithAttributes(
		attribute.String("payment.provider", payment.ProviderName),
		attribute.String("payment.status", "refunded"),
	))

	return nil
}

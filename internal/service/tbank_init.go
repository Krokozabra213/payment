package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	comtel "github.com/Krokozabra213/gargantua_common/pkg/telemetry"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/GargantuaLabs/payment/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	tbankOpInit = "payment.tbank.init"
)

func (s *Service) TbankInit(
	ctx context.Context,
	input *domain.TbankInit,
	idempotencyKey string,
) (*domain.TbankInitResult, error) {
	ctx, span := telemetry.ServiceTrace.Start(ctx, tbankOpInit,
		trace.WithAttributes(
			attribute.Int("payment.amount", input.Amount.Value()),
			attribute.String("payment.currency", "RUB(копейки)"),
			attribute.String("payment.provider", "tbank"),
		),
	)
	defer span.End()

	s.metrics.PaymentInProcess.Add(ctx, 1)
	defer s.metrics.PaymentInProcess.Add(ctx, -1)

	s.log.InfoContext(ctx, "creating tbank payment")

	payment, inserted, err := s.dbRepo.PaymentReserve(ctx, domain.CreatePaymentParams{
		UserID:         input.UserID,
		IdempotencyKey: idempotencyKey,
		Amount:         input.Amount.Value(),
		Currency:       string(domain.CurrencyTypeRUB),
		ProviderName:   string(domain.PaymentTbankForm),
		Description:    &input.Desc,
	})
	if err != nil {
		comtel.HandleError(span, "failed reserve payment", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("error.type", comtel.DBAttr),
		))
		return nil, apperror.NewAppErr(apperror.CodeInternal, "repository.PaymentReserve",
			"failed reserve payment", err, apperror.LevelError, nil)
	}

	span.SetAttributes(attribute.String("payment.id", payment.ID.String()))
	span.AddEvent("payment.reserved")

	if !inserted {
		if err := s.validateExistingTbankPayment(input, payment); err != nil {
			comtel.HandleError(span, "failed validate existing tbank payment", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", "tbank"),
				attribute.String("error.type", comtel.ValidationAttr),
			))
			return nil, apperror.NewAppErr(apperror.CodeConflict, "Service.TbankInit",
				"failed validate existing tbank payment", err, apperror.LevelError, nil)
		}

		result, err := s.handleExistingTbankPayment(ctx, input, payment, idempotencyKey, nil)
		if err != nil {
			comtel.HandleError(span, "failed handle existing tbank payment", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", "tbank"),
				attribute.String("error.type", comtel.ValidationAttr),
			))
			return nil, err
		}

		span.AddEvent("payment.reused")
		s.log.InfoContext(ctx, "tbank payment reused",
			slog.String("payment_id", payment.ID.String()),
		)
		return result, nil
	}

	result, err := s.tbankProvider.Init(ctx, input, idempotencyKey)
	if err != nil {
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("error.type", comtel.ExternalAttr),
		))

		if dbErr := s.dbRepo.MarkTGPaymentInitFailed(ctx, payment.ID); dbErr != nil {
			comtel.HandleError(span, "failed mark tbank payment init failed", dbErr)
			return nil, apperror.NewAppErr(apperror.CodeInternal, "repository.MarkTGPaymentInitFailed",
				"failed mark payment init failed", errors.Join(err, dbErr), apperror.LevelError, nil)
		}
		comtel.HandleError(span, "failed init payment in tbank provider", err)
		return nil, err
	}

	span.AddEvent("provider.initialized")

	if err := s.dbRepo.MarkTbankPaymentPending(ctx, payment.ID, result.PaymentURL.Value(), result.PaymentID.Value()); err != nil {
		comtel.HandleError(span, "failed mark tbank payment pending", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "tbank"),
			attribute.String("error.type", comtel.DBAttr),
		))
		return nil, apperror.NewAppErr(apperror.CodeInternal, "Service.TbankInit",
			"failed mark payment pending", err, apperror.LevelError, nil)
	}

	span.AddEvent("payment.pending")
	s.log.InfoContext(ctx, "tbank payment created",
		slog.String("payment_id", payment.ID.String()),
	)
	s.metrics.PaymentCreated.Add(ctx, 1, metric.WithAttributes(
		attribute.String("payment.provider", "tbank"),
	))

	return result, nil
}

func (s *Service) validateExistingTbankPayment(
	input *domain.TbankInit,
	existing *domain.Payment,
) error {
	if existing.ProviderName != string(domain.PaymentTbankForm) {
		return ErrProviderMismatch
	}

	if existing.UserID != input.UserID {
		return ErrIdempotencyConflict
	}

	if existing.Amount != input.Amount.Value() {
		return ErrIdempotencyConflict
	}

	if existing.Currency != string(domain.CurrencyTypeRUB) {
		return ErrIdempotencyConflict
	}

	if existing.Description != input.Desc {
		return ErrIdempotencyConflict
	}

	return nil
}

func (s *Service) handleExistingTbankPayment(
	ctx context.Context,
	input *domain.TbankInit,
	existing *domain.Payment,
	idempotencyKey string,
	baseFields apperror.Fields,
) (*domain.TbankInitResult, error) {
	switch existing.Status {
	case domain.PaymentStatusPending:
		if existing.PaymentURL == "" || existing.ProviderPaymentID == "" {
			return nil, apperror.NewAppErr(apperror.CodeInternal, "Service.TbankInit",
				"pending payment has empty url or provider_payment_id", nil, apperror.LevelError, baseFields)
		}

		initResult, err := domain.NewTbankInitResult(existing.PaymentURL, existing.ProviderPaymentID)
		if err != nil {
			return nil, apperror.NewAppErr(apperror.CodeInternal, "Service.TbankInit",
				"parse existing payment", err, apperror.LevelError, baseFields)
		}

		return &initResult, nil

	case domain.PaymentStatusStarted:
		age := time.Since(existing.UpdatedAt)
		if age < 30*time.Second {
			return nil, apperror.NewAppErr(apperror.CodeConflict, "Service.TbankInit",
				"payment is initializing", nil, apperror.LevelError, baseFields)
		}

		result, err := s.tbankProvider.Init(ctx, input, idempotencyKey)
		if err != nil {
			return nil, err
		}

		if err := s.dbRepo.MarkTbankPaymentPending(ctx, existing.ID, result.PaymentURL.Value(), result.PaymentID.Value()); err != nil {
			return nil, apperror.NewAppErr(apperror.CodeInternal, "Service.TbankInit",
				"failed mark payment pending after recovery", err, apperror.LevelError, baseFields)
		}

		return result, nil

	default:
		msg := fmt.Sprintf("unexpected existing payment status: %s", string(existing.Status))
		return nil, apperror.NewAppErr(apperror.CodeInternal, "Service.TbankInit",
			msg, nil, apperror.LevelError, baseFields)
	}
}

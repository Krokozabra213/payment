package svc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	comtel "github.com/Krokozabra213/gargantua_common/pkg/telemetry"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/GargantuaLabs/payment/internal/telemetry"
	"github.com/Krokozabra213/common/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	opInit = "payment.telegram.init"
)

func (s *Service) TelegramInit(
	ctx context.Context,
	input *domain.TGInit,
	idempotencyKey string,
) (*domain.TGInitResult, error) {
	ctx, span := telemetry.ServiceTrace.Start(ctx, opInit,
		trace.WithAttributes(
			attribute.Int("payment.amount", input.Amount.Value()),
			attribute.String("payment.currency", string(input.Currency)),
			attribute.String("payment.provider", "telegram"),
		),
	)
	defer span.End()

	s.metrics.PaymentInProcess.Add(ctx, 1)
	defer s.metrics.PaymentInProcess.Add(ctx, -1)

	s.log.InfoContext(ctx, "creating telegram payment")

	desc := input.Desc.Value()
	payment, inserted, err := s.dbRepo.PaymentReserve(ctx, domain.CreatePaymentParams{
		UserID:         input.UserID,
		IdempotencyKey: idempotencyKey,
		Amount:         input.Amount.Value(),
		Currency:       string(input.Currency),
		ProviderName:   string(domain.PaymentTGStars),
		Description:    &desc,
	})
	if err != nil {
		comtel.HandleError(span, "failed reserve payment", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "telegram"),
			attribute.String("error.type", comtel.DBAttr),
		))
		return nil, apperror.NewAppErr(apperror.CodeInternal, "repository.PaymentReserve",
			"failed reserve payment", err, apperror.LevelError, nil)
	}

	span.SetAttributes(attribute.String("payment.id", payment.ID.String()))

	span.AddEvent("payment.reserved")

	if !inserted {
		if err := s.validateExistingTGPayment(input, payment); err != nil {
			comtel.HandleError(span, "failed validate existing tg payment", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", "telegram"),
				attribute.String("error.type", comtel.ValidationAttr),
			))
			return nil, apperror.NewAppErr(apperror.CodeConflict, opInit,
				"failed validate existing tg payment", err, apperror.LevelError, nil)
		}

		result, err := s.handleExistingTGPayment(ctx, input, payment, idempotencyKey, nil)
		if err != nil {
			comtel.HandleError(span, "failed handle existing tg payment", err)
			s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
				attribute.String("payment.provider", "telegram"),
				attribute.String("error.type", comtel.ValidationAttr),
			))
			return nil, err
		}

		span.AddEvent("payment.reused")
		s.log.InfoContext(ctx, "telegram payment reused",
			slog.String("payment_id", payment.ID.String()),
		)
		return result, nil
	}

	result, err := s.tgProvider.Init(ctx, input, idempotencyKey)
	if err != nil {
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "telegram"),
			attribute.String("error.type", comtel.ExternalAttr),
		))

		if dbErr := s.dbRepo.MarkTGPaymentInitFailed(ctx, payment.ID); dbErr != nil {
			comtel.HandleError(span, "failed mark tg payment init failed", dbErr)
			return nil, apperror.NewAppErr(apperror.CodeInternal, "repository.MarkTGPaymentInitFailed",
				"failed mark tg payment init failed", dbErr, apperror.LevelError, nil)
		}

		comtel.HandleError(span, "failed init payment in tg provider", err)

		return nil, err
	}

	if err := s.dbRepo.MarkTGPaymentPending(ctx, payment.ID, result.PaymentURL.Value()); err != nil {
		comtel.HandleError(span, "failed mark tg payment pending", err)
		s.metrics.PaymentFailed.Add(ctx, 1, metric.WithAttributes(
			attribute.String("payment.provider", "telegram"),
			attribute.String("error.type", comtel.DBAttr),
		))
		return nil, apperror.NewAppErr(apperror.CodeInternal, opInit,
			"failed mark payment pending", err, apperror.LevelError, nil)
	}

	span.AddEvent("payment.pending")
	s.log.InfoContext(ctx, "telegram payment created",
		slog.String("payment_id", payment.ID.String()),
	)
	s.metrics.PaymentCreated.Add(ctx, 1, metric.WithAttributes(
		attribute.String("payment.provider", "telegram"),
	))

	return result, nil
}

func (s *Service) validateExistingTGPayment(
	input *domain.TGInit,
	existing *domain.Payment,
) error {
	if existing.ProviderName != string(domain.PaymentTGStars) {
		return ErrProviderMismatch
	}

	if existing.UserID != input.UserID {
		return ErrIdempotencyConflict
	}

	if existing.Amount != input.Amount.Value() {
		return ErrIdempotencyConflict
	}

	if existing.Currency != string(input.Currency) {
		return ErrIdempotencyConflict
	}

	if existing.Description != input.Desc.Value() {
		return ErrIdempotencyConflict
	}

	return nil
}

func (s *Service) handleExistingTGPayment(ctx context.Context, input *domain.TGInit,
	existing *domain.Payment, idempotencyKey string, baseFields apperror.Fields,
) (*domain.TGInitResult, error) {
	switch existing.Status {
	case domain.PaymentStatusPending:
		if existing.PaymentURL == "" {
			return nil, apperror.NewAppErr(apperror.CodeInternal, opInit,
				"pending payment has empty payment_url", nil, apperror.LevelError, baseFields)
		}

		url, err := types.NewURL(existing.PaymentURL)
		if err != nil {
			return nil, apperror.NewAppErr(apperror.CodeInternal, opInit,
				"parse existing payment url", err, apperror.LevelError, baseFields)
		}

		return &domain.TGInitResult{
			PaymentURL: url,
		}, nil

	case domain.PaymentStatusStarted:
		age := time.Since(existing.UpdatedAt)
		if age < 30*time.Second {
			return nil, apperror.NewAppErr(apperror.CodeConflict, opInit,
				"payment is initializing", nil, apperror.LevelError, baseFields)
		}
		result, err := s.tgProvider.Init(ctx, input, idempotencyKey)
		if err != nil {
			return nil, err
		}
		err = s.dbRepo.MarkTGPaymentPending(ctx, existing.ID, result.PaymentURL.Value())
		if err != nil {
			return nil, apperror.NewAppErr(apperror.CodeInternal, "repository.MarkTGPaymentPending",
				"failed mark tg payment pending", err, apperror.LevelError, baseFields)
		}
		return result, nil

	default:
		msg := fmt.Sprintf("unexpected existing payment status: %s", string(existing.Status))
		return nil, apperror.NewAppErr(apperror.CodeInternal, opInit,
			msg, nil, apperror.LevelError, baseFields)
	}
}

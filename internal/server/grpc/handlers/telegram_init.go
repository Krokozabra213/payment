package handlers

import (
	"context"

	paymentv1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	"go.opentelemetry.io/otel/trace"
)

const OpTelegramInit = "handler.TGInit"

func (s *PaymentAPI) TGInit(ctx context.Context, r *paymentv1.TGInitRequest) (*paymentv1.TGInitResponse, error) {

	span := trace.SpanFromContext(ctx)

	baseFields := apperror.Fields{
		apperror.F("user_id", r.GetUserId()),
		apperror.F("amount", r.GetAmount()),
		apperror.F("idempotency_key", r.GetIdempotencyKey()),
		apperror.F("provider", "telegram"),
	}

	payload, err := domain.NewTGInitPayload(r.GetUserId(), int(r.GetAmount()), r.GetCurrency(), r.GetDescription(), r.GetTitle())
	if err != nil {
		span.RecordError(err)
		return nil, apperror.NewAppErr(apperror.CodeValidation, OpTelegramInit,
			"failed parse tgInitRequest", err, apperror.LevelError, baseFields)
	}

	result, err := s.service.TelegramInit(ctx, payload, r.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}

	return &paymentv1.TGInitResponse{
		PaymentUrl: result.PaymentURL.Value(),
	}, nil
}

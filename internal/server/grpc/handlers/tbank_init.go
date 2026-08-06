package handlers

import (
	"context"

	paymentv1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	"go.opentelemetry.io/otel/trace"
)

const OpTbankInit = "handler.TbankInit"

func (h *PaymentAPI) TbankInit(ctx context.Context, r *paymentv1.TbankInitRequest) (*paymentv1.TbankInitResponse, error) {
	span := trace.SpanFromContext(ctx)

	cops := r.GetAmount() * 100
	baseFields := apperror.Fields{
		apperror.F("user_id", r.GetUserId()),
		apperror.F("amount", cops),
		apperror.F("idempotency_key", r.GetIdempotencyKey()),
		apperror.F("provider", "tbank"),
	}

	input, err := domain.NewTbankInit(r.GetUserId(), int(cops), r.GetDescription(), h.cfg.Tinkoff.NotificationURL)
	if err != nil {
		span.RecordError(err)
		return nil, apperror.NewAppErr(apperror.CodeValidation, OpTbankInit,
			"failed validation tbankInitRequest", err, apperror.LevelError, baseFields)
	}

	result, err := h.service.TbankInit(ctx, input, r.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}

	return &paymentv1.TbankInitResponse{
		PaymentUrl: result.PaymentURL.Value(),
		PaymentId:  result.PaymentID.Value(),
	}, nil
}

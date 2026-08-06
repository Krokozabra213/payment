package handlers

import (
	"context"

	paymentv1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

const OpCancel = "handler.PaymentCancel"

func (s *PaymentAPI) PaymentCancel(ctx context.Context, r *paymentv1.PaymentCancelRequest) (*paymentv1.PaymentCancelResponse, error) {
	span := trace.SpanFromContext(ctx)

	baseFields := apperror.Fields{
		apperror.F("payment_id", r.GetPaymentId()),
	}

	paymentID, err := uuid.Parse(r.GetPaymentId())
	if err != nil {
		span.RecordError(err)
		return nil, apperror.NewAppErr(apperror.CodeValidation, OpCancel,
			"failed parse paymentID", err, apperror.LevelError, baseFields)
	}

	err = s.service.PaymentCancel(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	return &paymentv1.PaymentCancelResponse{}, nil
}

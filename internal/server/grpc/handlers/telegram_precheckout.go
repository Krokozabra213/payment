package handlers

import (
	"context"
	"errors"

	paymentv1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1"
	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	"go.opentelemetry.io/otel/trace"
)

const OpTbankPreCheckout = "handler.TGPreCheckout"

func (s *PaymentAPI) TGPreCheckout(ctx context.Context, r *paymentv1.TGPreCheckoutRequest) (*paymentv1.TGPreCheckoutResponse, error) {
	span := trace.SpanFromContext(ctx)

	baseFields := apperror.Fields{
		apperror.F("idempotency_key", r.GetInvoicePayload()),
	}

	from := r.GetFrom()
	if from == nil {
		span.RecordError(errors.New("field: from is empty"))
		return nil, apperror.NewAppErr(apperror.CodeValidation, OpTbankPreCheckout,
			"from is required", nil, apperror.LevelError, baseFields)
	}

	user := domain.NewUser(r.From.GetId(), r.From.GetIsBot(), r.From.GetFirstName())
	query, err := domain.NewPreCheckoutQuery(r.GetId(), r.GetInvoicePayload(), int(r.GetTotalAmount()), r.GetCurrency(), user)
	if err != nil {
		span.RecordError(err)
		return nil, apperror.NewAppErr(apperror.CodeValidation, OpTbankPreCheckout,
			"failed parse tgPrecheckoutRequest", err, apperror.LevelError, baseFields)
	}

	err = s.service.TGPrecheckout(ctx, &query)
	if err != nil {
		return nil, err
	}

	return &paymentv1.TGPreCheckoutResponse{}, nil
}

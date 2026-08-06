package handlers

import (
	"context"

	paymentv1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1"
	"github.com/GargantuaLabs/payment/internal/domain"
)

func (s *PaymentAPI) TGSuccessful(ctx context.Context, r *paymentv1.TGSuccessfulRequest) (*paymentv1.TGSuccessfulResponse, error) {
	sp := domain.NewSuccessfulPayment(r.GetCurrency(), r.GetInvoicePayload(), r.GetTelegramPaymentChargeId(),
		r.GetProviderPaymentChargeId(), int(r.GetTotalAmount()))

	err := s.service.TGSuccessful(ctx, sp)
	if err != nil {
		return nil, err
	}

	return &paymentv1.TGSuccessfulResponse{}, nil
}

package handlers

import (
	"context"

	paymentv1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1"
	"github.com/GargantuaLabs/payment/internal/domain"
)

const OpTbankWebhook = "handler.TbankWebhook"

func (s *PaymentAPI) TbankWebhook(ctx context.Context, r *paymentv1.TbankWebhookRequest) (*paymentv1.TbankWebhookResponse, error) {

	notification := domain.NewTBankPaymentNotification(r.TerminalKey, r.OrderId, r.Status, r.ErrorCode,
		r.Token, r.PaymentId, r.Amount, r.Success, r.Message, r.Details, r.Pan, r.ExpDate, r.RebillId, r.CardId)

	err := s.service.TbankWebhook(ctx, notification)
	if err != nil {
		return nil, err
	}

	return &paymentv1.TbankWebhookResponse{}, nil
}

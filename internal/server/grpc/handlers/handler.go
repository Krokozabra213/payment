package handlers

import (
	"context"

	paymentv1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1"
	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
)

type Service interface {
	PaymentCancel(ctx context.Context, paymentID uuid.UUID) error

	TbankInit(
		ctx context.Context,
		input *domain.TbankInit,
		idempotencyKey string,
	) (*domain.TbankInitResult, error)

	TbankWebhook(ctx context.Context, n *domain.TBankPaymentNotification) error

	TelegramInit(
		ctx context.Context,
		input *domain.TGInit,
		idempotencyKey string,
	) (*domain.TGInitResult, error)

	TGPrecheckout(ctx context.Context, query *domain.PreCheckoutQuery) error

	TGSuccessful(ctx context.Context, sp domain.SuccessfulPayment) error
}

type PaymentAPI struct {
	paymentv1.UnimplementedPaymentService_APIServer
	cfg     *config.Config
	service Service
}

func New(cfg *config.Config, svc Service) *PaymentAPI {
	return &PaymentAPI{
		cfg:     cfg,
		service: svc,
	}
}

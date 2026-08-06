package svc

import (
	"context"
	"log/slog"

	"github.com/GargantuaLabs/payment/internal/config"
	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/GargantuaLabs/payment/internal/telemetry"
	"github.com/Krokozabra213/common/types"
	"github.com/google/uuid"
)

const componentName = "internal/service"

//go:generate mockgen -source=service.go -destination=mocks/mock_service.go -package=mocks

type DBProvider interface {
	MarkTbankPaymentPending(ctx context.Context, paymentID uuid.UUID, paymentURL string, providerPaymentID string) error
	TGPrecheckoutApprove(ctx context.Context, params domain.TGPrecheckoutApproveParams) error
	PaymentReserve(ctx context.Context, params domain.CreatePaymentParams) (*domain.Payment, bool, error)
	MarkTGPaymentPending(ctx context.Context, paymentID uuid.UUID, paymentURL string) error
	MarkTGPaymentInitFailed(ctx context.Context, paymentID uuid.UUID) error
	TGCompletePayment(ctx context.Context, params domain.TGCompletePaymentParams) error
	TbankCompletePayment(ctx context.Context, params domain.TbankCompleteParams) error
	TbankUpdateStatus(ctx context.Context, params domain.TbankUpdateStatusParams) error
	GetPaymentByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	PaymentCancelLock(ctx context.Context, params domain.PaymentCancelLockParams) (*domain.Payment, error)
	PaymentFinishCancel(ctx context.Context, paymentID uuid.UUID) error
	PaymentRevertLock(ctx context.Context, paymentID uuid.UUID,
		lockStatus domain.PaymentStatus, revertStatus domain.PaymentStatus) error
	PaymentFinishRefund(ctx context.Context, paymentID uuid.UUID, amount int) error
	GetLatestRate(ctx context.Context, code domain.CurrencyCode) (int64, error)
}

type TGProvider interface {
	AnswerPreCheckout(ctx context.Context, preCheckout domain.PreCheckoutPayload) error
	Cancel(ctx context.Context, userID types.PositiveInt[int64], tgPaymentChargeID types.NonEmptyString) error
	Init(ctx context.Context, payload *domain.TGInit, idempotencyKey string) (*domain.TGInitResult, error)
}

type TbankProvider interface {
	VerifyWebhook(payload map[string]string, token string) bool
	Cancel(ctx context.Context, paymentID types.NonEmptyString) (*domain.TbankCancel, error)
	Init(ctx context.Context, payload *domain.TbankInit, idempotencyKey string) (*domain.TbankInitResult, error)
}

type Service struct {
	cfg           *config.Config
	log           *slog.Logger
	tbankProvider TbankProvider
	tgProvider    TGProvider
	dbRepo        DBProvider
	metrics       *telemetry.PaymentMetrics
}

func New(
	cfg *config.Config,
	log *slog.Logger,
	tbankProvider TbankProvider,
	tgProvider TGProvider,
	dbRepo DBProvider,
) *Service {
	return &Service{
		cfg:           cfg,
		log:           log.With(slog.String("component", componentName)),
		tbankProvider: tbankProvider,
		tgProvider:    tgProvider,
		dbRepo:        dbRepo,
		metrics:       telemetry.NewPaymentMetrics(telemetry.ServiceMeter),
	}
}

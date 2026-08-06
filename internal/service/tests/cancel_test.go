package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/Krokozabra213/common/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentCancel(t *testing.T) {
	paymentID := uuid.New()

	makePayment := func(mods ...func(*domain.Payment)) *domain.Payment {
		p := &domain.Payment{
			ID:                paymentID,
			UserID:            uuid.New(),
			IdempotencyKey:    "cancel-key-123",
			Amount:            100,
			Currency:          "RUB",
			ProviderName:      string(domain.PaymentTbankForm),
			Status:            domain.PaymentStatusPending,
			ProviderPaymentID: "tbank-pid-123",
			UpdatedAt:         time.Now(),
		}
		for _, mod := range mods {
			mod(p)
		}
		return p
	}

	tests := []struct {
		name       string
		setupMocks func(d *testServiceDeps)
		wantErr    bool
	}{
		// ==================== cancelStarted ====================
		{
			name: "cancel started - success",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusStarted
					}), nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), domain.PaymentCancelLockParams{
						PaymentID:     paymentID,
						CurrentStatus: domain.PaymentStatusStarted,
						LockStatus:    domain.PaymentStatusCancelling,
					}).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusCancelling
					}), nil)

				d.dbRepo.EXPECT().
					PaymentFinishCancel(gomock.Any(), paymentID).
					Return(nil)
			},
			wantErr: false,
		},

		// ==================== cancelPending ====================
		{
			name: "cancel pending tbank - success",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment()

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), domain.PaymentCancelLockParams{
						PaymentID:     paymentID,
						CurrentStatus: domain.PaymentStatusPending,
						LockStatus:    domain.PaymentStatusCancelling,
					}).
					Return(payment, nil)

				providerPID, _ := types.NewNonEmptyString("tbank-pid-123")
				d.tbank.EXPECT().
					Cancel(gomock.Any(), providerPID).
					Return(nil, nil)

				d.dbRepo.EXPECT().
					PaymentFinishCancel(gomock.Any(), paymentID).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "cancel pending tg stars - success (no provider call)",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.ProviderName = string(domain.PaymentTGStars)
					p.ProviderPaymentID = ""
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), domain.PaymentCancelLockParams{
						PaymentID:     paymentID,
						CurrentStatus: domain.PaymentStatusPending,
						LockStatus:    domain.PaymentStatusCancelling,
					}).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentFinishCancel(gomock.Any(), paymentID).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "cancel pending tbank - provider fails - reverts lock",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment()

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(payment, nil)

				providerPID, _ := types.NewNonEmptyString("tbank-pid-123")
				d.tbank.EXPECT().
					Cancel(gomock.Any(), providerPID).
					Return(nil, fmt.Errorf("tbank api error"))

				d.dbRepo.EXPECT().
					PaymentRevertLock(gomock.Any(), paymentID,
						domain.PaymentStatusCancelling, domain.PaymentStatusPending).
					Return(nil)
			},
			wantErr: true,
		},
		{
			name: "cancel pending - lock fails - already processing",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(makePayment(), nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(nil, pgRepo.ErrPaymentProcessed)
			},
			wantErr: false, // другой процесс уже отменяет — не ошибка
		},
		{
			name: "cancel pending - finish cancel fails",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.ProviderName = string(domain.PaymentTGStars)
					p.ProviderPaymentID = ""
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentFinishCancel(gomock.Any(), paymentID).
					Return(fmt.Errorf("db error"))
			},
			wantErr: true,
		},

		// ==================== refundPaid ====================
		{
			name: "refund paid tbank - success",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.Status = domain.PaymentStatusCompleted
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), domain.PaymentCancelLockParams{
						PaymentID:     paymentID,
						CurrentStatus: domain.PaymentStatusCompleted,
						LockStatus:    domain.PaymentStatusRefunding,
					}).
					Return(payment, nil)

				providerPID, _ := types.NewNonEmptyString("tbank-pid-123")
				d.tbank.EXPECT().
					Cancel(gomock.Any(), providerPID).
					Return(nil, nil)

				d.dbRepo.EXPECT().
					PaymentFinishRefund(gomock.Any(), paymentID, 100).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "refund paid tg stars - success",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.Status = domain.PaymentStatusCompleted
					p.ProviderName = string(domain.PaymentTGStars)
					p.ProviderPaymentID = "tg-charge-123"
					p.ProviderUserID = 123456
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), domain.PaymentCancelLockParams{
						PaymentID:     paymentID,
						CurrentStatus: domain.PaymentStatusCompleted,
						LockStatus:    domain.PaymentStatusRefunding,
					}).
					Return(payment, nil)

				providerUserID, _ := types.NewPositiveInt[int64](123456)
				providerPID, _ := types.NewNonEmptyString("tg-charge-123")
				d.tg.EXPECT().
					Cancel(gomock.Any(), providerUserID, providerPID).
					Return(nil)

				d.dbRepo.EXPECT().
					PaymentFinishRefund(gomock.Any(), paymentID, 100).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "refund paid tbank - provider fails - reverts lock",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.Status = domain.PaymentStatusCompleted
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(payment, nil)

				providerPID, _ := types.NewNonEmptyString("tbank-pid-123")
				d.tbank.EXPECT().
					Cancel(gomock.Any(), providerPID).
					Return(nil, fmt.Errorf("tbank api error"))

				d.dbRepo.EXPECT().
					PaymentRevertLock(gomock.Any(), paymentID,
						domain.PaymentStatusRefunding, domain.PaymentStatusCompleted).
					Return(nil)
			},
			wantErr: true,
		},
		{
			name: "refund paid tg - provider fails - reverts lock",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.Status = domain.PaymentStatusCompleted
					p.ProviderName = string(domain.PaymentTGStars)
					p.ProviderPaymentID = "tg-charge-123"
					p.ProviderUserID = 123456
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(payment, nil)

				providerUserID, _ := types.NewPositiveInt[int64](123456)
				providerPID, _ := types.NewNonEmptyString("tg-charge-123")
				d.tg.EXPECT().
					Cancel(gomock.Any(), providerUserID, providerPID).
					Return(fmt.Errorf("telegram api error"))

				d.dbRepo.EXPECT().
					PaymentRevertLock(gomock.Any(), paymentID,
						domain.PaymentStatusRefunding, domain.PaymentStatusCompleted).
					Return(nil)
			},
			wantErr: true,
		},
		{
			name: "refund paid - empty provider_payment_id - reverts lock",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.Status = domain.PaymentStatusCompleted
					p.ProviderPaymentID = ""
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentRevertLock(gomock.Any(), paymentID,
						domain.PaymentStatusRefunding, domain.PaymentStatusCompleted).
					Return(nil)
			},
			wantErr: true,
		},
		{
			name: "refund paid - lock fails - already processing",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusCompleted
					}), nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(nil, pgRepo.ErrPaymentProcessed)
			},
			wantErr: false,
		},
		{
			name: "refund paid - finish refund fails",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.Status = domain.PaymentStatusCompleted
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(payment, nil)

				providerPID, _ := types.NewNonEmptyString("tbank-pid-123")
				d.tbank.EXPECT().
					Cancel(gomock.Any(), providerPID).
					Return(nil, nil)

				d.dbRepo.EXPECT().
					PaymentFinishRefund(gomock.Any(), paymentID, 100).
					Return(fmt.Errorf("db error"))
			},
			wantErr: true,
		},
		{
			name: "refund paid - unsupported provider - reverts lock",
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment(func(p *domain.Payment) {
					p.Status = domain.PaymentStatusCompleted
					p.ProviderName = "unknown_provider"
				})

				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentCancelLock(gomock.Any(), gomock.Any()).
					Return(payment, nil)

				d.dbRepo.EXPECT().
					PaymentRevertLock(gomock.Any(), paymentID,
						domain.PaymentStatusRefunding, domain.PaymentStatusCompleted).
					Return(nil)
			},
			wantErr: true,
		},

		// ==================== idempotent cases ====================
		{
			name: "already cancelled - returns nil",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusCancelled
					}), nil)
			},
			wantErr: false,
		},
		{
			name: "already refunded - returns nil",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusRefunded
					}), nil)
			},
			wantErr: false,
		},

		// ==================== invalid statuses ====================
		{
			name: "processing status - returns error",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusProcessed
					}), nil)
			},
			wantErr: true,
		},
		{
			name: "failed status - returns error",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusFailed
					}), nil)
			},
			wantErr: true,
		},
		{
			name: "expired status - returns error",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusExpired
					}), nil)
			},
			wantErr: true,
		},

		// ==================== get payment errors ====================
		{
			name: "payment not found",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(nil, pgRepo.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "db error on get payment",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					GetPaymentByID(gomock.Any(), paymentID).
					Return(nil, fmt.Errorf("connection refused"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestService(t)
			tt.setupMocks(deps)

			err := svc.PaymentCancel(context.Background(), paymentID)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

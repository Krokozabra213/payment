package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTbankWebhook(t *testing.T) {
	makeNotification := func(mods ...func(*domain.TBankPaymentNotification)) *domain.TBankPaymentNotification {
		n := &domain.TBankPaymentNotification{
			OrderID:   "order-123",
			PaymentID: "tbank-pid-456",
			Amount:    10000,
			Status:    "CONFIRMED",
			Success:   true,
			Token:     "valid-token",
		}
		for _, mod := range mods {
			mod(n)
		}
		return n
	}

	tests := []struct {
		name       string
		input      *domain.TBankPaymentNotification
		setupMocks func(d *testServiceDeps)
		wantErr    bool
	}{
		// ==================== CONFIRMED (completed) ====================
		{
			name:  "confirmed - success",
			input: makeNotification(),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), "valid-token").
					Return(true)

				d.dbRepo.EXPECT().
					TbankCompletePayment(gomock.Any(), domain.TbankCompleteParams{
						IdempotencyKey:    "order-123",
						ProviderPaymentID: "tbank-pid-456",
						Amount:            10000,
						NewStatus:         domain.PaymentStatusCompleted,
						CurrentStatus:     domain.PaymentStatusPending,
						OpType:            domain.OpTypeDeposit,
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "confirmed - already processed - idempotent",
			input: makeNotification(),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankCompletePayment(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrAlreadyProcessed)
			},
			wantErr: false,
		},
		{
			name:  "confirmed - db error",
			input: makeNotification(),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankCompletePayment(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("connection refused"))
			},
			wantErr: true,
		},
		{
			name: "confirmed - success false - validation error",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Success = false
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)
			},
			wantErr: true,
		},

		// ==================== REFUNDED ====================
		{
			name: "refunded - success",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "REFUNDED"
				n.Success = true
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankCompletePayment(gomock.Any(), domain.TbankCompleteParams{
						IdempotencyKey:    "order-123",
						ProviderPaymentID: "tbank-pid-456",
						Amount:            10000,
						NewStatus:         domain.PaymentStatusRefunded,
						CurrentStatus:     domain.PaymentStatusCompleted,
						OpType:            domain.OpTypeRefund,
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "refunded - already processed - idempotent",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "REFUNDED"
				n.Success = true
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankCompletePayment(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrAlreadyProcessed)
			},
			wantErr: false,
		},
		{
			name: "refunded - success false - validation error",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "REFUNDED"
				n.Success = false
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)
			},
			wantErr: true,
		},

		// ==================== REJECTED (failed) ====================
		{
			name: "rejected - success",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "REJECTED"
				n.Success = false
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankUpdateStatus(gomock.Any(), domain.TbankUpdateStatusParams{
						IdempotencyKey: "order-123",
						NewStatus:      domain.PaymentStatusFailed,
						CurrentStatus:  domain.PaymentStatusPending,
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "rejected - already in different status - idempotent",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "REJECTED"
				n.Success = false
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankUpdateStatus(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrInvalidPaymentState)
			},
			wantErr: false,
		},
		{
			name: "rejected - success true - validation error",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "REJECTED"
				n.Success = true
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)
			},
			wantErr: true,
		},

		// ==================== DEADLINE_EXPIRED (expired) ====================
		{
			name: "deadline expired - success",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "DEADLINE_EXPIRED"
				n.Success = false
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankUpdateStatus(gomock.Any(), domain.TbankUpdateStatusParams{
						IdempotencyKey: "order-123",
						NewStatus:      domain.PaymentStatusExpired,
						CurrentStatus:  domain.PaymentStatusPending,
					}).
					Return(nil)
			},
			wantErr: false,
		},

		// ==================== CANCELED (cancelled) ====================
		{
			name: "canceled - success",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "CANCELED"
				n.Success = false
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankUpdateStatus(gomock.Any(), domain.TbankUpdateStatusParams{
						IdempotencyKey: "order-123",
						NewStatus:      domain.PaymentStatusCancelled,
						CurrentStatus:  domain.PaymentStatusPending,
					}).
					Return(nil)
			},
			wantErr: false,
		},

		// ==================== terminal - db error ====================
		{
			name: "rejected - db error",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "REJECTED"
				n.Success = false
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)

				d.dbRepo.EXPECT().
					TbankUpdateStatus(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("connection refused"))
			},
			wantErr: true,
		},

		// ==================== ignored statuses ====================
		{
			name: "NEW status - ignored",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "NEW"
				n.Success = true
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)
			},
			wantErr: false,
		},
		{
			name: "FORM_SHOWED status - ignored",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "FORM_SHOWED"
				n.Success = true
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)
			},
			wantErr: false,
		},
		{
			name: "AUTHORIZED status - ignored",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "AUTHORIZED"
				n.Success = true
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)
			},
			wantErr: false,
		},

		// ==================== token validation ====================
		{
			name:  "invalid token",
			input: makeNotification(),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(false)
			},
			wantErr: true,
		},

		// ==================== unknown status ====================
		{
			name: "unknown provider status",
			input: makeNotification(func(n *domain.TBankPaymentNotification) {
				n.Status = "TOTALLY_UNKNOWN"
			}),
			setupMocks: func(d *testServiceDeps) {
				d.tbank.EXPECT().
					VerifyWebhook(gomock.Any(), gomock.Any()).
					Return(true)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestService(t)
			tt.setupMocks(deps)

			err := svc.TbankWebhook(context.Background(), tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

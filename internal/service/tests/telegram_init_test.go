package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/Krokozabra213/common/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTelegramInit(t *testing.T) {
	userID := uuid.New()
	idempotencyKey := "init-key-123"

	makeInput := func(mods ...func(*domain.TGInit)) *domain.TGInit {
		amount, _ := types.NewPositiveInt(100)
		desc, _ := types.NewNonEmptyString("test payment")
		input := &domain.TGInit{
			UserID:   userID,
			Amount:   amount,
			Currency: domain.CurrencyTypeTGStars,
			Desc:     desc,
		}
		for _, mod := range mods {
			mod(input)
		}
		return input
	}

	makePayment := func(mods ...func(*domain.Payment)) *domain.Payment {
		p := &domain.Payment{
			ID:             uuid.New(),
			UserID:         userID,
			IdempotencyKey: idempotencyKey,
			Amount:         100,
			Currency:       "XTR",
			ProviderName:   string(domain.PaymentTGStars),
			Status:         domain.PaymentStatusStarted,
			Description:    "test payment",
			UpdatedAt:      time.Now(),
		}
		for _, mod := range mods {
			mod(p)
		}
		return p
	}

	makeInitResult := func() *domain.TGInitResult {
		url, _ := types.NewURL("https://t.me/invoice/test123")
		return &domain.TGInitResult{
			PaymentURL: url,
		}
	}

	tests := []struct {
		name       string
		input      *domain.TGInit
		key        string
		setupMocks func(d *testServiceDeps)
		wantErr    bool
		wantURL    string
	}{
		{
			name:  "success - new payment",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(), true, nil)

				d.tg.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTGPaymentPending(gomock.Any(), gomock.Any(), "https://t.me/invoice/test123").
					Return(nil)
			},
			wantErr: false,
			wantURL: "https://t.me/invoice/test123",
		},
		{
			name:  "reserve fails - db error",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(nil, false, fmt.Errorf("connection refused"))
			},
			wantErr: true,
		},
		{
			name:  "existing payment - pending with url - returns existing url",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusPending
						p.PaymentURL = "https://t.me/invoice/existing"
					}), false, nil)
			},
			wantErr: false,
			wantURL: "https://t.me/invoice/existing",
		},
		{
			name:  "existing payment - pending without url - error",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusPending
						p.PaymentURL = ""
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "existing payment - started fresh - payment initializing error",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusStarted
						p.UpdatedAt = time.Now()
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "existing payment - started stale - recovers with new init",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				stalePayment := makePayment(func(p *domain.Payment) {
					p.Status = domain.PaymentStatusStarted
					p.UpdatedAt = time.Now().Add(-60 * time.Second)
				})

				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(stalePayment, false, nil)

				d.tg.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTGPaymentPending(gomock.Any(), stalePayment.ID, "https://t.me/invoice/test123").
					Return(nil)
			},
			wantErr: false,
			wantURL: "https://t.me/invoice/test123",
		},
		{
			name:  "existing payment - started stale - provider init fails",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusStarted
						p.UpdatedAt = time.Now().Add(-60 * time.Second)
					}), false, nil)

				d.tg.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(nil, fmt.Errorf("telegram api error"))
			},
			wantErr: true,
		},
		{
			name:  "existing payment - started stale - mark pending fails",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusStarted
						p.UpdatedAt = time.Now().Add(-60 * time.Second)
					}), false, nil)

				d.tg.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTGPaymentPending(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("db error"))
			},
			wantErr: true,
		},
		{
			name:  "existing payment - unexpected status",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusCompleted
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "existing payment - provider mismatch",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.ProviderName = string(domain.PaymentTbankForm)
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "existing payment - user id mismatch",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.UserID = uuid.New()
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "existing payment - amount mismatch",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Amount = 999
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "existing payment - currency mismatch",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Currency = "RUB"
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "existing payment - description mismatch",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Description = "other description"
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "new payment - provider init fails - marks init failed",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment()

				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(payment, true, nil)

				d.tg.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(nil, fmt.Errorf("telegram unavailable"))

				d.dbRepo.EXPECT().
					MarkTGPaymentInitFailed(gomock.Any(), payment.ID).
					Return(nil)
			},
			wantErr: true,
		},
		{
			name:  "new payment - provider init fails - mark init failed also fails",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment()

				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(payment, true, nil)

				d.tg.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(nil, fmt.Errorf("telegram unavailable"))

				d.dbRepo.EXPECT().
					MarkTGPaymentInitFailed(gomock.Any(), payment.ID).
					Return(fmt.Errorf("db error"))
			},
			wantErr: true,
		},
		{
			name:  "new payment - mark pending fails",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment()

				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(payment, true, nil)

				d.tg.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTGPaymentPending(gomock.Any(), payment.ID, "https://t.me/invoice/test123").
					Return(fmt.Errorf("db error"))
			},
			wantErr: true,
		},
		{
			name:  "passes correct params to reserve",
			input: makeInput(),
			key:   "custom-key-456",
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), domain.CreatePaymentParams{
						UserID:         userID,
						IdempotencyKey: "custom-key-456",
						Amount:         100,
						Currency:       "XTR",
						ProviderName:   string(domain.PaymentTGStars),
						Description:    ptr("test payment"),
					}).
					Return(makePayment(), true, nil)

				d.tg.EXPECT().
					Init(gomock.Any(), gomock.Any(), "custom-key-456").
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTGPaymentPending(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
			wantURL: "https://t.me/invoice/test123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestService(t)
			tt.setupMocks(deps)

			result, err := svc.TelegramInit(context.Background(), tt.input, tt.key)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.wantURL != "" {
					assert.Equal(t, tt.wantURL, result.PaymentURL.Value())
				}
			}
		})
	}
}

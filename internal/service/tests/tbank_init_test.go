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

func TestTbankInit(t *testing.T) {
	userID := uuid.New()
	idempotencyKey := "tbank-init-key-123"

	makeInput := func(mods ...func(*domain.TbankInit)) *domain.TbankInit {
		amount, _ := types.NewPositiveInt(10000)
		input := &domain.TbankInit{
			UserID: userID,
			Amount: amount,
			Desc:   "test tbank payment",
		}
		for _, mod := range mods {
			mod(input)
		}
		return input
	}

	makePayment := func(mods ...func(*domain.Payment)) *domain.Payment {
		p := &domain.Payment{
			ID:                uuid.New(),
			UserID:            userID,
			IdempotencyKey:    idempotencyKey,
			Amount:            10000,
			Currency:          "RUB",
			ProviderName:      string(domain.PaymentTbankForm),
			Status:            domain.PaymentStatusStarted,
			Description:       "test tbank payment",
			ProviderPaymentID: "",
			PaymentURL:        "",
			UpdatedAt:         time.Now(),
		}
		for _, mod := range mods {
			mod(p)
		}
		return p
	}

	makeInitResult := func() *domain.TbankInitResult {
		url, _ := types.NewURL("https://pay.tbank.ru/xyz")
		pid, _ := types.NewNonEmptyString("tbank-pid-456")
		return &domain.TbankInitResult{
			PaymentURL: url,
			PaymentID:  pid,
		}
	}

	tests := []struct {
		name       string
		input      *domain.TbankInit
		key        string
		setupMocks func(d *testServiceDeps)
		wantErr    bool
		wantURL    string
		wantPID    string
	}{
		// ==================== new payment ====================
		{
			name:  "success - new payment",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(), true, nil)

				d.tbank.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTbankPaymentPending(gomock.Any(), gomock.Any(),
						"https://pay.tbank.ru/xyz", "tbank-pid-456").
					Return(nil)
			},
			wantErr: false,
			wantURL: "https://pay.tbank.ru/xyz",
			wantPID: "tbank-pid-456",
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
			name:  "new payment - provider init fails - marks init failed",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				payment := makePayment()

				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(payment, true, nil)

				d.tbank.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(nil, fmt.Errorf("tbank unavailable"))

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

				d.tbank.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(nil, fmt.Errorf("tbank unavailable"))

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

				d.tbank.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTbankPaymentPending(gomock.Any(), payment.ID, gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("db error"))
			},
			wantErr: true,
		},

		// ==================== existing pending ====================
		{
			name:  "existing pending - returns existing url and pid",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusPending
						p.PaymentURL = "https://pay.tbank.ru/existing"
						p.ProviderPaymentID = "existing-pid"
					}), false, nil)
			},
			wantErr: false,
			wantURL: "https://pay.tbank.ru/existing",
			wantPID: "existing-pid",
		},
		{
			name:  "existing pending - empty url - error",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusPending
						p.PaymentURL = ""
						p.ProviderPaymentID = "pid-123"
					}), false, nil)
			},
			wantErr: true,
		},
		{
			name:  "existing pending - empty provider_payment_id - error",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusPending
						p.PaymentURL = "https://pay.tbank.ru/test"
						p.ProviderPaymentID = ""
					}), false, nil)
			},
			wantErr: true,
		},

		// ==================== existing started ====================
		{
			name:  "existing started fresh - payment initializing error",
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
			name:  "existing started stale - recovers with new init",
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

				d.tbank.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTbankPaymentPending(gomock.Any(), stalePayment.ID,
						"https://pay.tbank.ru/xyz", "tbank-pid-456").
					Return(nil)
			},
			wantErr: false,
			wantURL: "https://pay.tbank.ru/xyz",
			wantPID: "tbank-pid-456",
		},
		{
			name:  "existing started stale - provider init fails",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusStarted
						p.UpdatedAt = time.Now().Add(-60 * time.Second)
					}), false, nil)

				d.tbank.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(nil, fmt.Errorf("tbank api error"))
			},
			wantErr: true,
		},
		{
			name:  "existing started stale - mark pending fails",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusStarted
						p.UpdatedAt = time.Now().Add(-60 * time.Second)
					}), false, nil)

				d.tbank.EXPECT().
					Init(gomock.Any(), gomock.Any(), idempotencyKey).
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTbankPaymentPending(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("db error"))
			},
			wantErr: true,
		},

		// ==================== existing unexpected status ====================
		{
			name:  "existing paid - unexpected status error",
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
			name:  "existing failed - unexpected status error",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.Status = domain.PaymentStatusFailed
					}), false, nil)
			},
			wantErr: true,
		},

		// ==================== validation conflicts ====================
		{
			name:  "existing payment - provider mismatch",
			input: makeInput(),
			key:   idempotencyKey,
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), gomock.Any()).
					Return(makePayment(func(p *domain.Payment) {
						p.ProviderName = string(domain.PaymentTGStars)
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
						p.Amount = 999999
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
						p.Currency = "XTR"
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

		// ==================== correct params mapping ====================
		{
			name:  "passes correct params to reserve",
			input: makeInput(),
			key:   "custom-key-789",
			setupMocks: func(d *testServiceDeps) {
				desc := "test tbank payment"
				d.dbRepo.EXPECT().
					PaymentReserve(gomock.Any(), domain.CreatePaymentParams{
						UserID:         userID,
						IdempotencyKey: "custom-key-789",
						Amount:         10000,
						Currency:       string(domain.CurrencyTypeRUB),
						ProviderName:   string(domain.PaymentTbankForm),
						Description:    &desc,
					}).
					Return(makePayment(), true, nil)

				d.tbank.EXPECT().
					Init(gomock.Any(), gomock.Any(), "custom-key-789").
					Return(makeInitResult(), nil)

				d.dbRepo.EXPECT().
					MarkTbankPaymentPending(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
			wantURL: "https://pay.tbank.ru/xyz",
			wantPID: "tbank-pid-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestService(t)
			tt.setupMocks(deps)

			result, err := svc.TbankInit(context.Background(), tt.input, tt.key)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.wantURL != "" {
					assert.Equal(t, tt.wantURL, result.PaymentURL.Value())
				}
				if tt.wantPID != "" {
					assert.Equal(t, tt.wantPID, result.PaymentID.Value())
				}
			}
		})
	}
}

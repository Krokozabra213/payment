package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTGSuccessful(t *testing.T) {
	makeSP := func(mods ...func(*domain.SuccessfulPayment)) domain.SuccessfulPayment {
		sp := domain.SuccessfulPayment{
			Currency:                "XTR",
			TotalAmount:             100,
			InvoicePayload:          "order-123",
			TelegramPaymentChargeID: "tg-charge-abc",
			ProviderPaymentChargeID: "provider-charge-xyz",
		}
		for _, mod := range mods {
			mod(&sp)
		}
		return sp
	}

	tests := []struct {
		name       string
		input      domain.SuccessfulPayment
		setupMocks func(d *testServiceDeps)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:  "success",
			input: makeSP(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), domain.TGCompletePaymentParams{
						IdempotencyKey:          "order-123",
						TelegramPaymentChargeID: "tg-charge-abc",
						Amount:                  100,
						Currency:                "XTR",
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "already paid - idempotent success",
			input: makeSP(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), domain.TGCompletePaymentParams{
						IdempotencyKey:          "order-123",
						TelegramPaymentChargeID: "tg-charge-abc",
						Amount:                  100,
						Currency:                "XTR",
					}).
					Return(pgRepo.ErrAlreadyPaid)
			},
			wantErr: false,
		},
		{
			name:  "payment not found",
			input: makeSP(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrNotFound)
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:  "invalid payment status",
			input: makeSP(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrInvalidPaymentStatus)
			},
			wantErr:    true,
			wantErrMsg: "invalid payment status",
		},
		{
			name:  "amount mismatch",
			input: makeSP(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrAmountMismatch)
			},
			wantErr:    true,
			wantErrMsg: "amount mismatch",
		},
		{
			name:  "currency mismatch",
			input: makeSP(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrCurrencyMismatch)
			},
			wantErr:    true,
			wantErrMsg: "currency mismatch",
		},
		{
			name:  "provider mismatch",
			input: makeSP(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrProviderMismatch)
			},
			wantErr:    true,
			wantErrMsg: "provider mismatch",
		},
		{
			name:  "db internal error",
			input: makeSP(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("connection refused"))
			},
			wantErr:    true,
			wantErrMsg: "connection refused",
		},
		{
			name: "passes correct params from input",
			input: makeSP(func(sp *domain.SuccessfulPayment) {
				sp.InvoicePayload = "custom-key-999"
				sp.TelegramPaymentChargeID = "custom-charge-id"
				sp.TotalAmount = 555
				sp.Currency = "USD"
			}),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGCompletePayment(gomock.Any(), domain.TGCompletePaymentParams{
						IdempotencyKey:          "custom-key-999",
						TelegramPaymentChargeID: "custom-charge-id",
						Amount:                  555,
						Currency:                "USD",
					}).
					Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, deps := newTestService(t)
			tt.setupMocks(deps)

			err := svc.TGSuccessful(context.Background(), tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/GargantuaLabs/payment/internal/domain"
	pgRepo "github.com/GargantuaLabs/payment/internal/repository/postgres"
	"github.com/Krokozabra213/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTGPrecheckout(t *testing.T) {
	id, _ := types.NewNonEmptyString("query-123")
	customID, _ := types.NewNonEmptyString("custom-query-id")
	makeQuery := func(mods ...func(*domain.PreCheckoutQuery)) *domain.PreCheckoutQuery {
		q := &domain.PreCheckoutQuery{
			ID:             id,
			InvoicePayload: "order-123",
			Currency:       "XTR",
			TotalAmount:    100,
			From: domain.User{
				ID: 123456,
			},
		}
		for _, mod := range mods {
			mod(q)
		}
		return q
	}

	tests := []struct {
		name       string
		input      *domain.PreCheckoutQuery
		setupMocks func(d *testServiceDeps)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:  "success",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), domain.TGPrecheckoutApproveParams{
						IdempotencyKey: "order-123",
						TGUserID:       int64(123456),
						Amount:         100,
						Currency:       "XTR",
					}).
					Return(nil)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), domain.PreCheckoutPayload{
						PreCheckoutQueryID: id,
						Ok:                 true,
					}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "db approve fails - answers telegram with error",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("connection refused"))

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p domain.PreCheckoutPayload) error {
						assert.False(t, p.Ok)
						assert.Equal(t, "query-123", p.PreCheckoutQueryID.Value())
						assert.NotEmpty(t, p.ErrorMsg)
						return nil
					})
			},
			wantErr:    true,
			wantErrMsg: "connection refused",
		},
		{
			name:  "payment not found - answers telegram with error",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrNotFound)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p domain.PreCheckoutPayload) error {
						assert.False(t, p.Ok)
						assert.Contains(t, p.ErrorMsg, "Произошла ошибка, попробуйте чуть позже")
						return nil
					})
			},
			wantErr:    true,
			wantErrMsg: "not found",
		},
		{
			name:  "amount mismatch - answers telegram with error",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrAmountMismatch)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p domain.PreCheckoutPayload) error {
						assert.False(t, p.Ok)
						assert.NotEmpty(t, p.ErrorMsg)
						return nil
					})
			},
			wantErr:    true,
			wantErrMsg: "amount mismatch",
		},
		{
			name:  "currency mismatch - answers telegram with error",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrCurrencyMismatch)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p domain.PreCheckoutPayload) error {
						assert.False(t, p.Ok)
						assert.NotEmpty(t, p.ErrorMsg)
						return nil
					})
			},
			wantErr:    true,
			wantErrMsg: "currency mismatch",
		},
		{
			name:  "invalid payment status - answers telegram with error",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrInvalidPaymentStatus)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p domain.PreCheckoutPayload) error {
						assert.False(t, p.Ok)
						assert.NotEmpty(t, p.ErrorMsg)
						return nil
					})
			},
			wantErr:    true,
			wantErrMsg: "invalid payment status",
		},
		{
			name:  "payment is processing - answers telegram with error",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrPaymentProcessed)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, p domain.PreCheckoutPayload) error {
						assert.False(t, p.Ok)
						assert.NotEmpty(t, p.ErrorMsg)
						return nil
					})
			},
			wantErr:    true,
			wantErrMsg: "payment is processing",
		},
		// {
		// 	name:  "provider user mismatch - answers telegram with error",
		// 	input: makeQuery(),
		// 	setupMocks: func(d *testServiceDeps) {
		// 		d.dbRepo.EXPECT().
		// 			TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
		// 			Return(pgRepo.ErrProviderUserMismatch)

		// 		d.tg.EXPECT().
		// 			AnswerPreCheckout(gomock.Any(), gomock.Any()).
		// 			DoAndReturn(func(_ context.Context, p domain.PreCheckoutPayload) error {
		// 				assert.False(t, p.Ok)
		// 				assert.NotEmpty(t, p.ErrorMsg)
		// 				return nil
		// 			})
		// 	},
		// 	wantErr:    true,
		// 	wantErrMsg: "provider user mismatch",
		// },
		{
			name:  "approve ok but answer telegram fails",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
					Return(nil)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), domain.PreCheckoutPayload{
						PreCheckoutQueryID: id,
						Ok:                 true,
					}).
					Return(fmt.Errorf("telegram api timeout"))
			},
			wantErr:    true,
			wantErrMsg: "telegram api timeout",
		},
		{
			name:  "approve fails and answer telegram also fails - returns original error",
			input: makeQuery(),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), gomock.Any()).
					Return(pgRepo.ErrAmountMismatch)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("telegram api down"))
			},
			wantErr:    true,
			wantErrMsg: "amount mismatch",
		},
		{
			name: "passes correct params from query",
			input: makeQuery(func(q *domain.PreCheckoutQuery) {
				q.ID = customID
				q.InvoicePayload = "custom-payload"
				q.TotalAmount = 999
				q.Currency = "USD"
				q.From.ID = 789012
			}),
			setupMocks: func(d *testServiceDeps) {
				d.dbRepo.EXPECT().
					TGPrecheckoutApprove(gomock.Any(), domain.TGPrecheckoutApproveParams{
						IdempotencyKey: "custom-payload",
						TGUserID:       int64(789012),
						Amount:         999,
						Currency:       "USD",
					}).
					Return(nil)

				d.tg.EXPECT().
					AnswerPreCheckout(gomock.Any(), domain.PreCheckoutPayload{
						PreCheckoutQueryID: customID,
						Ok:                 true,
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

			err := svc.TGPrecheckout(context.Background(), tt.input)

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

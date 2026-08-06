package tg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/Krokozabra213/common/types"
)

func (p *Provider) Cancel(ctx context.Context, userID types.PositiveInt[int64], tgPaymentChargeID types.NonEmptyString) error {
	op := "TelegramProvider.Cancel"

	baseFields := apperror.Fields{
		apperror.F("tg_payment_charge_id", tgPaymentChargeID.Value()),
		apperror.F("user_id", userID.Value()),
	}

	reqPayload := NewRefundStarPaymentRequest(userID.Value(), tgPaymentChargeID.Value())

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to marshal refund request", err, apperror.LevelError, baseFields)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/refundStarPayment", bytes.NewReader(body))
	if err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to create refund request", err, apperror.LevelError, baseFields)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to send refund request", err, apperror.LevelError, baseFields)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to read refund response", err, apperror.LevelError, baseFields)
	}

	var result RefundStarPaymentResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse dto refund response", err, apperror.LevelError, baseFields)
	}

	if !result.OK {
		msg := fmt.Sprintf("telegram cancel failed: code=%d, message=%s", result.ErrorCode, result.Description)
		return apperror.NewAppErr(apperror.CodeInternal, op, msg, nil, apperror.LevelError, baseFields)
	}

	return nil
}

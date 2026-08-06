package tbank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/Krokozabra213/common/types"
)

func (p *Provider) Cancel(ctx context.Context, paymentID types.NonEmptyString) (*domain.TbankCancel, error) {
	op := "TbankProvider.CancelPayment"

	baseFields := apperror.Fields{
		apperror.F("payment_id", paymentID.Value()),
	}

	reqBody := NewCancelRequest(p.terminalKey, paymentID.Value())
	reqBody.addToken(p.generateToken(reqBody.ToMap()))

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed marshal cancel body", err, apperror.LevelError, baseFields)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/Cancel", bytes.NewReader(body))
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to create cancel request", err, apperror.LevelError, baseFields)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to send cancel request", err, apperror.LevelError, baseFields)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to read cancel response", err, apperror.LevelError, baseFields)
	}

	var result CancelResponse

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse cancel response", err, apperror.LevelError, baseFields)
	}

	if !result.Success {
		msg := fmt.Sprintf("tinkoff cancel failed: code=%s, message=%s", result.ErrorCode, result.Message)
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, msg, nil, apperror.LevelError, baseFields)
	}

	res, err := domain.NewTbankCancel(result.OrderID, result.PaymentID)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse cancel domain response", err, apperror.LevelError, baseFields)
	}

	return &res, nil
}

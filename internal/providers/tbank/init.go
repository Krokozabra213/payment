package tbank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
)

func (p *Provider) Init(ctx context.Context, payload *domain.TbankInit, idempotencyKey string) (*domain.TbankInitResult, error) {
	op := "TbankProvider.InitPayment"

	baseFields := apperror.Fields{
		apperror.F("idempotencyKey", idempotencyKey),
		apperror.F("user_id", payload.UserID.String()),
		apperror.F("amount", payload.Amount.Value()),
	}

	reqBody := NewInitRequest(p.terminalKey, idempotencyKey, payload.Desc, payload.NotificationURL, payload.Amount.Value())
	fmt.Println(reqBody.ToMap())
	reqBody.addToken(p.generateToken(reqBody.ToMap()))

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed marshal init body", err, apperror.LevelError, baseFields)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/Init", bytes.NewReader(body))
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to create init request", err, apperror.LevelError, baseFields)
	}
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	var reqErr error

	for i := 0; i < 3; i++ {
		req.Body = io.NopCloser(bytes.NewReader(body))
		resp, reqErr = p.httpClient.Do(req)

		if reqErr == nil {
			break
		}

		if resp != nil {
			resp.Body.Close()
		}

		time.Sleep(time.Duration(i*i) * 100 * time.Millisecond)
	}
	err = reqErr
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to send init request", err, apperror.LevelError, baseFields)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to read init response", err, apperror.LevelError, baseFields)
	}

	var result InitResponse
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse init response", err, apperror.LevelError, baseFields)
	}

	err = p.validateTbankInitResult(&result, reqBody)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to validate init response", err, apperror.LevelError, baseFields)
	}

	initResult, err := domain.NewTbankInitResult(result.PaymentURL, result.PaymentID)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse in structure init response", err, apperror.LevelError, baseFields)
	}

	return &initResult, nil
}

func (p *Provider) validateTbankInitResult(result *InitResponse, req *InitRequest) error {
	if result.TerminalKey != p.terminalKey {
		return fmt.Errorf("terminal key mismatched")
	}

	if !result.Success {
		return fmt.Errorf("success=false")
	}

	if result.Status != string(domain.TbankProviderStatusNew) {
		return fmt.Errorf("status=%s", result.Status)
	}

	if result.OrderID != req.OrderID {
		return fmt.Errorf("request order_id=%s, response order_id=%s", req.OrderID, result.OrderID)
	}

	if result.Amount != req.Amount {
		return fmt.Errorf("request amount=%d, response amount=%d", req.Amount, result.Amount)
	}
	return nil
}

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

func (p *Provider) Status(ctx context.Context, paymentID types.NonEmptyString) (*domain.PaymentStatus, error) {
	op := "TbankProvider.Status"

	baseFields := apperror.Fields{
		apperror.F("payment_id", paymentID.Value()),
	}

	reqBody := StateRequest{
		TerminalKey: p.terminalKey,
		PaymentID:   paymentID.Value(),
	}

	reqBody.addToken(p.generateToken(reqBody.ToMap()))

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed marshal status body", err, apperror.LevelError, baseFields)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/GetState", bytes.NewReader(body))
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to create status request", err, apperror.LevelError, baseFields)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to send status request", err, apperror.LevelError, baseFields)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to read status response", err, apperror.LevelError, baseFields)
	}

	var result StateResponse

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse status response", err, apperror.LevelError, baseFields)
	}

	if !result.Success {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op,
			fmt.Sprintf("tinkoff GetState failed: code=%s, message=%s", result.ErrorCode, result.Message),
			nil, apperror.LevelError, baseFields)
	}

	providerStatus, err := domain.NewTBankProviderStatus(result.Status)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse status from response", err, apperror.LevelError, baseFields)
	}

	if providerStatus == domain.TbankProviderStatusNew {
		statusPending := domain.PaymentStatusPending
		return &statusPending, nil
	}

	paymentStatus, err := providerStatus.ToPaymentStatus()
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed parse to payment status from provider status", err, apperror.LevelError, baseFields)
	}

	return &paymentStatus, nil
}

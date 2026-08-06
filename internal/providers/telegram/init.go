package tg

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

func (p *Provider) Init(ctx context.Context, payload *domain.TGInit, idempotencyKey string) (*domain.TGInitResult, error) {
	op := "TelegramProvider.Init"

	baseFields := apperror.Fields{
		apperror.F("idempotencyKey", idempotencyKey),
		apperror.F("user_id", payload.UserID.String()),
		apperror.F("amount", payload.Amount.Value()),
	}

	prices := make([]LabeledPrice, 0, 1)
	prices = append(prices, NewLabeledPrice(payload.Title.Value(), payload.Amount.Value()))

	reqPayload := NewCreateInvoiceLinkRequest(payload.Title.Value(), payload.Desc.Value(),
		idempotencyKey, string(payload.Currency), prices)

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed marshal init body", err, apperror.LevelError, baseFields)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/createInvoiceLink", bytes.NewReader(body))
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to create invoice link request", err, apperror.LevelError, baseFields)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to send create invoice link request", err, apperror.LevelError, baseFields)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to read create invoice link response", err, apperror.LevelError, baseFields)
	}

	var result CreateInvoiceLinkResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse create invoice link response", err, apperror.LevelError, baseFields)
	}

	if !result.OK {
		msg := fmt.Sprintf("failed create invoice link: code=%d, description=%s", result.ErrorCode, result.Description)
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, msg, nil, apperror.LevelError, baseFields)
	}

	paymentURL, err := types.NewURL(result.Result)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to shared parse paymentURL from response", err, apperror.LevelError, baseFields)
	}

	return &domain.TGInitResult{
		PaymentURL: paymentURL,
	}, nil
}

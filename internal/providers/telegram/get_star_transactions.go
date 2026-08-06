package tg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	apperror "github.com/GargantuaLabs/payment/internal/app_error"
	"github.com/GargantuaLabs/payment/internal/domain"
)

func (p *Provider) GetStarTransactions(ctx context.Context, limit, offset int) ([]domain.ConfirmedStarTx, error) {
	op := "TelegramProvider.GetStarTransactions"

	baseFields := apperror.Fields{
		apperror.F("limit", limit),
		apperror.F("offset", offset),
	}

	reqPayload := GetStarTransactionsRequest{
		Offset: offset,
		Limit:  limit,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to marshal get star transactions request", err, apperror.LevelError, baseFields)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/getStarTransactions",
		bytes.NewReader(body))
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to create get star transactions request", err, apperror.LevelError, baseFields)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to send get star transactions request", err, apperror.LevelError, baseFields)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to read get star transactions response", err, apperror.LevelError, baseFields)
	}

	var result StarTransactionsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, apperror.NewAppErr(apperror.CodeInternal, op, "failed to read get star transactions response", err, apperror.LevelError, baseFields)
	}

	if result.Transactions == nil {
		return nil, nil
	}

	payloads := make([]domain.ConfirmedStarTx, 0, len(result.Transactions))
	for _, tx := range result.Transactions {
		if tx.Source != nil && tx.Source.Type == "user" && tx.Source.InvoicePayload != "" {
			payload, err := domain.NewConfirmedStarTx(tx.Source.User.ID, tx.Source.InvoicePayload)
			if err != nil {
				continue
			}
			payloads = append(payloads, payload)
		}
	}

	return payloads, nil
}

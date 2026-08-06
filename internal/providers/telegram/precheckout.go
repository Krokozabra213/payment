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
)

func (p *Provider) AnswerPreCheckout(ctx context.Context, preCheckout domain.PreCheckoutPayload) error {
	op := "Telegram.AnswerPreCheckout"

	baseFields := apperror.Fields{
		apperror.F("precheckout_query_id", preCheckout.PreCheckoutQueryID),
		apperror.F("error_message", preCheckout.ErrorMsg),
		apperror.F("ok", preCheckout.Ok),
	}

	reqPayload := NewAnswerPreCheckoutQueryRequest(preCheckout.PreCheckoutQueryID.Value(), preCheckout.ErrorMsg, preCheckout.Ok)

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to marshal answer pre checkout query request", err, apperror.LevelError, baseFields)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/answerPreCheckoutQuery", bytes.NewReader(body))
	if err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to create answer pre checkout query request", err, apperror.LevelError, baseFields)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to send answer pre checkout query request", err, apperror.LevelError, baseFields)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to read answer pre checkout query response", err, apperror.LevelError, baseFields)
	}

	var result AnswerPreCheckoutQueryResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return apperror.NewAppErr(apperror.CodeInternal, op, "failed to parse dto answer pre checkout query response", err, apperror.LevelError, baseFields)
	}

	if !result.OK {
		msg := fmt.Sprintf("answer pre checkout query failed: code=%d, message=%s", result.ErrorCode, result.Description)
		return apperror.NewAppErr(apperror.CodeInternal, op, msg, nil, apperror.LevelError, baseFields)
	}

	return nil
}

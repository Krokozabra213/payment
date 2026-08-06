package pgRepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) TGCompletePayment(
	ctx context.Context,
	params domain.TGCompletePaymentParams,
) error {
	tx, err := r.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `
		SELECT
			id, user_id, idempotency_key, amount, currency, status,
			provider_name, provider_payment_id, provider_user_id, payment_url,
			description, expires_at, paid_at, created_at, updated_at
		FROM payments
		WHERE idempotency_key = @idempotency_key
		FOR UPDATE;
	`

	rows, err := tx.Query(ctx, selectQuery, pgx.NamedArgs{
		"idempotency_key": params.IdempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("select for update: %w", err)
	}
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[paymentRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	payment, err := row.toDomain()
	if err != nil {
		return err
	}

	if err := validateTGComplete(payment, params); err != nil {
		return err
	}

	const updateQuery = `
		UPDATE payments
		SET
			status = @status,
			provider_payment_id = @provider_payment_id,
			paid_at = now(),
			updated_at = now()
		WHERE id = @id
		  AND status = @current_status;
	`

	cmd, err := tx.Exec(ctx, updateQuery, pgx.NamedArgs{
		"id":                  payment.ID,
		"status":              string(domain.PaymentStatusCompleted),
		"provider_payment_id": params.TelegramPaymentChargeID,
		"current_status":      string(domain.PaymentStatusProcessed),
	})
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	if cmd.RowsAffected() != 1 {
		return ErrInvalidPaymentState
	}

	operationID := fmt.Sprintf("%s:telegram:%s", string(domain.OpTypeDeposit), payment.ID.String())
	eventKey := payment.UserID.String()

	const query = `
		SELECT rub_rate
		FROM currency_rates
		WHERE code = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var usdRate int64
	err = r.QueryRow(ctx, query, string(domain.CurrencyUSD)).Scan(&usdRate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %s", ErrNotFound, string(domain.CurrencyUSD))
		}
		return fmt.Errorf("get latest rate for %s: %w", string(domain.CurrencyUSD), err)
	}

	xtrRateFloat := float64(usdRate) * 0.013
	xtrRateKopecks := int(math.Round(xtrRateFloat))

	payload, err := json.Marshal(domain.BalancePayload{
		OperationID: operationID,
		UserID:      payment.UserID.String(),
		PaymentID:   payment.ID.String(),
		Type:        string(domain.OpTypeDeposit),
		Amount:      xtrRateKopecks,
	})
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	const outboxQuery = `
		INSERT INTO payments_outbox (
			operation_id,
			payment_id,
			type,
			amount,
            event_key,
			payload,
            status
		)
		VALUES (@operation_id, @payment_id, @type, @amount,@event_key,@payload, @status);
	`

	_, err = tx.Exec(ctx, outboxQuery, pgx.NamedArgs{
		"operation_id": operationID,
		"payment_id":   payment.ID,
		"type":         string(domain.OpTypeDeposit),
		"amount":       payment.Amount,
		"event_key":    eventKey,
		"payload":      string(payload),
		"status":       domain.OutboxStatusPending,
	})
	if err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	return tx.Commit(ctx)
}

func validateTGComplete(payment *domain.Payment, p domain.TGCompletePaymentParams) error {
	if payment.ProviderName != string(domain.PaymentTGStars) {
		return ErrProviderMismatch
	}

	switch payment.Status {
	case domain.PaymentStatusProcessed:

	case domain.PaymentStatusCompleted:
		// уже оплачен — идемпотентность, не ошибка
		return ErrAlreadyPaid

	default:
		return ErrInvalidPaymentStatus
	}

	if payment.Amount != p.Amount {
		return ErrAmountMismatch
	}

	if payment.Currency != p.Currency {
		return ErrCurrencyMismatch
	}

	return nil
}

package pgRepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) TbankCompletePayment(
	ctx context.Context,
	params domain.TbankCompleteParams,
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
		  AND provider_name = @provider_name
		FOR UPDATE;
	`

	rows, err := tx.Query(ctx, selectQuery, pgx.NamedArgs{
		"idempotency_key": params.IdempotencyKey,
		"provider_name":   string(domain.PaymentTbankForm),
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

	if payment.Status == params.NewStatus {
		return ErrAlreadyProcessed
	}

	if payment.Status != params.CurrentStatus {
		return ErrInvalidPaymentState
	}

	if payment.ProviderPaymentID != "" && payment.ProviderPaymentID != params.ProviderPaymentID {
		return ErrProviderPaymentIDMismatch
	}

	if params.NewStatus == domain.PaymentStatusCompleted && payment.Amount != params.Amount {
		return ErrAmountMismatch
	}

	const updateQuery = `
		UPDATE payments
		SET
			status = @status,
			paid_at = CASE WHEN @set_paid_at THEN now() ELSE paid_at END,
			updated_at = now()
		WHERE id = @id;
	`

	cmd, err := tx.Exec(ctx, updateQuery, pgx.NamedArgs{
		"id":          payment.ID,
		"status":      string(params.NewStatus),
		"set_paid_at": params.NewStatus == domain.PaymentStatusCompleted,
	})
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	if cmd.RowsAffected() != 1 {
		return ErrInvalidPaymentState
	}

	operationID := fmt.Sprintf("%s:tbank:%s", string(params.OpType), payment.ID.String())
	eventKey := payment.UserID.String()

	payload, err := json.Marshal(domain.BalancePayload{
		OperationID: operationID,
		UserID:      payment.UserID.String(),
		PaymentID:   payment.ID.String(),
		Type:        string(params.OpType),
		Amount:      payment.Amount,
	})
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	const outboxQuery = `
		INSERT INTO payments_outbox (
			operation_id, payment_id, type, amount, event_key, payload, status
		)
		VALUES (@operation_id, @payment_id, @type, @amount, @event_key, @payload, @status);
	`

	_, err = tx.Exec(ctx, outboxQuery, pgx.NamedArgs{
		"operation_id": operationID,
		"payment_id":   payment.ID,
		"type":         string(params.OpType),
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

package pgRepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) PaymentFinishRefund(
	ctx context.Context,
	paymentID uuid.UUID,
	amount int,
) error {
	tx, err := r.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const updateQuery = `
		UPDATE payments
		SET status = @status, updated_at = now()
		WHERE id = @id AND status = @current_status
        RETURNING user_id;
	`

	var returnedUserID uuid.UUID

	err = tx.QueryRow(ctx, updateQuery, pgx.NamedArgs{
		"id":             paymentID,
		"status":         string(domain.PaymentStatusRefunded),
		"current_status": string(domain.PaymentStatusRefunding),
	}).Scan(&returnedUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidPaymentState
		}
		return fmt.Errorf("update status with returning: %w", err)
	}

	operationID := fmt.Sprintf("refund:%s", paymentID.String())
	eventKey := returnedUserID.String()

	payload, err := json.Marshal(domain.BalancePayload{
		OperationID: operationID,
		UserID:      returnedUserID.String(),
		PaymentID:   paymentID.String(),
		Type:        string(domain.OpTypeRefund),
		Amount:      amount,
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
		"payment_id":   paymentID,
		"type":         string(domain.OpTypeRefund),
		"amount":       amount,
		"event_key":    eventKey,
		"payload":      string(payload),
		"status":       domain.OutboxStatusPending,
	})
	if err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	return tx.Commit(ctx)
}

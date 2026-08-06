package pgRepo

import (
	"time"

	"github.com/GargantuaLabs/payment/internal/domain"
	"github.com/google/uuid"
)

type PaymentsOutbox struct {
	ID            uuid.UUID  `db:"id"`
	Attempts      int        `db:"attempts"`
	NextAttemptAt time.Time  `db:"next_attempt_at"`
	LastError     *string    `db:"last_error,omitempty"`
	ProcessedAt   *time.Time `db:"processed_at,omitempty"`
	ClaimedBy     *string    `db:"claimed_by,omitempty"`
	ClaimUntil    *time.Time `db:"claim_until,omitempty"`
	OperationID   string     `db:"operation_id"`
	PaymentID     uuid.UUID  `db:"payment_id"`
	Type          string     `db:"type"`
	Amount        int        `db:"amount"`
	Status        string     `db:"status"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	EventKey      string     `db:"event_key"`
	Payload       []byte     `db:"payload"`
}

func (p *PaymentsOutbox) ToBalanceEvent() *domain.BalanceEvent {
	return &domain.BalanceEvent{
		EventKey: p.EventKey,
		Payload:  p.Payload,
	}
}

type paymentRow struct {
	ID                uuid.UUID  `db:"id"`
	UserID            uuid.UUID  `db:"user_id"`
	IdempotencyKey    string     `db:"idempotency_key"`
	Amount            int        `db:"amount"`
	Currency          string     `db:"currency"`
	Status            string     `db:"status"`
	ProviderName      string     `db:"provider_name"`
	ProviderPaymentID *string    `db:"provider_payment_id"`
	PaymentURL        *string    `db:"payment_url"`
	Description       *string    `db:"description"`
	ProviderUserID    *int64     `db:"provider_user_id"`
	ExpiresAt         *time.Time `db:"expires_at"`
	PaidAt            *time.Time `db:"paid_at"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

func (r paymentRow) toDomain() (*domain.Payment, error) {
	status, err := domain.ParsePaymentStatus(r.Status)
	if err != nil {
		return nil, err
	}

	return &domain.Payment{
		ID:                r.ID,
		UserID:            r.UserID,
		IdempotencyKey:    r.IdempotencyKey,
		Amount:            r.Amount,
		Currency:          r.Currency,
		Status:            status,
		ProviderName:      r.ProviderName,
		ProviderPaymentID: derefString(r.ProviderPaymentID),
		PaymentURL:        derefString(r.PaymentURL),
		Description:       derefString(r.Description),
		ProviderUserID:    derefInt64(r.ProviderUserID),
		ExpAt:             derefTime(r.ExpiresAt),
		PaidAt:            derefTime(r.PaidAt),
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

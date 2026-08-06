package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type OpType string

const (
	OpTypeDeposit OpType = "deposit"
	OpTypeRefund  OpType = "refund"
)

type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusProcessed OutboxStatus = "processed"
	OutboxStatusFailed    OutboxStatus = "failed"
)

type PaymentType string

const (
	PaymentTbankForm PaymentType = "tbank_form"
	PaymentTGStars   PaymentType = "tg_stars"
)

type PaymentStatus string

const (
	PaymentStatusStarted    PaymentStatus = "started"
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusProcessed  PaymentStatus = "processed"
	PaymentStatusCompleted  PaymentStatus = "completed"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusExpired    PaymentStatus = "expired"
	PaymentStatusRefunded   PaymentStatus = "refunded"
	PaymentStatusCancelled  PaymentStatus = "cancelled"
	PaymentStatusRefunding  PaymentStatus = "refunding"
	PaymentStatusCancelling PaymentStatus = "cancelling"
)

var statusMap = map[string]PaymentStatus{
	"started":    PaymentStatusStarted,
	"pending":    PaymentStatusPending,
	"completed":  PaymentStatusCompleted,
	"failed":     PaymentStatusFailed,
	"expired":    PaymentStatusExpired,
	"refunded":   PaymentStatusRefunded,
	"cancelled":  PaymentStatusCancelled,
	"processed":  PaymentStatusProcessed,
	"refunding":  PaymentStatusRefunding,
	"cancelling": PaymentStatusCancelling,
}

func ParsePaymentStatus(s string) (PaymentStatus, error) {
	if status, ok := statusMap[strings.ToLower(s)]; ok {
		return status, nil
	}
	return "", fmt.Errorf("unknown payment status: %s", s)
}

type CreatePaymentParams struct {
	UserID            uuid.UUID
	IdempotencyKey    string
	Amount            int
	Currency          string
	ProviderName      string
	ProviderPaymentID *string
	PaymentURL        *string
	Description       *string
	ExpiresAt         *time.Time
}

type Payment struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	IdempotencyKey    string
	Amount            int
	Currency          string
	Status            PaymentStatus
	ProviderName      string
	ProviderPaymentID string
	PaymentURL        string
	Description       string
	ProviderUserID    int64
	ExpAt             time.Time
	PaidAt            time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PaymentCancelLockParams struct {
	PaymentID     uuid.UUID
	CurrentStatus PaymentStatus
	LockStatus    PaymentStatus
}

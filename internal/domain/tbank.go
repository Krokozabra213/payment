package domain

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Krokozabra213/common/types"
	"github.com/google/uuid"
)

type TbankInit struct {
	UserID          uuid.UUID
	Amount          types.PositiveInt[int]
	Desc            string
	NotificationURL string
}

func NewTbankInit(userID string, amount int, desc, notificationURL string) (*TbankInit, error) {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return nil, errors.New("failed parse user_id")
	}

	parsedAmount, err := types.NewPositiveInt(amount)
	if err != nil {
		return nil, fmt.Errorf("failed parse amount, %w", err)
	}

	return &TbankInit{
		UserID:          parsedID,
		Amount:          parsedAmount,
		Desc:            desc,
		NotificationURL: notificationURL,
	}, nil
}

type TbankInitResult struct {
	PaymentURL types.URL
	PaymentID  types.NonEmptyString
}

func NewTbankInitResult(url, id string) (TbankInitResult, error) {
	paymentID, err := types.NewNonEmptyString(id)
	if err != nil {
		return TbankInitResult{}, fmt.Errorf("invalid paymentID: %w", err)
	}

	paymentURL, err := types.NewURL(url)
	if err != nil {
		return TbankInitResult{}, fmt.Errorf("invalid paymentURL: %w", err)
	}
	return TbankInitResult{
		PaymentID:  paymentID,
		PaymentURL: paymentURL,
	}, nil
}

type TbankCancel struct {
	ProviderOrderID types.NonEmptyString
	PaymentID       types.NonEmptyString
}

func NewTbankCancel(orderID, paymentID string) (TbankCancel, error) {
	parsedOrderID, err := types.NewNonEmptyString(orderID)
	if err != nil {
		return TbankCancel{}, fmt.Errorf("invalid orderID: %w", err)
	}

	parsedPaymentID, err := types.NewNonEmptyString(paymentID)
	if err != nil {
		return TbankCancel{}, fmt.Errorf("invalid paymentID: %w", err)
	}
	return TbankCancel{
		ProviderOrderID: parsedOrderID,
		PaymentID:       parsedPaymentID,
	}, nil
}

type TBankPaymentNotification struct {
	TerminalKey string
	OrderID     string
	Success     bool
	Status      string
	PaymentID   string
	ErrorCode   string
	Amount      int
	Token       string

	Message  *string
	Details  *string
	Pan      *string
	ExpDate  *string
	RebillID *int64
	CardID   *int64
}

func NewTBankPaymentNotification(
	tKey, orderID, status, errCode, token string,
	payID int64, amount int32,
	success bool,
	message, details, pan, expDate *string,
	rebillID, cardID *int64,
) *TBankPaymentNotification {
	return &TBankPaymentNotification{
		TerminalKey: tKey,
		OrderID:     orderID,
		Status:      status,
		PaymentID:   strconv.FormatInt(payID, 10),
		ErrorCode:   errCode,
		Token:       token,
		Amount:      int(amount),
		Success:     success,
		Message:     message,
		Details:     details,
		Pan:         pan,
		ExpDate:     expDate,
		RebillID:    rebillID,
		CardID:      cardID,
	}
}

func (r TBankPaymentNotification) ToMap() map[string]string {
	m := map[string]string{
		"TerminalKey": r.TerminalKey,
		"Amount":      strconv.Itoa(r.Amount),
		"OrderId":     r.OrderID,
		"Success":     strconv.FormatBool(r.Success),
		"Status":      r.Status,
		"PaymentId":   r.PaymentID,
		"ErrorCode":   r.ErrorCode,
	}

	if r.Message != nil {
		m["Message"] = *r.Message
	}
	if r.Details != nil {
		m["Details"] = *r.Details
	}
	if r.Pan != nil {
		m["Pan"] = *r.Pan
	}
	if r.ExpDate != nil {
		m["ExpDate"] = *r.ExpDate
	}
	if r.RebillID != nil {
		m["RebillId"] = strconv.FormatInt(*r.RebillID, 10)
	}
	if r.CardID != nil {
		m["CardId"] = strconv.FormatInt(*r.CardID, 10)
	}
	return m
}

type TBankProviderStatus string

// статусы
const (
	TbankProviderStatusNew             TBankProviderStatus = "NEW"
	TbankProviderStatusFormShowed      TBankProviderStatus = "FORM_SHOWED"
	TbankProviderStatusConfirmed       TBankProviderStatus = "CONFIRMED"
	TbankProviderStatusRejected        TBankProviderStatus = "REJECTED"
	TbankProviderStatusCanceled        TBankProviderStatus = "CANCELED"
	TbankProviderStatusDeadlineExpired TBankProviderStatus = "DEADLINE_EXPIRED"
	TbankProviderStatusAuthorized      TBankProviderStatus = "AUTHORIZED"
	TbankProviderStatusReversed        TBankProviderStatus = "REVERSED"
	TbankProviderStatusRefunded        TBankProviderStatus = "REFUNDED"
)

func NewTBankProviderStatus(s string) (TBankProviderStatus, error) {
	status := TBankProviderStatus(s)
	switch status {
	case TbankProviderStatusNew, TbankProviderStatusConfirmed, TbankProviderStatusFormShowed,
		TbankProviderStatusRejected, TbankProviderStatusCanceled, TbankProviderStatusDeadlineExpired,
		TbankProviderStatusAuthorized, TbankProviderStatusReversed, TbankProviderStatusRefunded:
		return status, nil
	default:
		return "", fmt.Errorf("unknown provider status: %q", s)
	}
}

func (s TBankProviderStatus) ToPaymentStatus() (PaymentStatus, error) {
	switch s {
	case TbankProviderStatusConfirmed:
		return PaymentStatusCompleted, nil
	case TbankProviderStatusRejected:
		return PaymentStatusFailed, nil
	case TbankProviderStatusDeadlineExpired:
		return PaymentStatusExpired, nil
	case TbankProviderStatusCanceled, TbankProviderStatusReversed:
		return PaymentStatusCancelled, nil
	case TbankProviderStatusRefunded:
		return PaymentStatusRefunded, nil
	default:
		// NEW, FORM_SHOWED, AUTHORIZED — игнорируем
		return "", fmt.Errorf("failed mapping from %s provider status to payment status (unhandled status)", s)
	}
}

type TbankCompleteParams struct {
	IdempotencyKey    string
	ProviderPaymentID string
	Amount            int
	NewStatus         PaymentStatus
	CurrentStatus     PaymentStatus
	OpType            OpType
}

type TbankUpdateStatusParams struct {
	IdempotencyKey string
	NewStatus      PaymentStatus
	CurrentStatus  PaymentStatus
}

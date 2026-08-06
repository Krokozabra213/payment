package domain

import (
	"fmt"

	"github.com/Krokozabra213/common/types"
	"github.com/google/uuid"
)

type CurrencyType string

const (
	CurrencyTypeRUB     CurrencyType = "RUB"
	CurrencyTypeTGStars CurrencyType = "XTR"
)

func NewCurrencyType(s string) (CurrencyType, error) {
	currency := CurrencyType(s)
	switch currency {
	case CurrencyTypeRUB, CurrencyTypeTGStars:
		return currency, nil
	default:
		return "", fmt.Errorf("unknown currency type: %q", s)
	}
}

type TGInit struct {
	UserID   uuid.UUID
	Amount   types.PositiveInt[int]
	Currency CurrencyType
	Desc     types.NonEmptyString
	Title    types.NonEmptyString
}

func NewTGInitPayload(
	userID string,
	amount int,
	currency string,
	desc string,
	title string,
) (*TGInit, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid userID: %w", err)
	}

	parsedAmount, err := types.NewPositiveInt(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	parsedCurrency, err := NewCurrencyType(currency)
	if err != nil {
		return nil, err
	}

	parsedDesc, err := types.NewNonEmptyString(desc)
	if err != nil {
		return nil, fmt.Errorf("invalid description: %w", err)
	}

	parsedTitle, err := types.NewNonEmptyString(title)
	if err != nil {
		return nil, fmt.Errorf("invalid title: %w", err)
	}

	return &TGInit{
		UserID:   parsedUserID,
		Amount:   parsedAmount,
		Currency: parsedCurrency,
		Desc:     parsedDesc,
		Title:    parsedTitle,
	}, nil
}

type TGInitResult struct {
	PaymentURL types.URL
}

type PreCheckoutPayload struct {
	PreCheckoutQueryID types.NonEmptyString
	Ok                 bool
	ErrorMsg           string
}

type ConfirmedStarTx struct {
	UserID  types.PositiveInt[int64]
	OrderID types.NonEmptyString
}

func NewConfirmedStarTx(userID int64, orderID string) (ConfirmedStarTx, error) {
	uID, err := types.NewPositiveInt(userID)
	if err != nil {
		return ConfirmedStarTx{}, err
	}

	oID, err := types.NewNonEmptyString(orderID)
	if err != nil {
		return ConfirmedStarTx{}, err
	}

	return ConfirmedStarTx{
		UserID:  uID,
		OrderID: oID,
	}, nil
}

type TGPrecheckoutApproveParams struct {
	IdempotencyKey string
	TGUserID       int64
	Amount         int
	Currency       string
}

// TODO: добавить валидацию
type User struct {
	ID        int64
	IsBot     bool
	FirstName string
}

func NewUser(id int64, isBot bool, firstName string) User {
	return User{
		ID:        id,
		IsBot:     isBot,
		FirstName: firstName,
	}
}

type PreCheckoutQuery struct {
	ID             types.NonEmptyString
	From           User
	Currency       string
	TotalAmount    int
	InvoicePayload string
}

// TODO: расширить валидацию
func NewPreCheckoutQuery(id, payload string, amount int, currency string, from User) (PreCheckoutQuery, error) {
	precheckoutID, err := types.NewNonEmptyString(id)
	if err != nil {
		return PreCheckoutQuery{}, fmt.Errorf("invalid precheckoutID: %w", err)
	}

	return PreCheckoutQuery{
		ID:             precheckoutID,
		Currency:       currency,
		TotalAmount:    amount,
		InvoicePayload: payload,
		From:           from,
	}, nil
}

type TGCompletePaymentParams struct {
	IdempotencyKey          string
	TelegramPaymentChargeID string
	Amount                  int
	Currency                string
}

type SuccessfulPayment struct {
	Currency                string
	InvoicePayload          string
	TelegramPaymentChargeID string
	ProviderPaymentChargeID string
	TotalAmount             int
}

func NewSuccessfulPayment(currency, payload, tgChargeID, providerChargeID string, amount int) SuccessfulPayment {
	return SuccessfulPayment{
		Currency:                currency,
		InvoicePayload:          payload,
		TelegramPaymentChargeID: tgChargeID,
		ProviderPaymentChargeID: providerChargeID,
		TotalAmount:             amount,
	}
}

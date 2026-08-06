package tbank

import "strconv"

const (
	TerminalKeyField     = "TerminalKey"
	AmountField          = "Amount"
	OrderIDField         = "OrderId"
	DescriptionField     = "Description"
	PaymentIDField       = "PaymentId"
	NotificationURLField = "NotificationURL"
)

type InitRequest struct {
	TerminalKey     string `json:"TerminalKey"`
	Amount          int    `json:"Amount"`
	OrderID         string `json:"OrderId"`
	Description     string `json:"Description,omitempty"`
	Token           string `json:"Token"`
	NotificationURL string `json:"NotificationURL,omitempty"`
}

func NewInitRequest(terminalKey, orderID, desc, notificationUrl string, amount int) *InitRequest {
	return &InitRequest{
		TerminalKey:     terminalKey,
		Amount:          amount,
		OrderID:         orderID,
		Description:     desc,
		NotificationURL: notificationUrl,
	}
}

func (r *InitRequest) ToMap() map[string]string {
	m := map[string]string{
		TerminalKeyField:     r.TerminalKey,
		AmountField:          strconv.Itoa(r.Amount),
		OrderIDField:         r.OrderID,
		DescriptionField:     r.Description,
		NotificationURLField: r.NotificationURL,
	}
	return m
}

func (r *InitRequest) addToken(token string) {
	r.Token = token
}

type CancelRequest struct {
	TerminalKey string `json:"TerminalKey"`
	PaymentID   string `json:"PaymentId"`
	Token       string `json:"Token"`
}

func NewCancelRequest(tKey, payID string) *CancelRequest {
	return &CancelRequest{
		TerminalKey: tKey,
		PaymentID:   payID,
	}
}

func (r *CancelRequest) addToken(token string) {
	r.Token = token
}

func (r CancelRequest) ToMap() map[string]string {
	m := map[string]string{
		TerminalKeyField: r.TerminalKey,
		PaymentIDField:   r.PaymentID,
	}
	return m
}

type StateRequest struct {
	TerminalKey string `json:"TerminalKey"`
	PaymentID   string `json:"PaymentId"`
	Token       string `json:"Token"`
}

func (r *StateRequest) addToken(token string) {
	r.Token = token
}

func (r StateRequest) ToMap() map[string]string {
	m := map[string]string{
		TerminalKeyField: r.TerminalKey,
		PaymentIDField:   r.PaymentID,
	}
	return m
}

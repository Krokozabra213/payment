package domain

type BalanceEvent struct {
	EventKey string
	Payload  []byte
}

type BalancePayload struct {
	OperationID string `json:"operation_id"`
	UserID      string `json:"user_id"`
	PaymentID   string `json:"payment_id"`
	Type        string `json:"type"`
	Amount      int    `json:"amount"`
}

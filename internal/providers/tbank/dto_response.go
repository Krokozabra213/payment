package tbank

type InitResponse struct {
	Success     bool   `json:"Success"`
	ErrorCode   string `json:"ErrorCode"`
	TerminalKey string `json:"TerminalKey"`
	Status      string `json:"Status"`
	PaymentID   string `json:"PaymentId"`
	OrderID     string `json:"OrderId"`
	Amount      int    `json:"Amount"`
	PaymentURL  string `json:"PaymentURL,omitempty"`
}

type CancelResponse struct {
	TerminalKey string `json:"TerminalKey"`
	OrderID     string `json:"OrderId"`
	Success     bool   `json:"Success"`
	Status      string `json:"Status"`
	PaymentID   string `json:"PaymentId"`
	ErrorCode   string `json:"ErrorCode"`
	Message     string `json:"Message"`
}

type TinkoffWebhookPayload struct {
	TerminalKey string `json:"TerminalKey"`
	Amount      int    `json:"Amount"`
	OrderID     string `json:"OrderId"`
	Success     bool   `json:"Success"`
	Status      string `json:"Status"`
	ErrorCode   string `json:"ErrorCode"`
	Message     string `json:"Message"`
	Details     string `json:"Details"`
	RebillID    int64  `json:"RebillId"`
	CardID      int64  `json:"CardId"`
	Pan         string `json:"Pan"`
	ExpDate     string `json:"ExpDate"`
	PaymentID   int    `json:"PaymentId"`
	Token       string `json:"Token"`
}

type StateResponse struct {
	Success     bool         `json:"Success"`
	ErrorCode   string       `json:"ErrorCode"`
	Message     string       `json:"Message,omitempty"`
	TerminalKey string       `json:"TerminalKey"`
	Status      string       `json:"Status"`
	PaymentID   string       `json:"PaymentId"`
	OrderID     string       `json:"OrderId"`
	Amount      int          `json:"Amount"`
	Params      []StateParam `json:"Params,omitempty"`
}

type StateParam struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

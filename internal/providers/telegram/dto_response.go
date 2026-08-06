package tg

type CreateInvoiceLinkResponse struct {
	OK          bool   `json:"ok"`
	Result      string `json:"result"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
}

type RefundStarPaymentResponse struct {
	OK          bool   `json:"ok"`
	Result      bool   `json:"result"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
}

type AnswerPreCheckoutQueryResponse struct {
	OK          bool   `json:"ok"`
	Result      bool   `json:"result"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
}

type StarTransactionsResponse struct {
	Transactions []StarTransaction `json:"transactions"`
}

type StarTransaction struct {
	ID     string              `json:"id"`
	Amount int                 `json:"amount"`
	Date   int                 `json:"date"`
	Source *TransactionPartner `json:"source,omitempty"`
}

type TransactionPartner struct {
	Type           string `json:"type"`
	User           *User  `json:"user,omitempty"`
	InvoicePayload string `json:"invoice_payload,omitempty"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

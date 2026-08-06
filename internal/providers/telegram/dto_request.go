package tg

type LabeledPrice struct {
	Label  string `json:"label"`
	Amount int    `json:"amount"`
}

func NewLabeledPrice(label string, amount int) LabeledPrice {
	return LabeledPrice{
		Label:  label,
		Amount: amount,
	}
}

type CreateInvoiceLinkRequest struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Payload     string         `json:"payload"`
	Currency    string         `json:"currency"`
	Prices      []LabeledPrice `json:"prices"`
}

func NewCreateInvoiceLinkRequest(title, desc, payload, currency string, prices []LabeledPrice) CreateInvoiceLinkRequest {
	return CreateInvoiceLinkRequest{
		Title:       title,
		Description: desc,
		Payload:     payload,
		Currency:    currency,
		Prices:      prices,
	}
}

type RefundStarPaymentRequest struct {
	UserID                  int64  `json:"user_id"`
	TelegramPaymentChargeID string `json:"telegram_payment_charge_id"`
}

func NewRefundStarPaymentRequest(userID int64, chargeID string) RefundStarPaymentRequest {
	return RefundStarPaymentRequest{
		UserID:                  userID,
		TelegramPaymentChargeID: chargeID,
	}
}

type AnswerPreCheckoutQueryRequest struct {
	PreCheckoutQueryID string `json:"pre_checkout_query_id"`
	OK                 bool   `json:"ok"`
	ErrorMessage       string `json:"error_message,omitempty"`
}

func NewAnswerPreCheckoutQueryRequest(queryID, errorMsg string, ok bool) AnswerPreCheckoutQueryRequest {
	return AnswerPreCheckoutQueryRequest{
		PreCheckoutQueryID: queryID,
		ErrorMessage:       errorMsg,
		OK:                 ok,
	}
}

type GetStarTransactionsRequest struct {
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

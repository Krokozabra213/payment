package pgRepo

import "errors"

var (
	ErrInvalidPaymentState       = errors.New("invalid payment state")
	ErrProviderMismatch          = errors.New("provider mismatch")
	ErrAlreadyPaid               = errors.New("already paid")
	ErrInvalidPaymentStatus      = errors.New("invalid payment status")
	ErrAmountMismatch            = errors.New("amount mismatch")
	ErrCurrencyMismatch          = errors.New("currency mismatch")
	ErrPaymentProcessed          = errors.New("payment is processing")
	ErrAlreadyProcessed          = errors.New("already processed")
	ErrProviderPaymentIDMismatch = errors.New("provider payment id mismatch")
)

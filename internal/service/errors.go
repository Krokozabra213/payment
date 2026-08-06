package svc

import "errors"

var (
	ErrProviderMismatch            = errors.New("provider mismatch")
	ErrIdempotencyConflict         = errors.New("idempotency key reused with different payload")
	ErrPaymentInitializing         = errors.New("payment is initializing")
	ErrPaymentAlreadyPaid          = errors.New("payment already paid")
	ErrPaymentExpired              = errors.New("payment expired")
	ErrInvalidPaymentState         = errors.New("invalid payment state")
	ErrTbankInvalidToken           = errors.New("invalid tbank webhook token")
	ErrUnexpectedNotificationState = errors.New("unexpected notification state: success flag mismatch")
	ErrInvalidPaymentStatus        = errors.New("invalid payment status")
)

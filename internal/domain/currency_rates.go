package domain

import "time"

type CurrencySourceName string

const (
	CurrencySourceCBR CurrencySourceName = "CBR"
)

type CurrencyCode string

const (
	CurrencyUSD CurrencyCode = "USD"
	CurrencyEUR CurrencyCode = "EUR"
)

type CurrencyRate struct {
	Code       CurrencyCode
	RubRate    int64
	SourceName CurrencySourceName
	SourceAt   time.Time
}

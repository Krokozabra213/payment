package telemetry

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	ServiceMeter = otel.Meter("payment/internal/service")
	ServiceTrace = otel.Tracer("payment/internal/service")
)

type PaymentMetrics struct {
	PaymentInProcess metric.Int64UpDownCounter
	PaymentCreated   metric.Int64Counter
	PaymentCancelled metric.Int64Counter
	PaymentRefunded  metric.Int64Counter
	PaymentCompleted metric.Int64Counter
	PaymentSucceeded metric.Int64Counter
	PaymentFailed    metric.Int64Counter
}

func NewPaymentMetrics(meter metric.Meter) *PaymentMetrics {
	m := &PaymentMetrics{}
	var err error

	m.PaymentInProcess, err = meter.Int64UpDownCounter(
		"payment.in_process",
		metric.WithDescription("Number of payments currently being processed"),
		metric.WithUnit("{payment}"),
	)
	must(err)

	m.PaymentCreated, err = meter.Int64Counter(
		"payment.created",
		metric.WithDescription("Total number of created payments"),
		metric.WithUnit("{payment}"),
	)
	must(err)

	m.PaymentCancelled, err = meter.Int64Counter(
		"payment.cancelled",
		metric.WithDescription("Total number of cancelled payments"),
		metric.WithUnit("{payment}"),
	)
	must(err)

	m.PaymentRefunded, err = meter.Int64Counter(
		"payment.refunded",
		metric.WithDescription("Total number of refunded payments"),
		metric.WithUnit("{payment}"),
	)
	must(err)

	m.PaymentCompleted, err = meter.Int64Counter(
		"payment.completed",
		metric.WithDescription("Total number of completed payments"),
		metric.WithUnit("{payment}"),
	)
	must(err)

	m.PaymentFailed, err = meter.Int64Counter(
		"payment.failed",
		metric.WithDescription("Total number of failed payments"),
		metric.WithUnit("{payment}"),
	)
	must(err)

	m.PaymentSucceeded, err = meter.Int64Counter(
		"payment.succeeded",
		metric.WithDescription("Total number of succeeded payments"),
		metric.WithUnit("{payment}"),
	)
	must(err)

	return m
}

func must(err error) {
	if err != nil {
		panic(fmt.Sprintf("failed to create metric: %v", err))
	}
}

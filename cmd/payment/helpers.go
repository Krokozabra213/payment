package main

import (
	"context"
	"log/slog"

	"github.com/Krokozabra213/gargantua_common/pkg/telemetry"
	"github.com/GargantuaLabs/payment/internal/config"
)

func setupTelemetry(ctx context.Context, cfg *config.Config) (*telemetry.Telemetry, error) {
	return telemetry.Setup(ctx, telemetry.Config{
		Logs:              cfg.Telemetry.Logs,
		Metrics:           cfg.Telemetry.Metrics,
		Traces:            cfg.Telemetry.Traces,
		ServiceName:       cfg.App.Name,
		ServiceVersion:    cfg.App.Version,
		Environment:       cfg.App.ENV,
		OTELEndpoint:      cfg.Telemetry.EndPoint,
		TraceSampleRate:   cfg.Telemetry.TracesSampleRate,
		LogToStdout:       cfg.Telemetry.LogToStdout,
		StdoutLogLevel:    slog.Level(cfg.Telemetry.StdoutLogLevel),
		SampleLogs:        cfg.Telemetry.SampleLogs,
		UnsampledLogLevel: slog.Level(cfg.Telemetry.UnsampledLogLevel),
		Insecure:          cfg.Telemetry.Insecure,
	})
}

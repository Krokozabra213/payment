package tg

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/GargantuaLabs/payment/internal/config"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const pathName = "internal/providers/telegram"

type Provider struct {
	botToken   string
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewProvider(cfg *config.TelegramConfig, log *slog.Logger) *Provider {
	return &Provider{
		logger:   log.With(slog.String("pathName", pathName)),
		botToken: cfg.BotToken,
		baseURL:  fmt.Sprint(cfg.BaseURL + cfg.BotToken),

		httpClient: &http.Client{
			Timeout: cfg.Timeout,

			Transport: otelhttp.NewTransport(
				&http.Transport{
					MaxIdleConns:        cfg.MaxIdleConns,
					MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
					IdleConnTimeout:     cfg.IdleConnTimeout,
				},

				otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
					return "telegram " + r.Method + " " + r.URL.Path
				}),
			),
		},
	}
}

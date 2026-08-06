package tbank

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/GargantuaLabs/payment/internal/config"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const pathName = "internal/providers/tbank"

type Provider struct {
	terminalKey string
	password    string
	baseURL     string
	httpClient  *http.Client
	logger      *slog.Logger
}

func NewProvider(cfg *config.TinkoffConfig, log *slog.Logger) *Provider {
	return &Provider{
		logger:      log.With(slog.String("pathName", pathName)),
		terminalKey: cfg.TerminalKey,
		password:    cfg.Password,
		baseURL:     cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: otelhttp.NewTransport(
				&http.Transport{
					MaxIdleConns:        cfg.MaxIdleConns,
					MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
					IdleConnTimeout:     cfg.IdleConnTimeout,
				},
				
				otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
					return "tbank " + r.Method + " " + r.URL.Path
				}),
			),
		},
	}
}

func (p *Provider) generateToken(params map[string]string) string {
	keys := make([]string, 0, len(params)+1)
	for k := range params {
		keys = append(keys, k)
	}
	keys = append(keys, "Password")

	sort.Strings(keys)

	var sb strings.Builder
	sb.Grow(256)

	for _, k := range keys {
		var val string
		if k == "Password" {
			val = p.password
		} else {
			val = params[k]
		}
		sb.WriteString(val)
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}

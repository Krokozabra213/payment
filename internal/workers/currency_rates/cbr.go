package currency_rates

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/GargantuaLabs/payment/internal/domain"
)

const cbrURL = "https://www.cbr-xml-daily.ru/daily_json.js"

type CBRSource struct {
	client *http.Client
}

func NewCBRSource(client *http.Client) *CBRSource {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &CBRSource{client: client}
}

func (s *CBRSource) Fetch(ctx context.Context, codes []domain.CurrencyCode) ([]domain.CurrencyRate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cbrURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var data cbrResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	sourceAt, err := time.Parse(time.RFC3339, data.Date)
	if err != nil {
		return nil, fmt.Errorf("parse source date: %w", err)
	}

	result := make([]domain.CurrencyRate, 0, len(codes))

	for _, code := range codes {
		key := strings.ToUpper(string(code))
		v, ok := data.Valute[key]
		if !ok {
			return nil, fmt.Errorf("currency %s not found in source", code)
		}
		if v.Nominal <= 0 {
			return nil, fmt.Errorf("invalid nominal for %s", code)
		}

		rubPerUnit := v.Value / float64(v.Nominal)

		rubRateKopecks := int64(math.Round(rubPerUnit * 100))

		result = append(result, domain.CurrencyRate{
			Code:       code,
			RubRate:    rubRateKopecks,
			SourceName: domain.CurrencySourceCBR,
			SourceAt:   sourceAt,
		})
	}

	return result, nil
}

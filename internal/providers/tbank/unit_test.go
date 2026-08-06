package tbank

import "testing"

func TestGenerateToken(t *testing.T) {
	tests := []struct {
		name     string
		password string
		params   map[string]string
		want     string
	}{
		{
			name:     "документация T-Банка",
			password: "11111111111111",
			params: map[string]string{
				"TerminalKey": "MerchantTerminalKey",
				"Amount":      "19200",
				"OrderId":     "00000",
				"Description": "Подарочная карта на 1000 рублей",
			},
			want: "72dd466f8ace0a37a1f740ce5fb78101712bc0665d91a8108c7c8a0ccd426db2",
		},
		{
			name:     "тестовый терминал",
			password: "TinkoffBankTest",
			params: map[string]string{
				"TerminalKey": "TinkoffBankTest",
				"Amount":      "1000",
				"OrderId":     "15",
				"Description": "test",
			},
			want: "8ee842bfe2499994d703f7e710275fc7eda03f7a661442bea9991e2b523c0fb7",
		},
		{
			name:     "без description",
			password: "TinkoffBankTest",
			params: map[string]string{
				"TerminalKey": "TinkoffBankTest",
				"Amount":      "1000",
				"OrderId":     "15",
			},
			want: "5e71b996239804bff41d01d727e90f83059b07ff46b78c45a63f16ae88a56ec6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{password: tt.password}

			got := p.generateToken(tt.params)

			if got != tt.want {
				t.Errorf("generateToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

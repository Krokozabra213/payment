package config

import "time"

type TelegramConfig struct {
	BaseURL             string        `yaml:"base_url" env:"TELEGRAM_BASE_URL" env-default:"https://api.telegram.org/bot"`
	BotToken            string        `yaml:"bot_token" env:"TELEGRAM_BOT_TOKEN" env-required:"true"`
	Timeout             time.Duration `yaml:"timeout" env-default:"10s"`
	MaxIdleConns        int           `yaml:"max_idle_conns" env-default:"100"`
	MaxIdleConnsPerHost int           `yaml:"max_idle_conns_per_host" env-default:"10"`
	IdleConnTimeout     time.Duration `yaml:"idle_conn_timeout" env-default:"90s"`
}

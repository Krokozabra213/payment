package config

import "time"

type TinkoffConfig struct {
	TerminalKey         string        `yaml:"terminal_key" env:"TINKOFF_TERMINAL_KEY" env-required:"true"`
	Password            string        `yaml:"password"      env:"TINKOFF_PASSWORD"      env-required:"true"`
	BaseURL             string        `yaml:"base_url"      env:"TINKOFF_BASE_URL"      env-required:"true"`
	NotificationURL     string        `yaml:"notification_url"      env:"TINKOFF_NOTIFICATION_URL"`
	Timeout             time.Duration `yaml:"timeout" env-default:"10s"`
	MaxIdleConns        int           `yaml:"max_idle_conns" env-default:"100"`
	MaxIdleConnsPerHost int           `yaml:"max_idle_conns_per_host" env-default:"10"`
	IdleConnTimeout     time.Duration `yaml:"idle_conn_timeout" env-default:"90s"`
}

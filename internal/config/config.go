package config

import (
	"fmt"
	"log"
	"os"

	commonconfig "github.com/Krokozabra213/gargantua_common/pkg/config"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig                    `yaml:"app"`
	HTTP      commonconfig.HTTPConfig      `yaml:"http"`
	GRPC      GRPCConfig                   `yaml:"grpc"`
	Postgres  commonconfig.PostgresConfig  `yaml:"postgres"`
	Logger    commonconfig.SlogConfig      `yaml:"slog"`
	Tinkoff   TinkoffConfig                `yaml:"tinkoff"`
	Telegram  TelegramConfig               `yaml:"telegram"`
	Producer  ProducerConfig               `yaml:"producer"`
	Telemetry commonconfig.TelemetryConfig `yaml:"telemetry"`
}

func Init() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		log.Print(err)
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	configFile := fmt.Sprintf("configs/%s.yml", env)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s (ENV=%s)", configFile, env)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configFile, &cfg); err != nil {
		return nil, err
	}

	err = cfg.Validate()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	var err error
	err = c.Logger.Validate()
	if err != nil {
		return err
	}

	return c.Telemetry.Validate()
}

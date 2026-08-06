package config

import "time"

type ProducerConfig struct {
	Brokers []string `env:"KAFKA_BROKERS" env-required:"true"`

	DialTimeout  time.Duration `yaml:"dialTimeout" env-default:"5s"`
	ReadTimeout  time.Duration `yaml:"readTimeout" env-default:"10s"`
	WriteTimeout time.Duration `yaml:"writeTimeout" env-default:"10s"`

	MaxOpenRequests int           `yaml:"maxOpenRequests" env-default:"1"`
	MaxRetries      int           `yaml:"maxRetries" env-default:"5"`
	RetryBackoff    time.Duration `yaml:"retryBackoff" env-default:"300ms"`

	Version  string `yaml:"version" env-default:"4.1.2"`
	ClientID string `yaml:"clientID" env-default:"payment-service"`

	PaymentTopic string        `yaml:"paymentTopic" env-default:"payments"`
	BatchSize    int           `yaml:"batchSize" env-default:"100"`
	PollInterval time.Duration `yaml:"pollInterval" env-default:"1s"`
	Lease        time.Duration `yaml:"lease" env-default:"1m"`
	SendTimeout  time.Duration `yaml:"sendTimeout" env-default:"10s"`
	DBTimeout    time.Duration `yaml:"dbTimeout" env-default:"5s"`
	MaxAttempts  int           `yaml:"maxAttempts" env-default:"10"`
	ErrorBackoff time.Duration `yaml:"errorBackoff" env-default:"2s"`
}

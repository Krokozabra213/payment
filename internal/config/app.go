package config

type AppConfig struct {
	Name    string `yaml:"name" env-default:"gateway-api"`
	Version string `yaml:"version" env-default:"v1.0.0"`
	ENV     string `env:"ENV" env-default:"prod"`
}

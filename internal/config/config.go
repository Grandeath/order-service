package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

var (
	errNoConfigName = errors.New("CONFIG_NAME env is not set")
)

type (
	Config struct {
		ApiServer       ApiServer
		TechnicalServer TechnicalServer
		DB              DB
		Kafka           Kafka
		Cognito         Cognito
		LogLevel        string `env:"LOG_LEVEL"`
	}

	ApiServer struct {
		Port         string        `env:"API_PORT"`
		ReadTimeout  time.Duration `env:"API_READ_TIMEOUT"`
		WriteTimeout time.Duration `env:"API_WRITE_TIMEOUT"`
	}

	TechnicalServer struct {
		Port         string        `env:"TECHNICAL_PORT"`
		ReadTimeout  time.Duration `env:"TECHNICAL_READ_TIMEOUT"`
		WriteTimeout time.Duration `env:"TECHNICAL_WRITE_TIMEOUT"`
	}

	DB struct {
		DSN             Secret        `env:"DB_DSN"`
		MaxConns        int32         `env:"DB_MAX_CONNS"        envDefault:"10"`
		ConnectTimeout  time.Duration `env:"DB_CONNECT_TIMEOUT"  envDefault:"5s"`
	}

	Kafka struct {
		Enabled          bool   `env:"KAFKA_ENABLED"           envDefault:"false"`
		URL              string `env:"KAFKA_URL"               envDefault:""`
		Topic            string `env:"KAFKA_TOPIC"             envDefault:"order.events"`
		Compression      bool   `env:"KAFKA_COMPRESSION"       envDefault:"true"`
		MetricsNameSpace string `env:"KAFKA_METRICS_NAMESPACE" envDefault:"order_events"`
	}

	Cognito struct {
		Enabled     bool   `env:"COGNITO_ENABLED"       envDefault:"false"`
		Region      string `env:"COGNITO_REGION"        envDefault:""`
		UserPoolID  string `env:"COGNITO_USER_POOL_ID"  envDefault:""`
		AppClientID string `env:"COGNITO_APP_CLIENT_ID" envDefault:""`
	}
)

type Secret string

func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return "*****"
}

func (s Secret) Secret() string {
	return string(s)
}

func InitConfig[T any]() (*T, error) {
	configName := os.Getenv("CONFIG_NAME")
	if configName == "" {
		return nil, errNoConfigName
	}

	options := defaultOptions()
	config := new(T)

	if err := godotenv.Load(configName); err != nil {
		return nil, fmt.Errorf("unable to load .env file: %w", err)
	}
	if err := env.ParseWithOptions(config, options); err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}

	slog.Info("config:", "cfg", display(config))

	return config, nil
}

func defaultOptions() env.Options {
	return env.Options{
		RequiredIfNoDef: true,
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeFor[Secret](): func(v string) (any, error) {
				return Secret(v), nil
			},
		},
	}
}

func display(cfg any) string {
	var sb strings.Builder
	iterator := reflect.Indirect(reflect.ValueOf(cfg))
	for i := 0; i < iterator.NumField(); i++ {
		name := iterator.Type().Field(i).Name
		value := iterator.Field(i)
		fmt.Fprintf(&sb, "%s = %v | ", name, value)
	}
	return sb.String()
}

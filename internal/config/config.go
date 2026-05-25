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

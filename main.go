package main

import (
	"context"
	"log"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/Grandeath/order-service/internal/config"
	"github.com/Grandeath/order-service/internal/server"
	"golang.org/x/sync/errgroup"
)

func main() {
	cfg, err := initConfig()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := runBackgroundWorkers(ctx, cfg); err != nil {
		log.Fatal(err)
	}

	slog.Info("application - done")
}

func initConfig() (*config.Config, error) {
	cfg, err := config.InitConfig[config.Config]()
	if err != nil {
		return nil, err
	}
	config.InitLogger(cfg.LogLevel)
	return cfg, nil
}

func runBackgroundWorkers(ctx context.Context, cfg *config.Config) error {
	workers, ctx := errgroup.WithContext(ctx)

	workers.Go(server.Worker(ctx, server.TechConfig{
		Port: cfg.TechnicalServer.Port,
	}, server.TechnicalEndpoints()))

	return workers.Wait()
}

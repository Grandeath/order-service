package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/Grandeath/order-service/internal/auth"
	"github.com/Grandeath/order-service/internal/config"
	"github.com/Grandeath/order-service/internal/db"
	"github.com/Grandeath/order-service/internal/metrics"
	"github.com/Grandeath/order-service/internal/order/api"
	"github.com/Grandeath/order-service/internal/order/events"
	"github.com/Grandeath/order-service/internal/order/repository"
	"github.com/Grandeath/order-service/internal/order/service"
	"github.com/Grandeath/order-service/internal/producer"
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
	pool, err := db.NewPool(ctx, db.Config{
		DSN:            cfg.DB.DSN.Secret(),
		MaxConns:       cfg.DB.MaxConns,
		ConnectTimeout: cfg.DB.ConnectTimeout,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	notifier, err := producer.NewEventNotifier(producer.EventNotifierConfig{
		Enabled:          cfg.Kafka.Enabled,
		URL:              cfg.Kafka.URL,
		Topic:            cfg.Kafka.Topic,
		Registerer:       metrics.GetRegisterer(),
		MetricsNameSpace: cfg.Kafka.MetricsNameSpace,
		Compression:      cfg.Kafka.Compression,
	})
	if err != nil {
		return err
	}
	defer notifier.Close()

	repo := repository.NewPostgres(pool)
	svc := service.New(repo, service.WithPublisher(events.NewKafkaPublisher(notifier)))
	apiHandler := api.NewHandler(svc)

	authMW, err := buildAuthMiddleware(ctx, cfg)
	if err != nil {
		return err
	}

	apiMiddlewares := apiHandler.Middlewares(authMW...)

	workers, ctx := errgroup.WithContext(ctx)

	workers.Go(server.Worker(ctx, &server.Config{
		Port:         cfg.ApiServer.Port,
		ReadTimeout:  cfg.ApiServer.ReadTimeout,
		WriteTimeout: cfg.ApiServer.WriteTimeout,
	}, apiHandler.Endpoints(), apiMiddlewares))

	workers.Go(server.Worker(ctx, &server.Config{
		Port:         cfg.TechnicalServer.Port,
		ReadTimeout:  cfg.TechnicalServer.ReadTimeout,
		WriteTimeout: cfg.TechnicalServer.WriteTimeout,
	}, server.TechnicalEndpoints(), server.TechMiddlewares()))

	return workers.Wait()
}

// buildAuthMiddleware returns the list of auth middlewares to splice into the
// API chain. Empty when Cognito is disabled — Handler.Middlewares handles the
// zero-length case correctly.
func buildAuthMiddleware(ctx context.Context, cfg *config.Config) ([]func(http.Handler) http.Handler, error) {
	if !cfg.Cognito.Enabled {
		return nil, nil
	}

	verifier, err := auth.NewCognitoVerifier(ctx, auth.CognitoConfig{
		Region:      cfg.Cognito.Region,
		UserPoolID:  cfg.Cognito.UserPoolID,
		AppClientID: cfg.Cognito.AppClientID,
	})
	if err != nil {
		return nil, err
	}
	slog.Info("cognito auth enabled", "userPoolId", cfg.Cognito.UserPoolID, "region", cfg.Cognito.Region)
	return []func(http.Handler) http.Handler{auth.Middleware(verifier)}, nil
}

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
)

// Endpoint represents needed parameters for endpoint
type Endpoint struct {
	Path    string
	Handler http.HandlerFunc
}

func Worker(ctx context.Context, cfg TechConfig, endpoints []Endpoint) func() error {
	return func() error {
		server := createServer(cfg, endpoints)
		group, ctx := errgroup.WithContext(ctx)

		group.Go(listenAndServeWorker(server))
		group.Go(shutdownWorker(ctx, server))

		if err := group.Wait(); err != nil {
			return fmt.Errorf("server workers group: %w", err)
		}

		return nil
	}
}

func createServer(cfg TechConfig, endpoints []Endpoint) *http.Server {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = defaultTimeout
	}

	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaultTimeout
	}

	mux := http.NewServeMux()
	for _, endpoint := range endpoints {
		mux.HandleFunc(endpoint.Path, endpoint.Handler)
	}

	mux.HandleFunc("/", index(endpoints))

	return &http.Server{
		Addr:         fmt.Sprint(":", cfg.Port),
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
}

func shutdownWorker(ctx context.Context, server *http.Server) func() error {
	return func() error {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdownWorker: %w", err)
		}

		return nil
	}
}

func listenAndServeWorker(server *http.Server) func() error {
	return func() error {
		if err := server.ListenAndServe(); err != nil {
			return fmt.Errorf("listenAndServeWorker: %w", err)
		}

		return nil
	}
}

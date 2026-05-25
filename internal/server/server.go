package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"
)

type Endpoint struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

func Worker(ctx context.Context, cfg *Config, endpoints []*Endpoint, middlewares []func(http.Handler) http.Handler) func() error {
	return func() error {
		server := createServer(cfg, endpoints, middlewares)
		group, ctx := errgroup.WithContext(ctx)

		group.Go(listenAndServeWorker(server))
		group.Go(shutdownWorker(ctx, server))

		if err := group.Wait(); err != nil {
			return fmt.Errorf("server workers group: %w", err)
		}

		return nil
	}
}

// Router builds the chi router that the Worker would serve. Exported so tests
// can drive the same routing/middleware stack as production without spinning
// up a TCP listener.
func Router(endpoints []*Endpoint, middlewares []func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	if len(middlewares) > 0 {
		r.Use(middlewares...)
	}

	r.MethodNotAllowed(methodNotAllowed)
	r.NotFound(notFound)

	for _, endpoint := range endpoints {
		r.Method(endpoint.Method, endpoint.Path, endpoint.Handler)
		r.Method(endpoint.Method, endpoint.Path+"/", endpoint.Handler)
	}

	r.Method(http.MethodGet, "/", index(endpoints))
	return r
}

func createServer(cfg *Config, endpoints []*Endpoint, middlewares []func(http.Handler) http.Handler) *http.Server {
	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      Router(endpoints, middlewares),
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
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("listenAndServeWorker: %w", err)
		}

		return nil
	}
}

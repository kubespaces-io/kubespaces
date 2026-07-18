// Command api runs the KubeSpaces backend HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubespaces-io/kubespaces/api/internal/auth"
	"github.com/kubespaces-io/kubespaces/api/internal/config"
	"github.com/kubespaces-io/kubespaces/api/internal/k8s"
	"github.com/kubespaces-io/kubespaces/api/internal/server"
	"github.com/kubespaces-io/kubespaces/api/internal/store"
)

const (
	startupTimeout  = 30 * time.Second
	shutdownTimeout = 15 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startupCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	pool, err := pgxpool.New(startupCtx, cfg.DSN())
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(startupCtx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	if err := store.Migrate(startupCtx, pool); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	authenticator, err := auth.New(startupCtx, cfg.OIDCIssuerURL, cfg.OIDCClientID)
	if err != nil {
		return fmt.Errorf("configure OIDC: %w", err)
	}

	cluster, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("configure kubernetes client: %w", err)
	}

	srv := server.New(store.New(pool), cluster, authenticator.Middleware)
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.ListenAddr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

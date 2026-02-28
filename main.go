package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"entgo.io/ent/dialect"
	"github.com/ddevcap/jellyfin-proxy/api"
	"github.com/ddevcap/jellyfin-proxy/api/handler"
	"github.com/ddevcap/jellyfin-proxy/backend"
	"github.com/ddevcap/jellyfin-proxy/config"
	"github.com/ddevcap/jellyfin-proxy/ent/migrate"

	"github.com/ddevcap/jellyfin-proxy/ent"
	_ "github.com/lib/pq"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	client, err := ent.Open(dialect.Postgres, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to open database connection", "error", err)
		os.Exit(1)
	}
	defer func() { _ = client.Close() }()

	if err := client.Schema.Create(
		context.Background(),
		migrate.WithGlobalUniqueID(true),
	); err != nil {
		slog.Error("failed to run schema migration", "error", err)
		os.Exit(1)
	}

	api.SeedInitialAdmin(context.Background(), client, cfg)

	pool := backend.NewPool(client, cfg)

	// Start background health checker so fan-out requests skip offline backends.
	hc := backend.NewHealthChecker(pool, cfg.HealthCheckInterval)
	pool.SetHealthChecker(hc)
	hc.Start(context.Background())

	wsHub := handler.NewWSHub()
	h, stopLimiter := api.NewRouter(client, cfg, pool, wsHub)

	// Start periodic session cleanup.
	sessionCleaner := api.NewSessionCleaner(client, cfg)
	sessionCleaner.Start(context.Background())

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	// Start server in a goroutine so we can listen for shutdown signals.
	go func() {
		slog.Info("jellyfin proxy listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt or SIGTERM (e.g. from container orchestration).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	wsHub.Shutdown()
	hc.Stop()
	stopLimiter()
	sessionCleaner.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}
	slog.Info("server stopped")
}

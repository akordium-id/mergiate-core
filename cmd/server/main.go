package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/akordium-id/mergiate-core/pkg/app"
	"github.com/akordium-id/mergiate-core/pkg/config"
)

func main() {
	// Structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// Security: validate JWT secret
	if err := cfg.ValidateJWTSecret(); err != nil {
		slog.Error("SECURITY: refusing to start — insecure JWT configuration", slog.Any("error", err))
		os.Exit(1)
	}
	if config.IsInsecureJWTSecret(cfg.JWTSecret) {
		slog.Warn("SECURITY WARNING: using insecure default JWT secret — set JWT_SECRET in production")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	application, err := app.New(ctx, app.Options{
		Config:        cfg,
		Logger:        logger,
		RunMigrations: os.Getenv("AUTO_MIGRATE") == "true",
	})
	if err != nil {
		slog.Error("failed to initialize application", slog.Any("error", err))
		os.Exit(1)
	}

	if err := application.Start(ctx); err != nil {
		slog.Error("application stopped with error", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("server exited cleanly")
}

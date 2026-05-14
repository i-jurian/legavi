// Command api is the Legavi HTTP API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/i-jurian/legavi/backend/internal/auth"
	"github.com/i-jurian/legavi/backend/internal/config"
	"github.com/i-jurian/legavi/backend/internal/pool"
	"github.com/i-jurian/legavi/backend/internal/server"
	"github.com/i-jurian/legavi/backend/internal/store"
	"github.com/i-jurian/legavi/backend/migrations"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load("../.env")
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := pool.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()

	log.Info("running migrations")
	if err := database.Migrate(migrations.FS); err != nil {
		return err
	}

	wa, err := auth.NewWebAuthn(cfg.PublicURL)
	if err != nil {
		return err
	}
	jwt, err := auth.NewJWT(cfg.JWTSigningKey, cfg.JWTTTL)
	if err != nil {
		return err
	}
	st := store.NewStore(database.Pool)
	cookies := auth.NewCookies(cfg.IsSecure(), cfg.JWTTTL)
	authH := auth.NewHandler(wa, jwt, st, cookies)

	log.Info("api starting", "public_url", cfg.PublicURL, "test_mode", cfg.TestMode)

	srv := server.New(cfg, database, log, authH)
	if err := srv.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	log.Info("api stopped")
	return nil
}

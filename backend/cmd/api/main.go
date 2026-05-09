// Command api is the Legavi HTTP API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/i-jurian/legavi/backend/internal/config"
	"github.com/i-jurian/legavi/backend/internal/db"
	"github.com/i-jurian/legavi/backend/internal/server"
	"github.com/i-jurian/legavi/backend/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()

	log.Info("running migrations")
	if err := database.Migrate(migrations.FS); err != nil {
		return err
	}

	log.Info("api starting", "public_url", cfg.PublicURL, "test_mode", cfg.TestMode)

	srv := server.New(cfg, database, log)
	if err := srv.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	log.Info("api stopped")
	return nil
}

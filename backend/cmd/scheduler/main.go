// Command scheduler is the Legavi inactivity-detection scheduler.
// M0 skeleton: logs started, waits for signal, exits. Full state-machine
// implementation lands in M4 (release state machine) per the implementation plan.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("scheduler started", "phase", "M0 skeleton")
	<-ctx.Done()
	log.Info("scheduler stopped")
}

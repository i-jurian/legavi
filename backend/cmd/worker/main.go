// Command worker is the Legavi async job consumer (emails, retries).
// M0 skeleton: logs started, waits for signal, exits. Full queue-consumer
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

	log.Info("worker started", "phase", "M0 skeleton")
	<-ctx.Done()
	log.Info("worker stopped")
}

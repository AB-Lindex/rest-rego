package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AB-Lindex/rest-rego/internal/application"
)

func main() {
	err := run(context.Background())
	if err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	app, ok := application.New()
	if !ok {
		return fmt.Errorf("failed to create application")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := app.Run(ctx)
	if err != nil {
		return fmt.Errorf("application failed: %w", err)
	}

	slog.Info("Application exiting...")

	return nil
}

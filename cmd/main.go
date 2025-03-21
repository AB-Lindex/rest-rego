package main

import (
	"log/slog"
	"os"

	"github.com/AB-Lindex/rest-rego/internal/application"
)

func main() {
	app, ok := application.New()
	if !ok {
		os.Exit(1)
	}
	defer app.Close()

	if !app.Run() {
		slog.Warn("Application exited while not ready!")
		os.Exit(1)
	}
	slog.Info("Application exiting...")
}

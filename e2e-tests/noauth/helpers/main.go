package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	e2eshared "github.com/AB-Lindex/rest-rego/e2e-tests/shared"
)

func main() {
	backendServer, err := e2eshared.NewBackendServer(18204)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start backend server: %v\n", err)
		os.Exit(1)
	}

	backendServer.Mount("noauth-allow", 200, "ok")

	fmt.Println("BACKEND: http://127.0.0.1:18204")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	backendServer.Close()
	os.Exit(0)
}

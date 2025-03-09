package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/azure"
	"github.com/AB-Lindex/rest-rego/internal/config"
	"github.com/AB-Lindex/rest-rego/internal/router"
	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/AB-Lindex/rest-rego/pkg/regocache"
	"golang.org/x/sync/errgroup"
)

// AppData is the main application data structure and coordinates the business-logic
type AppData struct {
	config *config.Fields
	regos  *regocache.RegoCache
	router *router.Proxy
	auth   types.AuthProvider
}

// New creates a new instance of the application
func New() (*AppData, bool) {
	app := &AppData{}

	app.config = config.New()

	app.startMgmt()

	// create file-cache
	slog.Debug("application: creating policy cache", "dir", app.config.PolicyDir)
	c, err := regocache.New(app.config.PolicyDir, app.config.Debug)
	if err != nil || c == nil {
		return nil, false
	}
	app.regos = c

	// create auth provider
	slog.Debug("application: creating auth provider", "tenant", app.config.AzureTenant)
	app.auth = azure.New(app.config.AzureTenant, app.config.AuthHeader)
	if app.auth == nil {
		return nil, false
	}

	// create router
	backend := fmt.Sprintf("%s://%s:%d", app.config.BackendScheme, app.config.BackendHost, app.config.BackendPort)
	slog.Debug("application: creating router", "addr", app.config.ListenAddr, "proxy", backend)
	app.router = router.New(app.config.ListenAddr, app.config.RequestRego, backend, app.auth, app.regos)
	if app.router == nil {
		return nil, false
	}

	return app, true
}

// Run starts the application
func (app *AppData) Run(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := app.regos.Watch(gCtx)
		if err != nil {
			slog.Error("application: rego watch error", "error", err)
			return err
		}

		return nil
	})

	g.Go(func() error {
		err := app.router.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Minute)
		defer shutdownCancel()
		err := app.router.Close(shutdownCtx)
		if err != nil {
			slog.Error("application: router shutdown error", "error", err)
			return err
		}

		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		app.regos.Close()
		return nil
	})

	err := g.Wait()
	if err != nil {
		slog.Error("application: error", "error", err)
		return err
	}

	return nil
}

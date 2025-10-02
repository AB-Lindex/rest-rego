package application

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AB-Lindex/rest-rego/internal/azure"
	"github.com/AB-Lindex/rest-rego/internal/config"
	"github.com/AB-Lindex/rest-rego/internal/jwtsupport"
	"github.com/AB-Lindex/rest-rego/internal/router"
	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/AB-Lindex/rest-rego/pkg/regocache"
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
	c, err := regocache.New(app.config.PolicyDir, app.config.FilePattern, app.config.Debug, app.config.RequestRego)
	if err != nil || c == nil {
		return nil, false
	}
	app.regos = c

	switch {
	case len(app.config.AzureTenant) > 0:
		slog.Debug("application: creating auth provider", "tenant", app.config.AzureTenant)
		app.auth = azure.New(app.config.AzureTenant, app.config.AuthHeader)
		if app.auth == nil {
			return nil, false
		}

	case len(app.config.WellKnownURL) > 0:
		slog.Debug("application: creating jwt-auth-provider", "well-knowns", len(app.config.WellKnownURL))
		app.auth = jwtsupport.New(app.config.WellKnownURL, app.config.Audiences)
		if app.auth == nil {
			return nil, false
		}

	default:
		slog.Error("application: no auth-provider configured")
		return nil, false
	}

	// create router
	backend := fmt.Sprintf("%s://%s:%d", app.config.BackendScheme, app.config.BackendHost, app.config.BackendPort)
	slog.Debug("application: creating router", "addr", app.config.ListenAddr, "proxy", backend)
	app.router = router.New(
		app.config.ListenAddr,
		app.config.RequestRego,
		app.config.AuthHeader,
		backend,
		app.auth,
		app.regos)
	if app.router == nil {
		return nil, false
	}

	return app, true
}

// Close closes the application
func (app *AppData) Close() {
	slog.Debug("closing router...")
	app.router.Close()
	slog.Debug("closing regos...")
	app.regos.Close()
	slog.Info("all closed - exiting")
}

// Run starts the application
func (app *AppData) Run() bool {
	cancelChan := make(chan os.Signal, 1)
	signal.Notify(cancelChan, syscall.SIGTERM, syscall.SIGINT)

	app.regos.Watch()
	app.router.ListenAndServe()

	// go func() {
	// 	for {
	// 		time.Sleep(10 * time.Second)
	// 		fmt.Println("Loop tick")
	// 	}
	// }()

	if !app.regos.Ready() {
		slog.Warn("application: waiting for regos to be ready")
	}

	sig := <-cancelChan
	slog.Warn("application: caught signal", "signal", sig)
	return app.regos.Ready()
}

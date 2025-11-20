package application

import (
	"log/slog"
	"net/http"

	"github.com/AB-Lindex/rest-rego/internal/metrics"
	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

var mgmt struct {
	mux    *chi.Mux
	reg    *prometheus.Registry
	server *http.Server
}

func (app *AppData) startMgmt() {

	metrics.New()

	// Initialize blocked headers feature metric
	metrics.SetBlockedHeadersExposed(app.config.ExposeBlockedHeaders)
	if app.config.ExposeBlockedHeaders {
		slog.Info("blocked headers feature enabled - X-Restrego-* headers will be exposed to policy")
	} else {
		slog.Info("blocked headers feature disabled - X-Restrego-* headers will be removed and not exposed to policy")
	}

	mgmt.mux = chi.NewRouter()
	mgmt.mux.Get("/healthz", app.healthzHandler)
	mgmt.mux.Get("/readyz", app.readyzHandler)
	mgmt.mux.Get("/version", versionHandler)
	mgmt.mux.Get("/config", app.configHandler)
	mgmt.mux.Get("/metrics", metrics.Handler())

	mgmt.server = &http.Server{
		Addr:    app.config.MgmtAddr,
		Handler: mgmt.mux,

		// Management endpoints are simple, use shorter timeouts (50% of proxy timeouts)
		ReadHeaderTimeout: app.config.ReadHeaderTimeout / 2,
		ReadTimeout:       app.config.ReadTimeout / 2,
		WriteTimeout:      app.config.WriteTimeout / 2,
		IdleTimeout:       app.config.IdleTimeout / 2,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	go func() {
		slog.Info("mgmt: starting management server", "addr", app.config.MgmtAddr)
		if err := mgmt.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("mgmt: server failed", "error", err)
		}
	}()
}

func (app *AppData) healthzHandler(w http.ResponseWriter, r *http.Request) {
	if !app.regos.Ready() {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (app *AppData) readyzHandler(w http.ResponseWriter, r *http.Request) {
	if !app.regos.Ready() {
		w.WriteHeader(http.StatusFailedDependency)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(types.Version()))
	w.WriteHeader(http.StatusOK)
}

func (app *AppData) configHandler(w http.ResponseWriter, r *http.Request) {
	app.regos.Info()
	w.WriteHeader(http.StatusOK)
}

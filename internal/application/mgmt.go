package application

import (
	"net/http"

	"github.com/AB-Lindex/rest-rego/internal/metrics"
	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

var mgmt struct {
	mux *chi.Mux
	reg *prometheus.Registry
}

func (app *AppData) startMgmt() {

	metrics.New()

	mgmt.mux = chi.NewRouter()
	mgmt.mux.Get("/healthz", app.healthzHandler)
	mgmt.mux.Get("/readyz", app.readyzHandler)
	mgmt.mux.Get("/version", versionHandler)
	mgmt.mux.Get("/config", app.configHandler)
	mgmt.mux.Get("/metrics", metrics.Handler())

	go http.ListenAndServe(app.config.MgmtAddr, mgmt.mux)
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

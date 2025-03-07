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
	mgmt.mux.Get("/healthz", healthzHandler)
	mgmt.mux.Get("/readyz", readyzHandler)
	mgmt.mux.Get("/version", versionHandler)
	mgmt.mux.Get("/config", configHandler)
	mgmt.mux.Get("/metrics", metrics.Handler())

	go http.ListenAndServe(app.config.MgmtAddr, mgmt.mux)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(types.Version()))
	w.WriteHeader(http.StatusOK)
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

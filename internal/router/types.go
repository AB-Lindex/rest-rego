package router

import (
	"net/http"
	"net/http/httputil"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/go-chi/chi/v5"
)

const headerPrefix = "X-Restrego-"

// Proxy is the main router and proxy-handler
type Proxy struct {
	listenAddr  string
	requestName string
	mux         *chi.Mux
	server      *http.Server
	auth        types.AuthProvider
	validator   types.Validator
	backendURL  string
	backend     *httputil.ReverseProxy
	authKey     string
}

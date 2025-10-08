package router

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/config"
	"github.com/AB-Lindex/rest-rego/internal/metrics"
	"github.com/AB-Lindex/rest-rego/internal/types"

	"github.com/go-chi/chi/v5"
)

// New creates a new instance of the Proxy
func New(auth types.AuthProvider, validator types.Validator, cfg *config.Fields) *Proxy {
	// Build backend URL from config
	backendURL := fmt.Sprintf("%s://%s:%d", cfg.BackendScheme, cfg.BackendHost, cfg.BackendPort)

	slog.Debug("router: creating proxy", "listen", cfg.ListenAddr, "backend", backendURL)

	proxy := &Proxy{
		listenAddr:  cfg.ListenAddr,
		requestName: cfg.RequestRego,
		authKey:     cfg.AuthHeader,
		backendURL:  backendURL,
		config:      cfg,
	}
	remote, err := url.Parse(proxy.backendURL)
	if err != nil {
		slog.Error("router: invalid backend URL", "error", err, "backend", proxy.backendURL)
		return nil
	}
	proxy.backend = httputil.NewSingleHostReverseProxy(remote)
	proxy.backend.Transport = &http.Transport{
		// Connection pooling
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     0, // Unlimited, but controlled by timeouts

		// Timeouts for backend communication (from config)
		DialContext: (&net.Dialer{
			Timeout:   cfg.BackendDialTimeout,
			KeepAlive: 30 * time.Second, // TCP keepalive interval
		}).DialContext,

		TLSHandshakeTimeout:   cfg.BackendDialTimeout,     // TLS handshake uses dial timeout
		ResponseHeaderTimeout: cfg.BackendResponseTimeout, // Time to receive response headers
		ExpectContinueTimeout: 1 * time.Second,            // 100-continue timeout
		IdleConnTimeout:       cfg.BackendIdleConnTimeout, // Idle connection timeout

		// Prevent connection reuse issues
		DisableKeepAlives:  false,
		DisableCompression: false,
	}

	// Add error handler for backend failures
	proxy.backend.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("router: backend proxy error",
			"error", err,
			"backend", proxy.backendURL,
			"path", r.URL.Path)
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}

	// proxy.backend.Director = nil
	// proxy.backend.Rewrite = proxy.Rewriter

	proxy.mux = chi.NewRouter()
	proxy.mux.Use(
		proxy.CleanupHandler, // cleanup before any other processing
		proxy.WrapHandler,
		metrics.Wrap,
		proxy.authHandler,
		proxy.policyHandler,
	)
	proxy.mux.Handle("/*", proxy)

	proxy.auth = auth

	proxy.validator = validator

	return proxy
}

// ListenAndServe starts the server (in background)
func (proxy *Proxy) ListenAndServe() {
	proxy.server = &http.Server{
		Addr:    proxy.listenAddr,
		Handler: proxy.mux,

		// Timeouts to prevent slowloris and similar attacks (from config)
		ReadHeaderTimeout: proxy.config.ReadHeaderTimeout,
		ReadTimeout:       proxy.config.ReadTimeout,
		WriteTimeout:      proxy.config.WriteTimeout,
		IdleTimeout:       proxy.config.IdleTimeout,

		// Maximum header size to prevent memory exhaustion
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
	go func() {
		slog.Info("router: starting server", "addr", proxy.listenAddr)
		err := proxy.server.ListenAndServe()
		if err != nil {
			if err == http.ErrServerClosed {
				slog.Info("router: server closed")
			} else {
				slog.Warn("router: server aborted", "error", err)
			}
		} else {
			slog.Error("router: server stopped")
		}
	}()
}

// Close stops the server
func (proxy *Proxy) Close() {
	if proxy.server != nil {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
		proxy.server.Shutdown(ctx)
		defer cancel()
	}
}

// ServeHTTP is the main handler
func (proxy *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	info := types.GetInfo(r)
	if info != nil {
		if resultMap, ok := info.Result.(map[string]interface{}); ok {
			for k, o := range resultMap {
				var txt string
				switch v := o.(type) {
				case string:
					txt = v
				case []string:
					txt = strings.Join(v, ",")
				case []interface{}:
					var buf strings.Builder
					for i, x := range v {
						if i > 0 {
							buf.WriteString(",")
						}
						buf.WriteString(fmt.Sprint(x))
					}
					txt = buf.String()
				default:
					txt = fmt.Sprint(v)
				}
				if len(txt) > 0 {
					key := strings.ReplaceAll(k, "_", "-")
					r.Header.Set(headerPrefix+key, txt)
				}
			}
		}
	}

	proxy.backend.ServeHTTP(w, r)
}

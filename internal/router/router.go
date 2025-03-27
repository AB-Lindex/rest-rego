package router

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/metrics"
	"github.com/AB-Lindex/rest-rego/internal/types"

	"github.com/go-chi/chi/v5"
)

// New creates a new instance of the Proxy
func New(listenAddr, requestName, authKey, backend string, auth types.AuthProvider, validator types.Validator) *Proxy {
	proxy := &Proxy{
		listenAddr:  listenAddr,
		requestName: requestName,
		authKey:     authKey,
		backendURL:  backend,
	}
	remote, err := url.Parse(proxy.backendURL)
	if err != nil {
		slog.Error("router: invalid backend URL", "error", err, "backend", proxy.backendURL)
		return nil
	}
	proxy.backend = httputil.NewSingleHostReverseProxy(remote)
	proxy.backend.Transport = &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
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

package router

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/metrics"
	"github.com/AB-Lindex/rest-rego/internal/types"

	"github.com/go-chi/chi/v5"
	"github.com/ninlil/butler/bufferedresponse"
)

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

func (proxy *Proxy) authHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := types.GetInfo(r)
		if info == nil {
			http.Error(w, "internal error - missing context", http.StatusInternalServerError)
			return
		}

		err := proxy.auth.Authenticate(info, r)
		if err != nil {
			slog.Error("router: authentication error", "error", err)
			http.Error(w, "authentication error", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (proxy *Proxy) policyHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := types.GetInfo(r)
		if info == nil {
			http.Error(w, "internal error - missing context", http.StatusInternalServerError)
			return
		}

		result, err := proxy.validator.Validate(proxy.requestName, info)
		if err != nil {
			slog.Error("router: request validation error", "error", err)
			result = err.Error()
			// http.Error(w, "internal validator error", http.StatusInternalServerError)
			// return
		}

		info.Result = result

		resultMap, ok := result.(map[string]interface{})
		if !ok {
			http.Error(w, "internal validator error", http.StatusInternalServerError)
			return
		}
		if resultMap["allow"] == false {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if url, ok := resultMap["url"].(string); ok && url != "" {
			info.URL = url
		}

		next.ServeHTTP(w, r)
	})
}

// WrapHandler wraps the handler for buffered-response and info-context
func (proxy *Proxy) WrapHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		w2 := bufferedresponse.Wrap(w)
		defer w2.Flush()

		info := types.NewInfo(r, proxy.authKey)

		r2 := info.RequestWithInfo(r)

		next.ServeHTTP(w2, r2)

		// wrap-up
		w2.Header().Set("Content-Length", strconv.Itoa(w2.Size()))

		slog.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			"status", w2.Status(),
			"duration", time.Since(now),
			"size", w2.Size(),
			"id", info.Request.ID,
		)
	})
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
	// // TODO: proxy to actual backend
	// info := types.GetInfo(r)
	// buf, err := json.Marshal(&info)
	// if err != nil {
	// 	http.Error(w, "internal error", http.StatusInternalServerError)
	// 	return
	// }
	// w.Write(buf)

}

const headerPrefix = "X-RestRego-"

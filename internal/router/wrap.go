package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

// responseTracker wraps a ResponseWriter to capture status and bytes written
// without buffering the response body in memory.
type responseTracker struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

func newResponseTracker(w http.ResponseWriter) *responseTracker {
	return &responseTracker{ResponseWriter: w, status: http.StatusOK}
}

func (rt *responseTracker) WriteHeader(status int) {
	if !rt.wroteHeader {
		rt.status = status
		rt.wroteHeader = true
	}
	rt.ResponseWriter.WriteHeader(status)
}

func (rt *responseTracker) Write(b []byte) (int, error) {
	n, err := rt.ResponseWriter.Write(b)
	rt.size += n
	return n, err
}

// Status returns the HTTP status code written to the response.
func (rt *responseTracker) Status() int { return rt.status }

// Size returns the number of bytes written to the response body.
func (rt *responseTracker) Size() int { return rt.size }

// WrapHandler wraps the handler to capture request info and log the response.
func (proxy *Proxy) WrapHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		w2 := newResponseTracker(w)

		info := types.NewInfo(r, proxy.authKey, proxy.config.URLMetricsLevel)
		r2 := info.RequestWithInfo(r)

		next.ServeHTTP(w2, r2)

		slog.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			"status", w2.status,
			"duration", time.Since(now),
			"size", w2.size,
			"id", info.Request.ID,
		)
	})
}

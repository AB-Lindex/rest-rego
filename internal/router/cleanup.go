package router

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AB-Lindex/rest-rego/internal/metrics"
	"github.com/AB-Lindex/rest-rego/internal/types"
)

type ctxKey int

const ctxBlockedHeadersKey ctxKey = 1

// CleanupHandler removes incoming headers that shouldn't be there
// (like any X-Restrego header which is considered spoofing-attempts)
func (proxy *Proxy) CleanupHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// to avoid risk of delete-key-while-looping we add keys to separate list
		var toClean []string
		for key := range r.Header {
			if strings.HasPrefix(key, "X-Restrego-") {
				toClean = append(toClean, key)
			}
		}

		// Capture blocked headers if feature is enabled
		var blocked map[string]interface{}
		if proxy.config != nil && proxy.config.ExposeBlockedHeaders && len(toClean) > 0 {
			blocked = make(map[string]interface{})
			for _, key := range toClean {
				values := r.Header.Values(key)
				if len(values) == 1 {
					// Single value stored as string
					blocked[key] = values[0]
				} else if len(values) > 1 {
					// Multiple values stored as slice
					blocked[key] = values
				}
				// Increment counter for each header captured
				metrics.IncrementBlockedHeadersCaptured(1)
			}
			// Store in context before removing headers
			r = r.WithContext(context.WithValue(r.Context(), types.CtxBlockedHeadersKey, blocked))
			// Increment counter for requests with blocked headers
			metrics.IncrementRequestsWithBlockedHeaders()
		}

		// Remove headers (always happens regardless of feature flag)
		for _, key := range toClean {
			if proxy.config != nil && proxy.config.ExposeBlockedHeaders {
				slog.Debug("removing and capturing header", "header", key)
			} else {
				slog.Warn("removing header (possible spoofing)", "header", key)
			}
			r.Header.Del(key)
		}

		next.ServeHTTP(w, r)
	})
}

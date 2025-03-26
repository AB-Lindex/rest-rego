package router

import (
	"log/slog"
	"net/http"
	"strings"
)

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
		for _, key := range toClean {
			slog.Warn("removing header (possible spoofing)", "header", key)
			r.Header.Del(key)
		}

		next.ServeHTTP(w, r)
	})
}

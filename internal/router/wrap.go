package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/ninlil/butler/bufferedresponse"
)

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

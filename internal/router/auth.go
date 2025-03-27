package router

import (
	"log/slog"
	"net/http"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

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

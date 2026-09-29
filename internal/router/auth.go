package router

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

// wwwAuthenticateChallenge returns the WWW-Authenticate header value for this
// proxy's auth provider, appending resource_metadata when RFC 9728 metadata is
// enabled.
func (proxy *Proxy) wwwAuthenticateChallenge() string {
	challenge := "Bearer"
	if c, ok := proxy.auth.(types.AuthChallenger); ok {
		challenge = c.WWWAuthenticate()
	}
	if proxy.resourceMetadataURL != "" {
		challenge = fmt.Sprintf(`%s resource_metadata="%s"`, challenge, proxy.resourceMetadataURL)
	}
	return challenge
}

func (proxy *Proxy) authHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := types.GetInfo(r)
		if info == nil {
			slog.Error("router: missing request context")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		err := proxy.auth.Authenticate(info, r)

		switch {
		case err == nil:
			// Success or anonymous - proceed to policy
			isAuth := info.JWT != nil || info.User != nil
			slog.Debug("router: authentication complete",
				"authenticated", isAuth,
				"path", r.URL.Path)
			next.ServeHTTP(w, r)

		case errors.Is(err, types.ErrAuthenticationFailed):
			// Invalid credentials in strict mode
			slog.Warn("router: authentication failed",
				"path", r.URL.Path,
				"method", r.Method)
			w.Header().Set("WWW-Authenticate", proxy.wwwAuthenticateChallenge())
			http.Error(w, "invalid credentials", http.StatusUnauthorized)

		case errors.Is(err, types.ErrAuthenticationUnavailable):
			// System unavailable - fail closed regardless of mode
			slog.Error("router: authentication system unavailable",
				"path", r.URL.Path)
			http.Error(w, "authentication service unavailable", http.StatusServiceUnavailable)

		default:
			// Unexpected error
			slog.Error("router: unexpected authentication error", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	})
}

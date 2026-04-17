package noauth

import (
	"log/slog"
	"net/http"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

type NoAuthProvider struct{}

func New(permissive bool) *NoAuthProvider {
	if permissive {
		slog.Error("noauth: PERMISSIVE_AUTH cannot be combined with NO_AUTH")
		return nil
	}
	slog.Warn("noauth: no-auth mode enabled — policy is the sole access control")
	return &NoAuthProvider{}
}

func (b *NoAuthProvider) Authenticate(info *types.Info, _ *http.Request) error {
	// No authentication performed; info.JWT and info.User remain unset.
	return nil
}

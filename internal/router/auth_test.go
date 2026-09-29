package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

// authTestMockProvider fails every authentication attempt with a fixed error.
type authTestMockProvider struct {
	err error
}

func (m *authTestMockProvider) Authenticate(info *types.Info, r *http.Request) error {
	return m.err
}

func TestAuthHandler_InvalidCredentials_ChallengeHeader(t *testing.T) {
	newAuthTestRequest := func(t *testing.T, proxy *Proxy) *httptest.ResponseRecorder {
		t.Helper()

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		info := &types.Info{}
		req = info.RequestWithInfo(req)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		proxy.authHandler(next).ServeHTTP(rr, req)

		if nextCalled {
			t.Errorf("expected next handler NOT to be called on authentication failure")
		}

		return rr
	}

	t.Run("resource metadata disabled - existing challenge behavior preserved", func(t *testing.T) {
		proxy := &Proxy{
			auth: &authTestMockProvider{err: types.ErrAuthenticationFailed},
		}
		rr := newAuthTestRequest(t, proxy)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
		wa := rr.Header().Get("WWW-Authenticate")
		if wa != "Bearer" {
			t.Errorf("expected WWW-Authenticate %q, got %q", "Bearer", wa)
		}
	})

	t.Run("resource metadata enabled - challenge carries resource_metadata", func(t *testing.T) {
		proxy := &Proxy{
			auth:                &authTestMockProvider{err: types.ErrAuthenticationFailed},
			resourceMetadataURL: "https://host/.well-known/oauth-protected-resource",
		}
		rr := newAuthTestRequest(t, proxy)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
		wa := rr.Header().Get("WWW-Authenticate")
		if !strings.HasPrefix(wa, "Bearer ") || !strings.Contains(wa, `resource_metadata="https://host/.well-known/oauth-protected-resource"`) {
			t.Errorf("expected WWW-Authenticate to carry resource_metadata, got %q", wa)
		}
	})
}

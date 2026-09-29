package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AB-Lindex/rest-rego/internal/config"
	"github.com/AB-Lindex/rest-rego/internal/router"
	"github.com/AB-Lindex/rest-rego/internal/types"
)

// failingAuthProvider fails every authentication attempt, so any request that
// reaches authHandler is guaranteed to be rejected.
type failingAuthProvider struct {
	issuers []string
}

func (m *failingAuthProvider) Authenticate(info *types.Info, r *http.Request) error {
	return types.ErrAuthenticationFailed
}

func (m *failingAuthProvider) Issuers() []string {
	return m.issuers
}

// TestMetadataEndpoint_BypassesAuthAndPolicy is a regression test for a bug where
// the RFC 9728 metadata endpoint was routed through chi's mux-wide Use()
// middleware chain (authHandler/policyHandler) and therefore returned 401 for
// anonymous requests instead of serving the public metadata document.
func TestMetadataEndpoint_BypassesAuthAndPolicy(t *testing.T) {
	cfg := &config.Fields{
		BackendScheme:        "http",
		BackendHost:          "localhost",
		BackendPort:          8080,
		ListenAddr:           ":8181",
		ResourceURL:          "https://api.example.com/mcp",
		ResourceMetadataPath: "/.well-known/oauth-protected-resource",
	}

	auth := &failingAuthProvider{issuers: []string{"https://idp.example.com"}}
	proxy := router.New(auth, &mockValidator{}, cfg)
	if proxy == nil {
		t.Fatal("Failed to create proxy")
	}
	handler := proxy.Handler()

	t.Run("metadata endpoint is served without authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var body map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("expected valid JSON body, got error: %v (body: %s)", err, rr.Body.String())
		}
		if body["resource"] != "https://api.example.com/mcp" {
			t.Errorf("expected resource %q, got %v", "https://api.example.com/mcp", body["resource"])
		}
	})

	t.Run("all other paths still require authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// TestMetadataEndpoint_ScopesSupported verifies scopes_supported is present in the
// PRM response body when RESOURCE_SCOPES is configured, and absent when it isn't.
func TestMetadataEndpoint_ScopesSupported(t *testing.T) {
	tests := []struct {
		name           string
		resourceScopes []string
		wantKey        bool
	}{
		{
			name:           "no resource-scopes configured",
			resourceScopes: nil,
			wantKey:        false,
		},
		{
			name:           "resource-scopes configured",
			resourceScopes: []string{"https://api.example.com/mcp/access_as_user"},
			wantKey:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Fields{
				BackendScheme:        "http",
				BackendHost:          "localhost",
				BackendPort:          8080,
				ListenAddr:           ":8181",
				ResourceURL:          "https://api.example.com/mcp",
				ResourceMetadataPath: "/.well-known/oauth-protected-resource",
				ResourceScopes:       tt.resourceScopes,
			}

			auth := &failingAuthProvider{issuers: []string{"https://idp.example.com"}}
			proxy := router.New(auth, &mockValidator{}, cfg)
			if proxy == nil {
				t.Fatal("Failed to create proxy")
			}
			handler := proxy.Handler()

			req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
			}

			var body map[string]interface{}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("expected valid JSON body, got error: %v (body: %s)", err, rr.Body.String())
			}

			scopes, hasKey := body["scopes_supported"]
			if hasKey != tt.wantKey {
				t.Errorf("expected scopes_supported key present=%v, got %v (scopes: %v)", tt.wantKey, hasKey, scopes)
			}
			if tt.wantKey {
				got, ok := scopes.([]interface{})
				if !ok || len(got) != len(tt.resourceScopes) || got[0] != tt.resourceScopes[0] {
					t.Errorf("expected scopes_supported %v, got %v", tt.resourceScopes, scopes)
				}
			}
		})
	}
}

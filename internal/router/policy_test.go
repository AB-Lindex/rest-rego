package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

// policyMockValidator returns a fixed result (or error) for every Validate call.
type policyMockValidator struct {
	result interface{}
	err    error
}

func (m *policyMockValidator) Validate(name string, input interface{}) (interface{}, error) {
	return m.result, m.err
}

func newPolicyTestRequest(t *testing.T, result interface{}) (*httptest.ResponseRecorder, *http.Request, *types.Info) {
	t.Helper()

	proxy := &Proxy{
		requestName: "request.rego",
		validator:   &policyMockValidator{result: result},
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	info := &types.Info{}
	req = info.RequestWithInfo(req)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	proxy.policyHandler(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Errorf("expected next handler to be called, got status %d", rr.Code)
	}

	return rr, req, info
}

func TestPolicyHandler_Labels(t *testing.T) {
	t.Run("no labels key leaves info.Labels nil", func(t *testing.T) {
		_, _, info := newPolicyTestRequest(t, map[string]interface{}{
			"allow": true,
		})
		if info.Labels != nil {
			t.Errorf("expected nil Labels, got %v", info.Labels)
		}
	})

	t.Run("string-valued labels populate info.Labels", func(t *testing.T) {
		_, _, info := newPolicyTestRequest(t, map[string]interface{}{
			"allow": true,
			"labels": map[string]interface{}{
				"client_version": "1.2.3",
				"tenant":         "acme",
			},
		})
		want := map[string]string{"client_version": "1.2.3", "tenant": "acme"}
		if len(info.Labels) != len(want) {
			t.Fatalf("expected %v, got %v", want, info.Labels)
		}
		for k, v := range want {
			if info.Labels[k] != v {
				t.Errorf("expected Labels[%q] = %q, got %q", k, v, info.Labels[k])
			}
		}
	})

	t.Run("non-string values are dropped, valid ones kept", func(t *testing.T) {
		_, _, info := newPolicyTestRequest(t, map[string]interface{}{
			"allow": true,
			"labels": map[string]interface{}{
				"tenant": "acme",
				"count":  42,
				"nested": map[string]interface{}{"a": "b"},
			},
		})
		if len(info.Labels) != 1 {
			t.Fatalf("expected only 1 valid label, got %v", info.Labels)
		}
		if info.Labels["tenant"] != "acme" {
			t.Errorf("expected Labels[\"tenant\"] = %q, got %q", "acme", info.Labels["tenant"])
		}
	})

	t.Run("labels present but wrong type is ignored without error", func(t *testing.T) {
		_, _, info := newPolicyTestRequest(t, map[string]interface{}{
			"allow":  true,
			"labels": "not-a-map",
		})
		if info.Labels != nil {
			t.Errorf("expected nil Labels, got %v", info.Labels)
		}
	})
}

func newDenyPathTestRequest(t *testing.T, proxy *Proxy, auth *types.RequestAuth) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	info := &types.Info{Request: types.RequestInfo{Auth: auth}}
	req = info.RequestWithInfo(req)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	proxy.policyHandler(next).ServeHTTP(rr, req)

	if nextCalled {
		t.Errorf("expected next handler NOT to be called on policy deny, but it was")
	}

	return rr
}

func TestPolicyHandler_AllowValidation(t *testing.T) {
	tests := []struct {
		name     string
		result   interface{}
		want     int
		wantNext bool
	}{
		{"allow=true proceeds", map[string]interface{}{"allow": true}, http.StatusOK, true},
		{"allow=false is denied", map[string]interface{}{"allow": false}, http.StatusForbidden, false},
		{"nil result (undefined) is denied", nil, http.StatusForbidden, false},
		{"missing allow is denied", map[string]interface{}{"url": "/x"}, http.StatusForbidden, false},
		{"empty map is denied", map[string]interface{}{}, http.StatusForbidden, false},
		{"string allow is a policy error", map[string]interface{}{"allow": "true"}, http.StatusInternalServerError, false},
		{"number allow is a policy error", map[string]interface{}{"allow": 1}, http.StatusInternalServerError, false},
		{"null allow is a policy error", map[string]interface{}{"allow": nil}, http.StatusInternalServerError, false},
		{"non-map result is a policy error", "allow", http.StatusInternalServerError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &Proxy{requestName: "request.rego", validator: &policyMockValidator{result: tt.result}}
			req := (&types.Info{}).RequestWithInfo(httptest.NewRequest(http.MethodGet, "/test", nil))

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { nextCalled = true })

			rr := httptest.NewRecorder()
			proxy.policyHandler(next).ServeHTTP(rr, req)

			if nextCalled != tt.wantNext {
				t.Errorf("next handler called = %v, want %v", nextCalled, tt.wantNext)
			}
			if rr.Code != tt.want {
				t.Errorf("expected %d, got %d", tt.want, rr.Code)
			}
		})
	}
}

func TestPolicyHandler_DenyPathStatusCodeSplit(t *testing.T) {
	t.Run("JWT mode + resource-url, no Authorization header -> 401 with resource_metadata challenge", func(t *testing.T) {
		proxy := &Proxy{
			requestName:         "request.rego",
			validator:           &policyMockValidator{result: map[string]interface{}{"allow": false}},
			resourceMetadataURL: "https://host/.well-known/oauth-protected-resource",
		}
		rr := newDenyPathTestRequest(t, proxy, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
		wa := rr.Header().Get("WWW-Authenticate")
		if !strings.Contains(wa, "resource_metadata=") {
			t.Errorf("expected WWW-Authenticate to contain resource_metadata, got %q", wa)
		}
	})

	t.Run("JWT mode + resource-url, valid credentials presented -> 403, no WWW-Authenticate", func(t *testing.T) {
		proxy := &Proxy{
			requestName:         "request.rego",
			validator:           &policyMockValidator{result: map[string]interface{}{"allow": false}},
			resourceMetadataURL: "https://host/.well-known/oauth-protected-resource",
		}
		rr := newDenyPathTestRequest(t, proxy, &types.RequestAuth{Kind: "bearer", Token: "abc"})
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
		if wa := rr.Header().Get("WWW-Authenticate"); wa != "" {
			t.Errorf("expected no WWW-Authenticate header, got %q", wa)
		}
	})

	t.Run("JWT mode, resource-url not set, no Authorization header -> 403 unchanged legacy behavior", func(t *testing.T) {
		proxy := &Proxy{
			requestName: "request.rego",
			validator:   &policyMockValidator{result: map[string]interface{}{"allow": false}},
		}
		rr := newDenyPathTestRequest(t, proxy, nil)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
		if wa := rr.Header().Get("WWW-Authenticate"); wa != "" {
			t.Errorf("expected no WWW-Authenticate header, got %q", wa)
		}
	})

	t.Run("allow=true anonymous request proceeds unaffected", func(t *testing.T) {
		proxy := &Proxy{
			requestName:         "request.rego",
			validator:           &policyMockValidator{result: map[string]interface{}{"allow": true}},
			resourceMetadataURL: "https://host/.well-known/oauth-protected-resource",
		}
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		info := &types.Info{Request: types.RequestInfo{Auth: nil}}
		req = info.RequestWithInfo(req)
		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})
		rr := httptest.NewRecorder()
		proxy.policyHandler(next).ServeHTTP(rr, req)
		if !nextCalled {
			t.Errorf("expected next handler to be called, got status %d", rr.Code)
		}
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

package router

import (
	"net/http"
	"net/http/httptest"
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

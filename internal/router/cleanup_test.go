package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AB-Lindex/rest-rego/internal/router"
)

func TestCleanupHandler(t *testing.T) {
	testCases := []struct {
		name           string
		setupHeaders   map[string]string
		expectedRemove []string
		expectedKeep   map[string]string
	}{
		{
			name: "removes X-Restrego headers",
			setupHeaders: map[string]string{
				"X-Restrego-Test":  "value",
				"X-Restrego-Other": "other-value",
				"X-Keep-This":      "keep",
				"Content-Type":     "application/json",
			},
			expectedRemove: []string{"X-Restrego-Test", "X-Restrego-Other"},
			expectedKeep: map[string]string{
				"X-Keep-This":  "keep",
				"Content-Type": "application/json",
			},
		},
		{
			name: "keeps all non X-Restrego headers",
			setupHeaders: map[string]string{
				"X-Custom":     "value",
				"Content-Type": "application/json",
				"User-Agent":   "test",
			},
			expectedRemove: []string{},
			expectedKeep: map[string]string{
				"X-Custom":     "value",
				"Content-Type": "application/json",
				"User-Agent":   "test",
			},
		},
		{
			name:           "handles empty headers",
			setupHeaders:   map[string]string{},
			expectedRemove: []string{},
			expectedKeep:   map[string]string{},
		},
		{
			name: "case insensitivity test",
			setupHeaders: map[string]string{
				"x-restrego-lowercase": "value1",
				"X-Restrego-Uppercase": "value2",
				"X-restrego-MixedCase": "value3",
				"Regular-Header":       "stay",
			},
			expectedRemove: []string{"x-restrego-lowercase", "X-Restrego-Uppercase", "X-restrego-MixedCase"},
			expectedKeep: map[string]string{
				"Regular-Header": "stay",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request with the specified headers
			req, err := http.NewRequest("GET", "http://example.com", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			for k, v := range tc.setupHeaders {
				req.Header.Add(k, v)
			}

			// Create a test response recorder
			w := httptest.NewRecorder()

			// Create a next handler that checks if headers were properly cleaned
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check that removed headers are no longer present
				for _, header := range tc.expectedRemove {
					if values, exists := r.Header[header]; exists {
						t.Errorf("Header %s should have been removed, but found with values: %v", header, values)
					}
				}

				// Check that kept headers are still present with correct values
				for header, expectedValue := range tc.expectedKeep {
					actualValue := r.Header.Get(header)
					if actualValue != expectedValue {
						t.Errorf("Expected header %s to have value %s, but got %s", header, expectedValue, actualValue)
					}
				}

				w.WriteHeader(http.StatusOK)
			})

			// Create and use the cleanup handler
			proxy := &router.Proxy{}
			cleanupHandler := proxy.CleanupHandler(nextHandler)
			cleanupHandler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
		})
	}
}

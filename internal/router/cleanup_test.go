package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AB-Lindex/rest-rego/internal/config"
	"github.com/AB-Lindex/rest-rego/internal/metrics"
	"github.com/AB-Lindex/rest-rego/internal/router"
	"github.com/AB-Lindex/rest-rego/internal/types"
)

func init() {
	// Initialize metrics for testing
	metrics.New()
}

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

func TestCleanupHandler_BlockedHeaders(t *testing.T) {
	testCases := []struct {
		name             string
		exposeEnabled    bool
		setupHeaders     map[string]string
		multiValueHeader map[string][]string // For testing multiple values
		expectedRemove   []string
		expectedKeep     map[string]string
		expectInContext  bool
		expectedBlocked  map[string]interface{}
	}{
		{
			name:          "feature disabled - headers removed, NOT in context",
			exposeEnabled: false,
			setupHeaders: map[string]string{
				"X-Restrego-Test":  "value1",
				"X-Restrego-Other": "value2",
				"Content-Type":     "application/json",
			},
			expectedRemove: []string{"X-Restrego-Test", "X-Restrego-Other"},
			expectedKeep: map[string]string{
				"Content-Type": "application/json",
			},
			expectInContext: false,
			expectedBlocked: nil,
		},
		{
			name:          "feature enabled - headers removed AND captured in context",
			exposeEnabled: true,
			setupHeaders: map[string]string{
				"X-Restrego-Test":  "value1",
				"X-Restrego-Other": "value2",
				"Content-Type":     "application/json",
			},
			expectedRemove: []string{"X-Restrego-Test", "X-Restrego-Other"},
			expectedKeep: map[string]string{
				"Content-Type": "application/json",
			},
			expectInContext: true,
			expectedBlocked: map[string]interface{}{
				"X-Restrego-Test":  "value1",
				"X-Restrego-Other": "value2",
			},
		},
		{
			name:            "feature enabled - no blocked headers present",
			exposeEnabled:   true,
			setupHeaders:    map[string]string{"Content-Type": "application/json"},
			expectedRemove:  []string{},
			expectedKeep:    map[string]string{"Content-Type": "application/json"},
			expectInContext: false,
			expectedBlocked: nil,
		},
		{
			name:            "feature disabled - no blocked headers present",
			exposeEnabled:   false,
			setupHeaders:    map[string]string{"Content-Type": "application/json"},
			expectedRemove:  []string{},
			expectedKeep:    map[string]string{"Content-Type": "application/json"},
			expectInContext: false,
			expectedBlocked: nil,
		},
		{
			name:          "feature enabled - multiple values for same header",
			exposeEnabled: true,
			setupHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			multiValueHeader: map[string][]string{
				"X-Restrego-Multi": {"value1", "value2", "value3"},
			},
			expectedRemove:  []string{"X-Restrego-Multi"},
			expectedKeep:    map[string]string{"Content-Type": "application/json"},
			expectInContext: true,
			expectedBlocked: map[string]interface{}{
				"X-Restrego-Multi": []string{"value1", "value2", "value3"},
			},
		},
		{
			name:          "feature enabled - single value stored as string",
			exposeEnabled: true,
			setupHeaders: map[string]string{
				"X-Restrego-Single": "single-value",
				"Content-Type":      "application/json",
			},
			expectedRemove:  []string{"X-Restrego-Single"},
			expectedKeep:    map[string]string{"Content-Type": "application/json"},
			expectInContext: true,
			expectedBlocked: map[string]interface{}{
				"X-Restrego-Single": "single-value",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			req, err := http.NewRequest("GET", "http://example.com", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Add single-value headers
			for k, v := range tc.setupHeaders {
				req.Header.Add(k, v)
			}

			// Add multi-value headers
			for k, values := range tc.multiValueHeader {
				for _, v := range values {
					req.Header.Add(k, v)
				}
			}

			w := httptest.NewRecorder()

			// Create next handler that validates the request state
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers are removed
				for _, header := range tc.expectedRemove {
					if values, exists := r.Header[header]; exists {
						t.Errorf("Header %s should have been removed, but found with values: %v", header, values)
					}
				}

				// Verify kept headers are still present
				for header, expectedValue := range tc.expectedKeep {
					actualValue := r.Header.Get(header)
					if actualValue != expectedValue {
						t.Errorf("Expected header %s to have value %s, but got %s", header, expectedValue, actualValue)
					}
				}

				// Verify context contains (or doesn't contain) blocked headers
				blocked := types.GetBlockedHeaders(r)
				if tc.expectInContext {
					if blocked == nil {
						t.Error("Expected blocked headers in context, but got nil")
					} else {
						// Verify blocked headers match expected
						for key, expectedVal := range tc.expectedBlocked {
							actualVal, exists := blocked[key]
							if !exists {
								t.Errorf("Expected blocked header %s not found in context", key)
								continue
							}

							// Handle both string and []string types
							switch expected := expectedVal.(type) {
							case string:
								if actual, ok := actualVal.(string); ok {
									if actual != expected {
										t.Errorf("Blocked header %s: expected %s, got %s", key, expected, actual)
									}
								} else {
									t.Errorf("Blocked header %s: expected string type, got %T", key, actualVal)
								}
							case []string:
								if actual, ok := actualVal.([]string); ok {
									if len(actual) != len(expected) {
										t.Errorf("Blocked header %s: expected %d values, got %d", key, len(expected), len(actual))
									} else {
										for i, exp := range expected {
											if actual[i] != exp {
												t.Errorf("Blocked header %s[%d]: expected %s, got %s", key, i, exp, actual[i])
											}
										}
									}
								} else {
									t.Errorf("Blocked header %s: expected []string type, got %T", key, actualVal)
								}
							}
						}
					}
				} else {
					if blocked != nil {
						t.Errorf("Expected no blocked headers in context, but got: %v", blocked)
					}
				}

				w.WriteHeader(http.StatusOK)
			})

			// Create proxy with appropriate config using router.New()
			cfg := &config.Fields{
				ExposeBlockedHeaders: tc.exposeEnabled,
				BackendScheme:        "http",
				BackendHost:          "localhost",
				BackendPort:          8080,
				ListenAddr:           ":8181",
			}

			// Create minimal mock auth provider and validator for testing
			mockAuth := &mockAuthProvider{}
			mockValidator := &mockValidator{}

			proxy := router.New(mockAuth, mockValidator, cfg)
			if proxy == nil {
				t.Fatal("Failed to create proxy")
			}

			cleanupHandler := proxy.CleanupHandler(nextHandler)
			cleanupHandler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
			}
		})
	}
}

// Mock implementations for testing
type mockAuthProvider struct{}

func (m *mockAuthProvider) Authenticate(info *types.Info, r *http.Request) error {
	return nil
}

type mockValidator struct{}

func (m *mockValidator) Validate(name string, input interface{}) (interface{}, error) {
	return map[string]interface{}{"allow": true}, nil
}

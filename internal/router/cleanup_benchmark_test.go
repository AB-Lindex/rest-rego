package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AB-Lindex/rest-rego/internal/config"
	"github.com/AB-Lindex/rest-rego/internal/router"
)

// BenchmarkCleanupHandler_Disabled measures baseline performance with feature disabled
func BenchmarkCleanupHandler_Disabled(b *testing.B) {
	// Create proxy with feature disabled (default)
	cfg := &config.Fields{
		ExposeBlockedHeaders: false,
		BackendScheme:        "http",
		BackendHost:          "localhost",
		BackendPort:          8080,
	}

	proxy := router.New(&mockAuthProvider{}, &mockValidator{}, cfg)
	if proxy == nil {
		b.Fatal("failed to create proxy")
	}

	// Create test request with X-Restrego headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Restrego-Test", "value")
	req.Header.Set("X-Restrego-User", "testuser")
	req.Header.Set("Content-Type", "application/json")

	// Backend handler (minimal)
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clone request for each iteration
		reqClone := req.Clone(req.Context())
		reqClone.Header = req.Header.Clone()

		w := httptest.NewRecorder()
		handler := proxy.CleanupHandler(backend)
		handler.ServeHTTP(w, reqClone)
	}
}

// BenchmarkCleanupHandler_Enabled_NoBlockedHeaders measures overhead when enabled but no blocked headers present
func BenchmarkCleanupHandler_Enabled_NoBlockedHeaders(b *testing.B) {
	// Create proxy with feature enabled
	cfg := &config.Fields{
		ExposeBlockedHeaders: true,
		BackendScheme:        "http",
		BackendHost:          "localhost",
		BackendPort:          8080,
	}

	proxy := router.New(&mockAuthProvider{}, &mockValidator{}, cfg)
	if proxy == nil {
		b.Fatal("failed to create proxy")
	}

	// Create test request WITHOUT X-Restrego headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "benchmark-test")
	req.Header.Set("Authorization", "Bearer token")

	// Backend handler (minimal)
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clone request for each iteration
		reqClone := req.Clone(req.Context())
		reqClone.Header = req.Header.Clone()

		w := httptest.NewRecorder()
		handler := proxy.CleanupHandler(backend)
		handler.ServeHTTP(w, reqClone)
	}
}

// BenchmarkCleanupHandler_Enabled_WithBlockedHeaders measures overhead when enabled with blocked headers
func BenchmarkCleanupHandler_Enabled_WithBlockedHeaders(b *testing.B) {
	// Create proxy with feature enabled
	cfg := &config.Fields{
		ExposeBlockedHeaders: true,
		BackendScheme:        "http",
		BackendHost:          "localhost",
		BackendPort:          8080,
	}

	proxy := router.New(&mockAuthProvider{}, &mockValidator{}, cfg)
	if proxy == nil {
		b.Fatal("failed to create proxy")
	}

	// Create test request WITH X-Restrego headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Restrego-Test", "value")
	req.Header.Set("X-Restrego-User", "testuser")
	req.Header.Set("X-Restrego-Layer", "upstream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "benchmark-test")

	// Backend handler (minimal)
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clone request for each iteration
		reqClone := req.Clone(req.Context())
		reqClone.Header = req.Header.Clone()

		w := httptest.NewRecorder()
		handler := proxy.CleanupHandler(backend)
		handler.ServeHTTP(w, reqClone)
	}
}

// BenchmarkCleanupHandler_Enabled_ManyBlockedHeaders measures overhead with many blocked headers
func BenchmarkCleanupHandler_Enabled_ManyBlockedHeaders(b *testing.B) {
	// Create proxy with feature enabled
	cfg := &config.Fields{
		ExposeBlockedHeaders: true,
		BackendScheme:        "http",
		BackendHost:          "localhost",
		BackendPort:          8080,
	}

	proxy := router.New(&mockAuthProvider{}, &mockValidator{}, cfg)
	if proxy == nil {
		b.Fatal("failed to create proxy")
	}

	// Create test request WITH many X-Restrego headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Restrego-Test-1", "value1")
	req.Header.Set("X-Restrego-Test-2", "value2")
	req.Header.Set("X-Restrego-Test-3", "value3")
	req.Header.Set("X-Restrego-Test-4", "value4")
	req.Header.Set("X-Restrego-Test-5", "value5")
	req.Header.Set("X-Restrego-Test-6", "value6")
	req.Header.Set("X-Restrego-Test-7", "value7")
	req.Header.Set("X-Restrego-Test-8", "value8")
	req.Header.Set("X-Restrego-Test-9", "value9")
	req.Header.Set("X-Restrego-Test-10", "value10")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "benchmark-test")

	// Backend handler (minimal)
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clone request for each iteration
		reqClone := req.Clone(req.Context())
		reqClone.Header = req.Header.Clone()

		w := httptest.NewRecorder()
		handler := proxy.CleanupHandler(backend)
		handler.ServeHTTP(w, reqClone)
	}
}

// BenchmarkCleanupHandler_Enabled_MultiValueHeaders measures overhead with multi-value blocked headers
func BenchmarkCleanupHandler_Enabled_MultiValueHeaders(b *testing.B) {
	// Create proxy with feature enabled
	cfg := &config.Fields{
		ExposeBlockedHeaders: true,
		BackendScheme:        "http",
		BackendHost:          "localhost",
		BackendPort:          8080,
	}

	proxy := router.New(&mockAuthProvider{}, &mockValidator{}, cfg)
	if proxy == nil {
		b.Fatal("failed to create proxy")
	}

	// Create test request WITH multi-value X-Restrego headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Add("X-Restrego-Values", "value1")
	req.Header.Add("X-Restrego-Values", "value2")
	req.Header.Add("X-Restrego-Values", "value3")
	req.Header.Add("X-Restrego-Tags", "tag1")
	req.Header.Add("X-Restrego-Tags", "tag2")
	req.Header.Set("Content-Type", "application/json")

	// Backend handler (minimal)
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clone request for each iteration
		reqClone := req.Clone(req.Context())
		reqClone.Header = req.Header.Clone()

		w := httptest.NewRecorder()
		handler := proxy.CleanupHandler(backend)
		handler.ServeHTTP(w, reqClone)
	}
}

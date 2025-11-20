package types

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestNewInfo(t *testing.T) {
	testCases := []struct {
		name            string
		method          string
		path            string
		headers         map[string][]string
		contentLength   int64
		authKey         string
		expectedMethod  string
		expectedPath    []string
		expectedSize    int64
		expectedURL     string
		expectedHeaders map[string]interface{}
		expectedAuth    *RequestAuth
	}{
		{
			name:            "Simple GET request",
			method:          "GET",
			path:            "/api/users",
			headers:         map[string][]string{},
			contentLength:   0,
			authKey:         "Authorization",
			expectedMethod:  "GET",
			expectedPath:    []string{"api", "users"},
			expectedSize:    0,
			expectedURL:     "/api/users",
			expectedHeaders: map[string]interface{}{},
			expectedAuth:    nil,
		},
		{
			name:   "POST request with single header values",
			method: "POST",
			path:   "/api/data",
			headers: map[string][]string{
				"Content-Type": {"application/json"},
				"X-Request-Id": {"12345"},
			},
			contentLength:  256,
			authKey:        "Authorization",
			expectedMethod: "POST",
			expectedPath:   []string{"api", "data"},
			expectedSize:   256,
			expectedURL:    "/api/data",
			expectedHeaders: map[string]interface{}{
				"Content-Type": "application/json",
				"X-Request-Id": "12345",
			},
			expectedAuth: nil,
		},
		{
			name:   "Request with multiple header values",
			method: "GET",
			path:   "/test",
			headers: map[string][]string{
				"Accept":          {"application/json", "text/html"},
				"Accept-Encoding": {"gzip", "deflate", "br"},
				"Single-Value":    {"only-one"},
			},
			contentLength:  0,
			authKey:        "Authorization",
			expectedMethod: "GET",
			expectedPath:   []string{"test"},
			expectedSize:   0,
			expectedURL:    "/test",
			expectedHeaders: map[string]interface{}{
				"Accept":          []string{"application/json", "text/html"},
				"Accept-Encoding": []string{"gzip", "deflate", "br"},
				"Single-Value":    "only-one",
			},
			expectedAuth: nil,
		},
		{
			name:   "Bearer token authentication",
			method: "GET",
			path:   "/protected",
			headers: map[string][]string{
				"Authorization": {"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
			},
			contentLength:  0,
			authKey:        "Authorization",
			expectedMethod: "GET",
			expectedPath:   []string{"protected"},
			expectedSize:   0,
			expectedURL:    "/protected",
			expectedHeaders: map[string]interface{}{
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			},
			expectedAuth: &RequestAuth{
				Kind:  "Bearer",
				Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			},
		},
		{
			name:   "Basic authentication",
			method: "GET",
			path:   "/admin",
			headers: map[string][]string{
				"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte("user:password"))},
			},
			contentLength:  0,
			authKey:        "Authorization",
			expectedMethod: "GET",
			expectedPath:   []string{"admin"},
			expectedSize:   0,
			expectedURL:    "/admin",
			expectedHeaders: map[string]interface{}{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user:password")),
			},
			expectedAuth: &RequestAuth{
				Kind:     "Basic",
				Token:    base64.StdEncoding.EncodeToString([]byte("user:password")),
				User:     "user",
				Password: "password",
			},
		},
		{
			name:   "Basic authentication with username only",
			method: "GET",
			path:   "/admin",
			headers: map[string][]string{
				"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte("user"))},
			},
			contentLength:  0,
			authKey:        "Authorization",
			expectedMethod: "GET",
			expectedPath:   []string{"admin"},
			expectedSize:   0,
			expectedURL:    "/admin",
			expectedHeaders: map[string]interface{}{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user")),
			},
			expectedAuth: &RequestAuth{
				Kind:  "Basic",
				Token: base64.StdEncoding.EncodeToString([]byte("user")),
				User:  "user",
			},
		},
		{
			name:   "Token without kind (raw token)",
			method: "GET",
			path:   "/api",
			headers: map[string][]string{
				"Authorization": {"abc123xyz"},
			},
			contentLength:  0,
			authKey:        "Authorization",
			expectedMethod: "GET",
			expectedPath:   []string{"api"},
			expectedSize:   0,
			expectedURL:    "/api",
			expectedHeaders: map[string]interface{}{
				"Authorization": "abc123xyz",
			},
			expectedAuth: &RequestAuth{
				Token: "abc123xyz",
			},
		},
		{
			name:   "Custom auth header",
			method: "GET",
			path:   "/api",
			headers: map[string][]string{
				"X-Api-Key": {"Bearer secret-token"},
			},
			contentLength:  0,
			authKey:        "X-Api-Key",
			expectedMethod: "GET",
			expectedPath:   []string{"api"},
			expectedSize:   0,
			expectedURL:    "/api",
			expectedHeaders: map[string]interface{}{
				"X-Api-Key": "Bearer secret-token",
			},
			expectedAuth: &RequestAuth{
				Kind:  "Bearer",
				Token: "secret-token",
			},
		},
		{
			name:            "Root path",
			method:          "GET",
			path:            "/",
			headers:         map[string][]string{},
			contentLength:   0,
			authKey:         "Authorization",
			expectedMethod:  "GET",
			expectedPath:    []string{""},
			expectedSize:    0,
			expectedURL:     "/",
			expectedHeaders: map[string]interface{}{},
			expectedAuth:    nil,
		},
		{
			name:            "Deep nested path",
			method:          "GET",
			path:            "/api/v1/users/123/posts/456",
			headers:         map[string][]string{},
			contentLength:   0,
			authKey:         "Authorization",
			expectedMethod:  "GET",
			expectedPath:    []string{"api", "v1", "users", "123", "posts", "456"},
			expectedSize:    0,
			expectedURL:     "/api/v1/users/123/posts/456",
			expectedHeaders: map[string]interface{}{},
			expectedAuth:    nil,
		},
		{
			name:   "Bearer token with extra whitespace",
			method: "GET",
			path:   "/api",
			headers: map[string][]string{
				"Authorization": {"Bearer   token-with-spaces  "},
			},
			contentLength:  0,
			authKey:        "Authorization",
			expectedMethod: "GET",
			expectedPath:   []string{"api"},
			expectedSize:   0,
			expectedURL:    "/api",
			expectedHeaders: map[string]interface{}{
				"Authorization": "Bearer   token-with-spaces  ",
			},
			expectedAuth: &RequestAuth{
				Kind:  "Bearer",
				Token: "token-with-spaces",
			},
		},
		{
			name:   "Case insensitive Basic auth",
			method: "GET",
			path:   "/admin",
			headers: map[string][]string{
				"Authorization": {"basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))},
			},
			contentLength:  0,
			authKey:        "Authorization",
			expectedMethod: "GET",
			expectedPath:   []string{"admin"},
			expectedSize:   0,
			expectedURL:    "/admin",
			expectedHeaders: map[string]interface{}{
				"Authorization": "basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret")),
			},
			expectedAuth: &RequestAuth{
				Kind:     "basic",
				Token:    base64.StdEncoding.EncodeToString([]byte("admin:secret")),
				User:     "admin",
				Password: "secret",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest(tc.method, tc.path, nil)
			for k, values := range tc.headers {
				for _, v := range values {
					req.Header.Add(k, v)
				}
			}
			req.ContentLength = tc.contentLength

			// Call NewInfo
			info := NewInfo(req, tc.authKey)

			// Verify method
			if info.Request.Method != tc.expectedMethod {
				t.Errorf("Method: expected %q, got %q", tc.expectedMethod, info.Request.Method)
			}

			// Verify path
			if !reflect.DeepEqual(info.Request.Path, tc.expectedPath) {
				t.Errorf("Path: expected %v, got %v", tc.expectedPath, info.Request.Path)
			}

			// Verify size
			if info.Request.Size != tc.expectedSize {
				t.Errorf("Size: expected %d, got %d", tc.expectedSize, info.Request.Size)
			}

			// Verify URL
			if info.URL != tc.expectedURL {
				t.Errorf("URL: expected %q, got %q", tc.expectedURL, info.URL)
			}

			// Verify headers
			if !reflect.DeepEqual(info.Request.Headers, tc.expectedHeaders) {
				t.Errorf("Headers: expected %+v, got %+v", tc.expectedHeaders, info.Request.Headers)
			}

			// Verify auth
			if tc.expectedAuth == nil {
				if info.Request.Auth != nil {
					t.Errorf("Auth: expected nil, got %+v", info.Request.Auth)
				}
			} else {
				if info.Request.Auth == nil {
					t.Errorf("Auth: expected %+v, got nil", tc.expectedAuth)
				} else {
					if info.Request.Auth.Kind != tc.expectedAuth.Kind {
						t.Errorf("Auth.Kind: expected %q, got %q", tc.expectedAuth.Kind, info.Request.Auth.Kind)
					}
					if info.Request.Auth.Token != tc.expectedAuth.Token {
						t.Errorf("Auth.Token: expected %q, got %q", tc.expectedAuth.Token, info.Request.Auth.Token)
					}
					if info.Request.Auth.User != tc.expectedAuth.User {
						t.Errorf("Auth.User: expected %q, got %q", tc.expectedAuth.User, info.Request.Auth.User)
					}
					if info.Request.Auth.Password != tc.expectedAuth.Password {
						t.Errorf("Auth.Password: expected %q, got %q", tc.expectedAuth.Password, info.Request.Auth.Password)
					}
				}
			}
		})
	}
}

func TestRequestWithInfo(t *testing.T) {
	testCases := []struct {
		name     string
		setupReq func() *http.Request
		info     *Info
	}{
		{
			name: "Add info to request context",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			info: &Info{
				Request: RequestInfo{
					Method: "GET",
					Path:   []string{"test"},
				},
				URL: "/test",
			},
		},
		{
			name: "Add info with auth data",
			setupReq: func() *http.Request {
				return httptest.NewRequest("POST", "/api/data", nil)
			},
			info: &Info{
				Request: RequestInfo{
					Method: "POST",
					Path:   []string{"api", "data"},
					Auth: &RequestAuth{
						Kind:  "Bearer",
						Token: "test-token",
					},
				},
				URL: "/api/data",
			},
		},
		{
			name: "Add info with JWT data",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/protected", nil)
			},
			info: &Info{
				Request: RequestInfo{
					Method: "GET",
					Path:   []string{"protected"},
				},
				JWT: map[string]interface{}{
					"sub": "user123",
					"exp": 1234567890,
				},
				URL: "/protected",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupReq()

			// Add info to request
			reqWithInfo := tc.info.RequestWithInfo(req)

			// Verify it's a new request with updated context
			if reqWithInfo == req {
				t.Error("Expected new request instance, got same instance")
			}

			// Verify info can be retrieved from context
			retrievedInfo := GetInfo(reqWithInfo)
			if retrievedInfo == nil {
				t.Fatal("Expected info to be retrievable from context, got nil")
			}

			// Verify it's the same info object
			if retrievedInfo != tc.info {
				t.Error("Retrieved info is not the same object as the one added")
			}

			// Verify info content matches
			if retrievedInfo.URL != tc.info.URL {
				t.Errorf("URL: expected %q, got %q", tc.info.URL, retrievedInfo.URL)
			}
			if retrievedInfo.Request.Method != tc.info.Request.Method {
				t.Errorf("Method: expected %q, got %q", tc.info.Request.Method, retrievedInfo.Request.Method)
			}
		})
	}
}

func TestGetInfo(t *testing.T) {
	testCases := []struct {
		name     string
		setupReq func() *http.Request
		expected *Info
	}{
		{
			name: "Get info from request with context",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				info := &Info{
					Request: RequestInfo{
						Method: "GET",
						Path:   []string{"test"},
					},
					URL: "/test",
				}
				return info.RequestWithInfo(req)
			},
			expected: &Info{
				Request: RequestInfo{
					Method: "GET",
					Path:   []string{"test"},
				},
				URL: "/test",
			},
		},
		{
			name: "Get info from request without context",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			expected: nil,
		},
		{
			name: "Get info with auth data",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("POST", "/api", nil)
				info := &Info{
					Request: RequestInfo{
						Method: "POST",
						Path:   []string{"api"},
						Auth: &RequestAuth{
							Kind:  "Bearer",
							Token: "abc123",
						},
					},
					URL: "/api",
				}
				return info.RequestWithInfo(req)
			},
			expected: &Info{
				Request: RequestInfo{
					Method: "POST",
					Path:   []string{"api"},
					Auth: &RequestAuth{
						Kind:  "Bearer",
						Token: "abc123",
					},
				},
				URL: "/api",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupReq()
			info := GetInfo(req)

			if tc.expected == nil {
				if info != nil {
					t.Errorf("Expected nil, got %+v", info)
				}
				return
			}

			if info == nil {
				t.Fatal("Expected info, got nil")
			}

			// Verify content
			if info.URL != tc.expected.URL {
				t.Errorf("URL: expected %q, got %q", tc.expected.URL, info.URL)
			}
			if info.Request.Method != tc.expected.Request.Method {
				t.Errorf("Method: expected %q, got %q", tc.expected.Request.Method, info.Request.Method)
			}
			if !reflect.DeepEqual(info.Request.Path, tc.expected.Request.Path) {
				t.Errorf("Path: expected %v, got %v", tc.expected.Request.Path, info.Request.Path)
			}
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	testCases := []struct {
		name          string
		setupReq      func() *http.Request
		authHeader    string
		expectedToken string
		expectedOk    bool
	}{
		{
			name: "Valid bearer token",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer my-secret-token")
				return req
			},
			authHeader:    "Authorization",
			expectedToken: "my-secret-token",
			expectedOk:    true,
		},
		{
			name: "Valid bearer token with different case",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "bearer lowercase-token")
				return req
			},
			authHeader:    "Authorization",
			expectedToken: "lowercase-token",
			expectedOk:    true,
		},
		{
			name: "Valid bearer token with BEARER uppercase",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "BEARER UPPERCASE-TOKEN")
				return req
			},
			authHeader:    "Authorization",
			expectedToken: "UPPERCASE-TOKEN",
			expectedOk:    true,
		},
		{
			name: "No authorization header",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			authHeader:    "Authorization",
			expectedToken: "",
			expectedOk:    false,
		},
		{
			name: "Authorization header without space",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "BearerNoSpace")
				return req
			},
			authHeader:    "Authorization",
			expectedToken: "",
			expectedOk:    false,
		},
		{
			name: "Non-bearer auth type",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
				return req
			},
			authHeader:    "Authorization",
			expectedToken: "",
			expectedOk:    false,
		},
		{
			name: "Empty bearer token",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer ")
				return req
			},
			authHeader:    "Authorization",
			expectedToken: "",
			expectedOk:    true,
		},
		{
			name: "Custom auth header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Api-Key", "Bearer custom-header-token")
				return req
			},
			authHeader:    "X-Api-Key",
			expectedToken: "custom-header-token",
			expectedOk:    true,
		},
		{
			name: "Bearer token with JWT format",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U")
				return req
			},
			authHeader:    "Authorization",
			expectedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			expectedOk:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupReq()
			info := &Info{} // Info doesn't need to be populated for this test

			token, ok := info.GetBearerToken(req, tc.authHeader)

			if ok != tc.expectedOk {
				t.Errorf("Expected ok=%v, got ok=%v", tc.expectedOk, ok)
			}

			tokenStr := string(token)
			if tokenStr != tc.expectedToken {
				t.Errorf("Expected token %q, got %q", tc.expectedToken, tokenStr)
			}
		})
	}
}

func TestNewInfo_EdgeCases(t *testing.T) {
	t.Run("Invalid base64 in Basic auth is ignored", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic not-valid-base64!!!")

		info := NewInfo(req, "Authorization")

		if info.Request.Auth == nil {
			t.Fatal("Expected auth to be set")
		}
		if info.Request.Auth.Kind != "Basic" {
			t.Errorf("Expected Kind=Basic, got %q", info.Request.Auth.Kind)
		}
		// User and Password should not be set because decoding failed
		if info.Request.Auth.User != "" {
			t.Errorf("Expected empty User, got %q", info.Request.Auth.User)
		}
		if info.Request.Auth.Password != "" {
			t.Errorf("Expected empty Password, got %q", info.Request.Auth.Password)
		}
	})

	t.Run("Path with query parameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users?id=123&sort=name", nil)
		info := NewInfo(req, "Authorization")

		expectedURL := "/api/users?id=123&sort=name"
		if info.URL != expectedURL {
			t.Errorf("Expected URL %q, got %q", expectedURL, info.URL)
		}

		expectedPath := []string{"api", "users"}
		if !reflect.DeepEqual(info.Request.Path, expectedPath) {
			t.Errorf("Expected path %v, got %v", expectedPath, info.Request.Path)
		}
	})

	t.Run("Empty headers map is initialized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		info := NewInfo(req, "Authorization")

		if info.Request.Headers == nil {
			t.Error("Expected Headers map to be initialized, got nil")
		}
	})
}

func TestNewInfo_BlockedHeaders(t *testing.T) {
	testCases := []struct {
		name                   string
		setupContext           func(*http.Request) *http.Request
		expectedBlockedHeaders map[string]interface{}
		expectNil              bool
	}{
		{
			name: "No blocked headers in context",
			setupContext: func(req *http.Request) *http.Request {
				// Don't add anything to context
				return req
			},
			expectedBlockedHeaders: nil,
			expectNil:              true,
		},
		{
			name: "Empty map in context",
			setupContext: func(req *http.Request) *http.Request {
				ctx := req.Context()
				blocked := make(map[string]interface{})
				return req.WithContext(context.WithValue(ctx, CtxBlockedHeadersKey, blocked))
			},
			expectedBlockedHeaders: nil,
			expectNil:              true,
		},
		{
			name: "Populated blocked headers in context - single values",
			setupContext: func(req *http.Request) *http.Request {
				ctx := req.Context()
				blocked := map[string]interface{}{
					"X-Restrego-User": "user123",
					"X-Restrego-Role": "admin",
				}
				return req.WithContext(context.WithValue(ctx, CtxBlockedHeadersKey, blocked))
			},
			expectedBlockedHeaders: map[string]interface{}{
				"X-Restrego-User": "user123",
				"X-Restrego-Role": "admin",
			},
			expectNil: false,
		},
		{
			name: "Populated blocked headers in context - mixed types",
			setupContext: func(req *http.Request) *http.Request {
				ctx := req.Context()
				blocked := map[string]interface{}{
					"X-Restrego-Single":   "value1",
					"X-Restrego-Multiple": []string{"val1", "val2", "val3"},
				}
				return req.WithContext(context.WithValue(ctx, CtxBlockedHeadersKey, blocked))
			},
			expectedBlockedHeaders: map[string]interface{}{
				"X-Restrego-Single":   "value1",
				"X-Restrego-Multiple": []string{"val1", "val2", "val3"},
			},
			expectNil: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create base request
			req := httptest.NewRequest("GET", "/api/test", nil)

			// Setup context if needed
			req = tc.setupContext(req)

			// Call NewInfo
			info := NewInfo(req, "Authorization")

			// Verify blocked headers
			if tc.expectNil {
				if info.Request.BlockedHeaders != nil {
					t.Errorf("Expected BlockedHeaders to be nil, got %+v", info.Request.BlockedHeaders)
				}
			} else {
				if info.Request.BlockedHeaders == nil {
					t.Error("Expected BlockedHeaders to be populated, got nil")
				} else {
					if !reflect.DeepEqual(info.Request.BlockedHeaders, tc.expectedBlockedHeaders) {
						t.Errorf("BlockedHeaders mismatch:\nexpected: %+v\ngot:      %+v",
							tc.expectedBlockedHeaders, info.Request.BlockedHeaders)
					}
				}
			}
		})
	}
}

func TestRequestInfo_JSONMarshal_BlockedHeaders(t *testing.T) {
	t.Run("JSON includes blocked_headers field when populated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := req.Context()
		blocked := map[string]interface{}{
			"X-Restrego-Test": "value1",
		}
		req = req.WithContext(context.WithValue(ctx, CtxBlockedHeadersKey, blocked))

		info := NewInfo(req, "Authorization")

		// Marshal to JSON
		data, err := json.Marshal(info.Request)
		if err != nil {
			t.Fatalf("Failed to marshal RequestInfo: %v", err)
		}

		jsonStr := string(data)

		// Verify blocked_headers field is present
		if !contains(jsonStr, "blocked_headers") {
			t.Error("Expected JSON to contain 'blocked_headers' field")
		}

		// Verify the field contains our test data
		if !contains(jsonStr, "X-Restrego-Test") {
			t.Error("Expected JSON to contain 'X-Restrego-Test' in blocked_headers")
		}
	})

	t.Run("JSON omits blocked_headers field when empty (omitempty)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		// Don't add anything to context

		info := NewInfo(req, "Authorization")

		// Marshal to JSON
		data, err := json.Marshal(info.Request)
		if err != nil {
			t.Fatalf("Failed to marshal RequestInfo: %v", err)
		}

		jsonStr := string(data)

		// Verify blocked_headers field is NOT present (omitempty works)
		if contains(jsonStr, "blocked_headers") {
			t.Error("Expected JSON to NOT contain 'blocked_headers' field when empty")
		}
	})

	t.Run("JSON structure with both single and multiple values", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := req.Context()
		blocked := map[string]interface{}{
			"X-Restrego-Single":   "single-value",
			"X-Restrego-Multiple": []string{"val1", "val2"},
		}
		req = req.WithContext(context.WithValue(ctx, CtxBlockedHeadersKey, blocked))

		info := NewInfo(req, "Authorization")

		// Marshal to JSON
		data, err := json.Marshal(info.Request)
		if err != nil {
			t.Fatalf("Failed to marshal RequestInfo: %v", err)
		}

		// Unmarshal back to verify structure
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}

		// Verify blocked_headers exists
		blockedHeadersRaw, exists := result["blocked_headers"]
		if !exists {
			t.Fatal("Expected 'blocked_headers' field in JSON")
		}

		blockedHeaders, ok := blockedHeadersRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected blocked_headers to be map[string]interface{}, got %T", blockedHeadersRaw)
		}

		// Verify single value
		if single, ok := blockedHeaders["X-Restrego-Single"]; !ok {
			t.Error("Expected X-Restrego-Single in blocked_headers")
		} else if single != "single-value" {
			t.Errorf("Expected X-Restrego-Single to be 'single-value', got %v", single)
		}

		// Verify multiple values
		if multiple, ok := blockedHeaders["X-Restrego-Multiple"]; !ok {
			t.Error("Expected X-Restrego-Multiple in blocked_headers")
		} else {
			multipleSlice, ok := multiple.([]interface{})
			if !ok {
				t.Errorf("Expected X-Restrego-Multiple to be []interface{}, got %T", multiple)
			} else if len(multipleSlice) != 2 {
				t.Errorf("Expected 2 values in X-Restrego-Multiple, got %d", len(multipleSlice))
			}
		}
	})
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

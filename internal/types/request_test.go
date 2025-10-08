package types

import (
	"encoding/base64"
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

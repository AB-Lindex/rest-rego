package router

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

const test_auth_key = "Test-Auth-Key"

func TestWrapHandler(t *testing.T) {
	testCases := []struct {
		name            string
		method          string
		path            string
		headers         map[string]string
		handlerFunc     func(w http.ResponseWriter, r *http.Request, t *testing.T)
		expectedStatus  int
		expectedHeaders map[string]string
		expectedBody    string
	}{
		{
			name:   "Success response",
			method: "GET",
			path:   "/test",
			handlerFunc: func(w http.ResponseWriter, r *http.Request, t *testing.T) {
				// Verify info is attached to the request
				//lint:ignore S1021 This is a test
				var info *types.Info
				info = types.GetInfo(r)
				if info == nil {
					t.Error("Expected info to be attached to request context")
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:   "Error response",
			method: "POST",
			path:   "/error",
			handlerFunc: func(w http.ResponseWriter, r *http.Request, t *testing.T) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("error message"))
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "error message",
		},
		{
			name:   "Empty response",
			method: "DELETE",
			path:   "/empty",
			handlerFunc: func(w http.ResponseWriter, r *http.Request, t *testing.T) {
				w.WriteHeader(http.StatusNoContent)
			},
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:   "Path response",
			method: "GET",
			path:   "/test1/test2",
			handlerFunc: func(w http.ResponseWriter, r *http.Request, t *testing.T) {
				// Verify info is attached to the request
				//lint:ignore S1021 This is a test
				var info *types.Info
				info = types.GetInfo(r)
				if info == nil {
					t.Error("Expected info to be attached to request context")
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(strings.Join(info.Request.Path, ",")))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "test1,test2",
		},
		{
			name:   "Header response",
			method: "GET",
			path:   "/test",
			headers: map[string]string{
				"X-Test-Header": "test-value",
			},
			handlerFunc: func(w http.ResponseWriter, r *http.Request, t *testing.T) {
				// Verify info is attached to the request
				//lint:ignore S1021 This is a test
				var info *types.Info
				info = types.GetInfo(r)
				if info == nil {
					t.Error("Expected info to be attached to request context")
					return
				}

				for k, v := range info.Request.Headers {
					switch vv := v.(type) {
					case string:
						w.Header().Set(k, vv)
					case []string:
						w.Header().Set(k, strings.Join(vv, ","))
					}
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(strings.Join(info.Request.Path, ",")))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "test",
			expectedHeaders: map[string]string{
				"X-Test-Header": "test-value",
			},
		},
		{
			name:   "Basic auth response",
			method: "GET",
			path:   "/test1/test2",
			headers: map[string]string{
				test_auth_key: "Basic " + base64.StdEncoding.EncodeToString([]byte("test-user:test-password")),
			},
			handlerFunc: func(w http.ResponseWriter, r *http.Request, t *testing.T) {
				// Verify info is attached to the request
				//lint:ignore S1021 This is a test
				var info *types.Info
				info = types.GetInfo(r)
				if info == nil {
					t.Error("Expected info to be attached to request context")
					return
				}
				if info.Request.Auth == nil {
					t.Error("Expected auth to be attached to request context")
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(info.Request.Auth.Kind))
				w.Write([]byte(","))
				w.Write([]byte(info.Request.Auth.User))
				w.Write([]byte(","))
				w.Write([]byte(info.Request.Auth.Password))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Basic,test-user,test-password",
		},
		{
			name:   "Bearer auth response",
			method: "GET",
			path:   "/test1/test2",
			headers: map[string]string{
				test_auth_key: "Bearer S0m3r4nd0mT0k3n",
			},
			handlerFunc: func(w http.ResponseWriter, r *http.Request, t *testing.T) {
				// Verify info is attached to the request
				//lint:ignore S1021 This is a test
				var info *types.Info
				info = types.GetInfo(r)
				if info == nil {
					t.Error("Expected info to be attached to request context")
					return
				}
				if info.Request.Auth == nil {
					t.Error("Expected auth to be attached to request context")
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(info.Request.Auth.Kind))
				w.Write([]byte(","))
				w.Write([]byte(info.Request.Auth.Token))
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Bearer,S0m3r4nd0mT0k3n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock handler
			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.handlerFunc(w, r, t)
			})

			// Create proxy with test auth key
			proxy := &Proxy{
				authKey: test_auth_key,
			}

			// Create test request
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call wrapped handler
			wrappedHandler := proxy.WrapHandler(mockHandler)
			wrappedHandler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatus, rr.Code)
			}

			// Check response body
			if rr.Body.String() != tc.expectedBody {
				t.Errorf("Expected body '%s', got '%s'", tc.expectedBody, rr.Body.String())
			}

			// Check headers
			if len(tc.expectedHeaders) > 0 {
				for k, v := range tc.expectedHeaders {
					headerValue := rr.Header().Get(k)
					if headerValue != v {
						t.Errorf("Expected header %s to be %s, got %s", k, v, headerValue)
					}
				}
			}

			// Check Content-Length header
			expectedLength := len(tc.expectedBody)
			contentLength := rr.Header().Get("Content-Length")
			if contentLength != strconv.Itoa(expectedLength) {
				t.Errorf("Expected Content-Length to be %d, got %s", expectedLength, contentLength)
			}
		})
	}
}

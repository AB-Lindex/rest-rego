package e2eshared

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// bodyCache builds and caches fixed-size response bodies derived from a base string.
// The body for each requested size is constructed once and reused on all subsequent calls.
type bodyCache struct {
	base  string
	mu    sync.RWMutex
	cache map[int]string
}

func newBodyCache(base string) *bodyCache {
	return &bodyCache{base: base, cache: map[int]string{}}
}

func (bc *bodyCache) get(n int) string {
	bc.mu.RLock()
	s, ok := bc.cache[n]
	bc.mu.RUnlock()
	if ok {
		return s
	}
	if len(bc.base) >= n {
		s = bc.base[:n]
	} else {
		s = bc.base + strings.Repeat("x", n-len(bc.base))
	}
	bc.mu.Lock()
	bc.cache[n] = s
	bc.mu.Unlock()
	return s
}

// BackendServer is a controllable HTTP server used by e2e tests.
// It provides a /health endpoint and allows mounting named endpoints
// via Mount, with support for query-parameter response overrides.
type BackendServer struct {
	server   *httptest.Server
	mux      *http.ServeMux
	mu       sync.Mutex
	Requests map[string][]*http.Request
}

// NewBackendServer starts a BackendServer listening on 127.0.0.1:<port>.
// It registers GET /health returning 200 OK.
func NewBackendServer(port int) (*BackendServer, error) {
	mux := http.NewServeMux()
	bs := &BackendServer{
		mux:      mux,
		Requests: make(map[string][]*http.Request),
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("BackendServer: listen on port %d: %w", port, err)
	}

	ts := httptest.NewUnstartedServer(mux)
	ts.Listener = ln
	ts.Start()
	bs.server = ts

	return bs, nil
}

// Mount registers a handler at /e2e/<name> that returns the given status and body.
// The handler captures each incoming *http.Request and supports query-parameter overrides:
//
//   - ?size=N   pad/truncate body to exactly N bytes (padded with 'x')
//   - ?cl=missing  delete Content-Length header; set Transfer-Encoding: chunked
//   - ?cl=bad      set Content-Length: notanumber (malformed)
//   - ?cl=N        set Content-Length: N regardless of actual body length
func (bs *BackendServer) Mount(name string, status int, body string) {
	path := "/e2e/" + name
	bc := newBodyCache(body)

	bs.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		bs.mu.Lock()
		bs.Requests[path] = append(bs.Requests[path], r)
		bs.mu.Unlock()

		q := r.URL.Query()
		responseBody := body

		// Apply ?size=N override (cached per size value).
		if sizeStr := q.Get("size"); sizeStr != "" {
			if n, err := strconv.Atoi(sizeStr); err == nil && n >= 0 {
				responseBody = bc.get(n)
			}
		}

		// Apply ?cl overrides.
		switch clVal := q.Get("cl"); clVal {
		case "missing":
			w.Header().Del("Content-Length")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.WriteHeader(status)
			fmt.Fprint(w, responseBody)
		case "bad":
			w.Header().Set("Content-Length", "notanumber")
			w.WriteHeader(status)
			fmt.Fprint(w, responseBody)
		case "":
			w.WriteHeader(status)
			fmt.Fprint(w, responseBody)
		default:
			// ?cl=N — explicit Content-Length regardless of actual body.
			w.Header().Set("Content-Length", clVal)
			w.WriteHeader(status)
			fmt.Fprint(w, responseBody)
		}
	})
}

// URL returns the base URL of the backend server.
func (bs *BackendServer) URL() string {
	return bs.server.URL
}

// Close shuts down the backend server.
func (bs *BackendServer) Close() {
	bs.server.Close()
}

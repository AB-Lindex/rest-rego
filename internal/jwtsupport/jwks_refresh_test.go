package jwtsupport

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type jwksLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *jwksLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *jwksLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestLoadJWKS_EmptySet(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"keys":[]}`))
		}))
		defer server.Close()

		j := &JWTSupport{wellknownList: []*wellKnownData{{
			JwksURI: server.URL, sourceURL: server.URL,
		}}}
		j.LoadJWKS()
		if len(j.JWKS) != 0 {
			t.Fatal("empty HTTP JWKS must not be loaded")
		}
	})

	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "jwks.json")
		if err := os.WriteFile(path, []byte(`{"keys":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		url := "file://" + path
		j := &JWTSupport{wellknownList: []*wellKnownData{{
			JwksURI: url, sourceURL: url, isLocalFile: true,
		}}}
		j.LoadJWKS()
		if len(j.JWKS) != 0 {
			t.Fatal("empty file JWKS must not be loaded")
		}
	})
}

func TestJWKSRefresh_RetainsKeysAndRecovers(t *testing.T) {
	publicKey, privateKey := newRSAJWK(t, "stable-key")
	set := jwk.NewSet()
	if err := set.AddKey(publicKey); err != nil {
		t.Fatal(err)
	}
	validBody, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	signedToken := signTestToken(t, "test-audience", privateKey)

	type response struct {
		body   string
		status int
	}
	var currentResponse atomic.Value
	currentResponse.Store(response{string(validBody), http.StatusOK})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := currentResponse.Load().(response)
		w.WriteHeader(current.status)
		w.Write([]byte(current.body))
	}))
	defer server.Close()

	j := &JWTSupport{wellknownList: []*wellKnownData{{
		JwksURI: server.URL, sourceURL: server.URL,
	}}}
	j.LoadJWKS()
	if len(j.JWKS) != 1 {
		t.Fatal("initial JWKS must load")
	}

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"empty keys", `{"keys":[]}`, http.StatusOK},
		{"null keys", `{"keys":null}`, http.StatusOK},
		{"blank body", "", http.StatusOK},
		{"malformed JSON", "{", http.StatusOK},
		{"HTTP error", "unavailable", http.StatusServiceUnavailable},
		{"no keys after enrichment", `{"keys":[{"kty":"RSA","kid":"unusable","n":"AQAB","e":"AQAB"}]}`, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentResponse.Store(response{tt.body, tt.status})
			if _, err := j.cache.Refresh(context.Background(), server.URL); err == nil {
				t.Fatal("invalid refresh must return an error")
			}
			cached, err := j.cache.Get(context.Background(), server.URL)
			if err != nil {
				t.Fatal(err)
			}
			if cached.Len() != 1 {
				t.Fatalf("expected one retained key, got %d", cached.Len())
			}
			if _, ok := cached.LookupKeyID("stable-key"); !ok {
				t.Fatal("last good key was lost")
			}
			if _, err := jwt.Parse(signedToken, jwt.WithKeySet(cached), jwt.WithAudience("test-audience")); err != nil {
				t.Fatalf("same token must still verify after rejected refresh: %v", err)
			}
		})
	}

	currentResponse.Store(response{string(validBody), http.StatusOK})
	if _, err := j.cache.Refresh(context.Background(), server.URL); err != nil {
		t.Fatalf("refresh must recover when valid keys are returned: %v", err)
	}
}

func TestJWKSRefresh_BackgroundLogging(t *testing.T) {
	var logs jwksLogBuffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)

	publicKey, _ := newRSAJWK(t, "stable-key")
	set := jwk.NewSet()
	if err := set.AddKey(publicKey); err != nil {
		t.Fatal(err)
	}
	validBody, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	var body atomic.Value
	body.Store(string(validBody))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body.Load().(string)))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cache := jwk.NewCache(ctx, jwk.WithRefreshWindow(time.Second),
		jwk.WithErrSink(jwksRefreshErrorSink{}))
	if err := cache.Register(server.URL, jwk.WithRefreshInterval(time.Second),
		jwk.WithPostFetcher(&wellKnownData{})); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Get(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `"level":"INFO","msg":"jwtsupport: JWKS fetched and validated"`) ||
		!strings.Contains(logs.String(), `"url":"`+server.URL+`","keys":1`) {
		t.Fatalf("missing successful fetch log with URL and key count: %s", logs.String())
	}

	body.Store(`{"keys":[]}`)
	waitForLog := func(message string) {
		t.Helper()
		timeout := time.NewTimer(5 * time.Second)
		defer timeout.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			if strings.Contains(logs.String(), message) {
				return
			}
			select {
			case <-timeout.C:
				t.Fatalf("missing background refresh log %q: %s", message, logs.String())
			case <-tick.C:
			}
		}
	}
	waitForLog(`"level":"ERROR","msg":"jwtsupport: JWKS refresh failed; keeping cached keys"`)
	if !strings.Contains(logs.String(), "contains no keys") || !strings.Contains(logs.String(), server.URL) {
		t.Fatalf("refresh failure must identify the URL and cause: %s", logs.String())
	}
	cached, err := cache.Get(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cached.LookupKeyID("stable-key"); !ok {
		t.Fatal("failed background refresh must preserve the last good key")
	}

	if err := publicKey.Set(jwk.KeyIDKey, "recovered-key"); err != nil {
		t.Fatal(err)
	}
	recoveredBody, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	body.Store(string(recoveredBody))
	deadline := time.Now().Add(5 * time.Second)
	for {
		cached, err := cache.Get(ctx, server.URL)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := cached.LookupKeyID("recovered-key"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background refresh did not recover")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Count(logs.String(), `"msg":"jwtsupport: JWKS fetched and validated"`) < 2 {
		t.Fatalf("successful background refresh must be logged: %s", logs.String())
	}
}

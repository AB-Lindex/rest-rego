package basicauth

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/AB-Lindex/rest-rego/internal/types"
)

// --- test fixtures ---

var (
	onceTestCreds sync.Once
	testHashCreds credMap
)

// getTestCreds returns a credMap with a single entry "alice":"correct-horse"
// (bcrypt cost 10). The hash is generated once per test run.
func getTestCreds() credMap {
	onceTestCreds.Do(func() {
		hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse"), 10)
		if err != nil {
			panic(fmt.Sprintf("bcrypt setup for tests failed: %v", err))
		}
		testHashCreds = credMap{"alice": string(hash)}
	})
	return testHashCreds
}

// newTestProvider builds a BasicAuthProvider with pre-loaded credentials,
// bypassing New() so no file I/O or goroutines are started.
func newTestProvider(creds credMap, permissive bool) *BasicAuthProvider {
	b := &BasicAuthProvider{permissive: permissive}
	b.creds.Store(&creds)
	return b
}

// makeBasicInfo returns a types.Info carrying Basic auth credentials.
func makeBasicInfo(user, password string) *types.Info {
	return &types.Info{
		Request: types.RequestInfo{
			Auth: &types.RequestAuth{Kind: "Basic", User: user, Password: password},
		},
	}
}

// --- Authenticate tests ---

func TestAuthenticate_NoAuthHeader_Anonymous(t *testing.T) {
	provider := newTestProvider(getTestCreds(), false)
	info := &types.Info{} // Auth is nil

	err := provider.Authenticate(info, &http.Request{})

	if err != nil {
		t.Errorf("expected nil (anonymous passthrough), got %v", err)
	}
}

func TestAuthenticate_NonBasicAuth_Anonymous(t *testing.T) {
	provider := newTestProvider(getTestCreds(), false)
	info := &types.Info{
		Request: types.RequestInfo{
			Auth: &types.RequestAuth{Kind: "Bearer", Token: "eyJtoken"},
		},
	}

	err := provider.Authenticate(info, &http.Request{})

	if err != nil {
		t.Errorf("expected nil (anonymous passthrough for Bearer), got %v", err)
	}
}

func TestAuthenticate_ValidCredentials_SuccessAndPasswordCleared(t *testing.T) {
	provider := newTestProvider(getTestCreds(), false)
	info := makeBasicInfo("alice", "correct-horse")

	err := provider.Authenticate(info, &http.Request{})

	if err != nil {
		t.Errorf("expected nil for valid credentials, got %v", err)
	}
	if info.Request.Auth.Password != "" {
		t.Errorf("expected password to be cleared after success, got %q", info.Request.Auth.Password)
	}
}

func TestAuthenticate_WrongPassword_FailsEvenWhenPermissive(t *testing.T) {
	for _, permissive := range []bool{false, true} {
		t.Run(fmt.Sprintf("permissive=%v", permissive), func(t *testing.T) {
			provider := newTestProvider(getTestCreds(), permissive)
			info := makeBasicInfo("alice", "wrong-password")

			err := provider.Authenticate(info, &http.Request{})

			if !errors.Is(err, types.ErrAuthenticationFailed) {
				t.Errorf("expected ErrAuthenticationFailed regardless of permissive=%v, got %v", permissive, err)
			}
		})
	}
}

func TestAuthenticate_UnknownUser_StrictMode(t *testing.T) {
	provider := newTestProvider(getTestCreds(), false)
	info := makeBasicInfo("unknown", "password")

	err := provider.Authenticate(info, &http.Request{})

	if !errors.Is(err, types.ErrAuthenticationFailed) {
		t.Errorf("expected ErrAuthenticationFailed for unknown user in strict mode, got %v", err)
	}
}

func TestAuthenticate_UnknownUser_PermissiveMode(t *testing.T) {
	provider := newTestProvider(getTestCreds(), true)
	info := makeBasicInfo("unknown", "password")

	err := provider.Authenticate(info, &http.Request{})

	if err != nil {
		t.Errorf("expected nil for unknown user in permissive mode, got %v", err)
	}
}

func TestAuthenticate_PasswordAlwaysClearedOnReturn(t *testing.T) {
	provider := newTestProvider(getTestCreds(), false)

	cases := []struct {
		name     string
		user     string
		password string
	}{
		{"valid credentials", "alice", "correct-horse"},
		{"wrong password", "alice", "wrong"},
		{"unknown user", "nobody", "anything"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := makeBasicInfo(tc.user, tc.password)
			_ = provider.Authenticate(info, &http.Request{})
			if info.Request.Auth.Password != "" {
				t.Errorf("password not cleared after Authenticate; got %q", info.Request.Auth.Password)
			}
		})
	}
}

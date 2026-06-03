package basicauth

import (
	"hash/maphash"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/dgraph-io/ristretto/v2"
)

type credMap = map[string]string // username → bcrypt hash

// BasicAuthProvider authenticates requests using an Apache htpasswd file (bcrypt only).
type BasicAuthProvider struct {
	filePath   string
	creds      atomic.Pointer[credMap]
	permissive bool
	watcher    *fsnotify.Watcher
	cache      *ristretto.Cache[uint64, bool]
	seed       maphash.Seed
}

// New creates a BasicAuthProvider that reads credentials from filePath.
// Returns nil if the file cannot be loaded or contains no valid bcrypt entries.
func New(filePath string, permissive bool) *BasicAuthProvider {
	creds, err := loadFile(filePath)
	if err != nil {
		slog.Error("basicauth: failed to load credentials", "file", filePath, "error", err)
		return nil
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("basicauth: failed to create file watcher", "error", err)
		return nil
	}

	b := &BasicAuthProvider{
		filePath:   filePath,
		permissive: permissive,
		watcher:    w,
	}
	b.creds.Store(creds)

	if err := w.Add(filePath); err != nil {
		slog.Warn("basicauth: failed to watch file, hot-reload disabled", "file", filePath, "error", err)
	} else {
		go startWatcher(b)
	}

	cache, err := ristretto.NewCache(&ristretto.Config[uint64, bool]{
		NumCounters: 10000, // number of keys to track frequency of.
		MaxCost:     1000,  // maximum cost of cache (no-of-entries since we use cost=1).
		BufferItems: 64,    // number of keys per Get buffer.
	})
	if err != nil {
		slog.Warn("basicauth: failed to create cache, authentication performance may be degraded", "error", err)
	}
	b.cache = cache

	b.seed = maphash.MakeSeed()

	return b
}

// Authenticate implements types.AuthProvider.
// Missing or non-Basic Authorization header → anonymous (nil error).
// Wrong password → always ErrAuthenticationFailed, even in permissive mode.
func (b *BasicAuthProvider) Authenticate(info *types.Info, _ *http.Request) error {
	auth := info.Request.Auth
	if auth == nil || !strings.EqualFold(auth.Kind, "basic") {
		return nil // anonymous passthrough
	}

	// Always clear the password before returning so it never reaches the policy engine.
	defer func() { auth.Password = "" }()

	if auth.User == "" {
		return handleFailure(b.permissive)
	}

	creds := *b.creds.Load()
	hash, known := creds[auth.User]
	if !known {
		return handleFailure(b.permissive)
	}

	if err := b.verifyPassword(auth.User, hash, auth.Password); err != nil {
		// Wrong password is always a hard failure regardless of permissive mode (SEC-002).
		return types.ErrAuthenticationFailed
	}

	// Set authenticated user info for logging and policy evaluation
	info.User = auth.User

	return nil
}

// WWWAuthenticate implements the optional types.AuthChallenger interface.
func (b *BasicAuthProvider) WWWAuthenticate() string {
	return `Basic realm="rest-rego"`
}

// handleFailure returns nil in permissive mode, ErrAuthenticationFailed in strict mode.
func handleFailure(permissive bool) error {
	if permissive {
		return nil
	}
	return types.ErrAuthenticationFailed
}

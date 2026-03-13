package basicauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// --- shared test helpers ---

// testLogHandler captures slog records so tests can assert on log messages.
type testLogHandler struct {
	mu   sync.Mutex
	msgs []string
}

func (h *testLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = append(h.msgs, r.Message)
	return nil
}

func (h *testLogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *testLogHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *testLogHandler) containsMessage(msg string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.msgs {
		if strings.Contains(m, msg) {
			return true
		}
	}
	return false
}

// captureLogger installs a testLogHandler as the default slog logger and
// restores the previous one when the test ends. Tests using this must not
// run in parallel.
func captureLogger(t *testing.T) *testLogHandler {
	t.Helper()
	prev := slog.Default()
	h := &testLogHandler{}
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return h
}

// writeTempHtpasswd writes content to a temporary file and returns its path.
func writeTempHtpasswd(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "htpasswd-*.txt")
	if err != nil {
		t.Fatalf("os.CreateTemp: %v", err)
	}
	_, writeErr := fmt.Fprint(f, content)
	f.Close()
	if writeErr != nil {
		t.Fatalf("write temp file: %v", writeErr)
	}
	return f.Name()
}

// bcryptHash generates a bcrypt hash at the given cost. Higher costs are slower.
func bcryptHash(t *testing.T, password string, cost int) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword(cost=%d): %v", cost, err)
	}
	return string(h)
}

// --- loadFile tests ---

func TestLoadFile_ValidBcryptEntry(t *testing.T) {
	hash := bcryptHash(t, "secret", 10)
	path := writeTempHtpasswd(t, "alice:"+hash+"\n")

	creds, err := loadFile(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := (*creds)["alice"]; !ok || got != hash {
		t.Errorf("expected alice's hash %q in creds, got %q (ok=%v)", hash, got, ok)
	}
}

func TestLoadFile_CommentLineSkipped(t *testing.T) {
	hash := bcryptHash(t, "secret", 10)
	path := writeTempHtpasswd(t, "# this is a comment\nalice:"+hash+"\n")

	creds, err := loadFile(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*creds) != 1 {
		t.Errorf("expected 1 entry, got %d", len(*creds))
	}
}

func TestLoadFile_EmptyLineSkipped(t *testing.T) {
	hash := bcryptHash(t, "secret", 10)
	path := writeTempHtpasswd(t, "\n\nalice:"+hash+"\n\n")

	creds, err := loadFile(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*creds) != 1 {
		t.Errorf("expected 1 entry, got %d", len(*creds))
	}
}

func TestLoadFile_MD5HashSkipped(t *testing.T) {
	// Must not run in parallel: captureLogger replaces the global slog default.
	logs := captureLogger(t)
	path := writeTempHtpasswd(t, "alice:$apr1$xyz$abc123placeholder\n")

	_, err := loadFile(path)

	if !errors.Is(err, ErrNoValidCredentials) {
		t.Errorf("expected ErrNoValidCredentials, got %v", err)
	}
	if !logs.containsMessage("skipping MD5 hash") {
		t.Errorf("expected WARN log containing 'skipping MD5 hash'; got messages: %v", logs.msgs)
	}
}

func TestLoadFile_SHA1HashSkipped(t *testing.T) {
	path := writeTempHtpasswd(t, "alice:{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=\n")

	_, err := loadFile(path)

	if !errors.Is(err, ErrNoValidCredentials) {
		t.Errorf("expected ErrNoValidCredentials, got %v", err)
	}
}

func TestLoadFile_NoColonLineSkipped(t *testing.T) {
	path := writeTempHtpasswd(t, "nocolonentry\n")

	_, err := loadFile(path)

	if !errors.Is(err, ErrNoValidCredentials) {
		t.Errorf("expected ErrNoValidCredentials, got %v", err)
	}
}

func TestLoadFile_EmptyFile_ReturnsError(t *testing.T) {
	path := writeTempHtpasswd(t, "")

	_, err := loadFile(path)

	if !errors.Is(err, ErrNoValidCredentials) {
		t.Errorf("expected ErrNoValidCredentials for empty file, got %v", err)
	}
}

func TestLoadFile_CostTooLow_Rejected(t *testing.T) {
	// bcrypt cost 9 is below minAllowedCost (10); entry must be rejected (REQ-007).
	hash := bcryptHash(t, "secret", 9)
	path := writeTempHtpasswd(t, "alice:"+hash+"\n")

	_, err := loadFile(path)

	if !errors.Is(err, ErrNoValidCredentials) {
		t.Errorf("expected ErrNoValidCredentials for cost-9 hash, got %v", err)
	}
}

func TestLoadFile_CostEleven_LoadsWithWarning(t *testing.T) {
	// Must not run in parallel: captureLogger replaces the global slog default.
	// bcrypt cost 11 is valid (>= 10) but below recommended 12 — loads with WARN.
	logs := captureLogger(t)
	hash := bcryptHash(t, "secret", 11)
	path := writeTempHtpasswd(t, "alice:"+hash+"\n")

	creds, err := loadFile(path)

	if err != nil {
		t.Fatalf("unexpected error for cost-11 hash: %v", err)
	}
	if _, ok := (*creds)["alice"]; !ok {
		t.Error("expected alice to be in creds for cost-11 hash")
	}
	if !logs.containsMessage("bcrypt cost below recommended") {
		t.Errorf("expected WARN log about cost below recommended; got messages: %v", logs.msgs)
	}
}

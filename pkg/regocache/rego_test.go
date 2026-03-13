package regocache

import (
	"os"
	"path/filepath"
	"testing"
)

func writePolicy(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write policy %s: %v", name, err)
	}
}

func newTestCache(t *testing.T, dir, policyFile string) *RegoCache {
	t.Helper()
	rc, err := New(dir, "*.rego", false, policyFile)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() { rc.Close() })
	rc.Watch()
	return rc
}

func TestEnvExpansion_matchingValue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TEST_REGO_SECRET", "supersecret")

	const policyFile = "request.rego"
	writePolicy(t, tmpDir, policyFile, `package testpkg

default allow := false

allow if {
	input.token == "$(TEST_REGO_SECRET)"
}
`)
	rc := newTestCache(t, tmpDir, policyFile)

	result, err := rc.Validate(policyFile, map[string]any{"token": "supersecret"})
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result)
	}
	if m["allow"] != true {
		t.Errorf("expected allow=true, got %v", m["allow"])
	}
}

func TestEnvExpansion_wrongValue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TEST_REGO_SECRET", "supersecret")

	const policyFile = "request.rego"
	writePolicy(t, tmpDir, policyFile, `package testpkg

default allow := false

allow if {
	input.token == "$(TEST_REGO_SECRET)"
}
`)
	rc := newTestCache(t, tmpDir, policyFile)

	result, err := rc.Validate(policyFile, map[string]any{"token": "wrongsecret"})
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result)
	}
	if m["allow"] != false {
		t.Errorf("expected allow=false, got %v", m["allow"])
	}
}

func TestEnvExpansion_missingVarExpandsToEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	os.Unsetenv("TEST_REGO_NONEXISTENT_XYZ")

	const policyFile = "request.rego"
	writePolicy(t, tmpDir, policyFile, `package testpkg

default allow := false

allow if {
	input.token == "$(TEST_REGO_NONEXISTENT_XYZ)"
}
`)
	rc := newTestCache(t, tmpDir, policyFile)

	// missing var expands to empty string, so only empty token matches
	result, err := rc.Validate(policyFile, map[string]any{"token": ""})
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result)
	}
	if m["allow"] != true {
		t.Errorf("expected allow=true (empty env var matches empty token), got %v", m["allow"])
	}
}

func TestEnvExpansion_multipleVars(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TEST_REGO_USER", "alice")
	t.Setenv("TEST_REGO_ROLE", "admin")

	const policyFile = "request.rego"
	writePolicy(t, tmpDir, policyFile, `package testpkg

default allow := false

allow if {
	input.user == "$(TEST_REGO_USER)"
	input.role == "$(TEST_REGO_ROLE)"
}
`)
	rc := newTestCache(t, tmpDir, policyFile)

	result, err := rc.Validate(policyFile, map[string]any{"user": "alice", "role": "admin"})
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result)
	}
	if m["allow"] != true {
		t.Errorf("expected allow=true, got %v", m["allow"])
	}

	// only one var matches
	result, err = rc.Validate(policyFile, map[string]any{"user": "alice", "role": "viewer"})
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	m, ok = result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result)
	}
	if m["allow"] != false {
		t.Errorf("expected allow=false when role doesn't match, got %v", m["allow"])
	}
}

func TestEnvExpansion_noVarsInPolicy(t *testing.T) {
	tmpDir := t.TempDir()

	const policyFile = "request.rego"
	writePolicy(t, tmpDir, policyFile, `package testpkg

default allow := true
`)
	rc := newTestCache(t, tmpDir, policyFile)

	result, err := rc.Validate(policyFile, map[string]any{})
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result)
	}
	if m["allow"] != true {
		t.Errorf("expected allow=true, got %v", m["allow"])
	}
}

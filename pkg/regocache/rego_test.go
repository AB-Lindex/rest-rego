package regocache

import (
	"os"
	"path/filepath"
	"testing"
)

func writePolicy(t testing.TB, dir, name, content string) {
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

// BenchmarkValidate measures per-evaluation allocations in the OPA policy path.
//
// Investigation: OPA's PreparedEvalQuery.Eval is suspected to accumulate internal
// state under sustained load. Run with:
//
//	go test -bench=BenchmarkValidate -benchmem -memprofile=mem.out ./pkg/regocache/
//	go tool pprof -alloc_space mem.out
func BenchmarkValidate(b *testing.B) {
	tmpDir := b.TempDir()

	const policyFile = "request.rego"
	writePolicy(b, tmpDir, policyFile, `package testpkg

default allow := false

allow if {
	input.request.method == "GET"
	input.jwt.sub != ""
}
`)

	rc, err := New(tmpDir, "*.rego", false, policyFile)
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	b.Cleanup(func() { rc.Close() })
	rc.Watch()

	input := map[string]any{
		"request": map[string]any{
			"method": "GET",
			"path":   []string{"api", "v1", "resource"},
		},
		"jwt": map[string]any{
			"sub": "user-123",
			"aud": "my-service",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := rc.Validate(policyFile, input)
		if err != nil {
			b.Fatalf("Validate() error: %v", err)
		}
		m, ok := result.(map[string]any)
		if !ok {
			b.Fatalf("result is not a map: %T", result)
		}
		if m["allow"] != true {
			b.Fatalf("expected allow=true, got %v", m["allow"])
		}
	}
}

// BenchmarkValidate_LargeInput benchmarks policy evaluation with a larger input
// that is closer to real-world JWT + request data, to surface any input marshalling
// or binding allocations that scale with input size.
func BenchmarkValidate_LargeInput(b *testing.B) {
	tmpDir := b.TempDir()

	const policyFile = "request.rego"
	writePolicy(b, tmpDir, policyFile, `package testpkg

default allow := false

allow if {
	input.request.method == "GET"
	input.jwt.sub != ""
	input.jwt.roles[_] == "reader"
}
`)

	rc, err := New(tmpDir, "*.rego", false, policyFile)
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	b.Cleanup(func() { rc.Close() })
	rc.Watch()

	input := map[string]any{
		"request": map[string]any{
			"method":  "GET",
			"path":    []string{"api", "v1", "resource", "items"},
			"headers": map[string]any{"content-type": "application/json", "x-request-id": "abc-123"},
			"id":      "req-abc-123",
		},
		"jwt": map[string]any{
			"sub":   "user-123",
			"aud":   []string{"my-service", "other-service"},
			"iss":   "https://auth.example.com",
			"email": "user@example.com",
			"roles": []string{"reader", "viewer"},
			"name":  "Test User",
			"iat":   1700000000,
			"exp":   1700003600,
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := rc.Validate(policyFile, input)
		if err != nil {
			b.Fatalf("Validate() error: %v", err)
		}
		m, ok := result.(map[string]any)
		if !ok {
			b.Fatalf("result is not a map: %T", result)
		}
		if m["allow"] != true {
			b.Fatalf("expected allow=true, got %v", m["allow"])
		}
	}
}

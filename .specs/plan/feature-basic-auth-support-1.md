---
goal: Implement Basic HTTP Authentication Support (htpasswd/bcrypt)
version: "1.0"
date_created: 2026-03-13
last_updated: 2026-03-13
owner: rest-rego team
status: 'Planned'
tags: [feature, auth, security]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

Add Basic HTTP authentication as a first-class auth provider alongside the existing Azure and JWT providers. Credentials are stored in an Apache 2.4 htpasswd file (bcrypt only). Changes are hot-reloaded via `fsnotify`. Passwords are never forwarded to the Rego policy engine.

## 1. Requirements & Constraints

- **REQ-001**: `BASIC_AUTH_FILE` env var / `--basic-auth-file` flag points to an htpasswd file; bcrypt hashes only (`$2y$`)
- **REQ-002**: Valid credentials → request proceeds to policy with `input.request.auth.user` populated and `input.request.auth.kind == "basic"`
- **REQ-003**: Invalid credentials → `401 Unauthorized` with `WWW-Authenticate: Basic realm="rest-rego"`
- **REQ-004**: Missing `Authorization` header → anonymous passthrough (consistent with JWT behaviour)
- **REQ-005**: Hot-reload via `fsnotify` — credential map replaced atomically; last valid set retained on read errors
- **REQ-006**: Startup fails with `os.Exit(1)` if `BASIC_AUTH_FILE` is configured alongside `AZURE_TENANT` or `WELLKNOWN_OIDC`
- **REQ-007**: bcrypt cost < 10 rejected at load time; cost < 12 warns at load time
- **REQ-008**: MD5 (`$apr1$`) and SHA-1 (`{SHA}`) htpasswd hashes are rejected per-line with a WARN log; loading continues
- **REQ-009**: File must contain at least one valid bcrypt entry for startup to succeed
- **SEC-001**: Password field on `RequestAuth` is cleared (set to `""`) before policy evaluation — never reaches Rego
- **SEC-002**: Wrong password → always `401`, even in permissive mode
- **SEC-003**: Auth failure log line must not include the password; username omitted at WARN level in production (DEBUG only)
- **CON-001**: No new external dependencies — `golang.org/x/crypto` (indirect) and `fsnotify` (direct) are already in `go.mod`
- **CON-002**: `AuthProvider` interface in `internal/types/types.go` must remain backward-compatible; use an optional `AuthChallenger` interface for the realm string
- **CON-003**: `NewInfo()` in `internal/types/request.go` already decodes Basic auth credentials — no change needed there
- **GUD-001**: Follow existing logging conventions (`log/slog`, structured key-value pairs)
- **GUD-002**: Follow existing auto-detection pattern in `internal/application/app.go` switch
- **PAT-001**: Use `sync/atomic.Pointer[credMap]` for credential map to allow lock-free concurrent reads during hot-reload

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD**: `/.specs/PRD.md`
- **Features**: `/.specs/features/basic-auth-support.md`
- **Technology Stack**: Go
- **Cross-Product Dependencies**: None

## 2. Implementation Steps

### Implementation Phase 1 — Configuration & Mutual-Exclusion

- **GOAL-001**: Extend config and validation so the new `BASIC_AUTH_FILE` field is parsed and its mutual exclusion with other providers is enforced before any file I/O.

- **TASK-001**: Add `BasicAuthFile` field to `internal/config/config.go` `Fields` struct `[✅ Done]`
  - Files: `internal/config/config.go`
  - Action: Insert the following field after the `PermissiveAuth` line in the `Fields` struct:
    ```go
    BasicAuthFile string `arg:"--basic-auth-file,env:BASIC_AUTH_FILE" help:"path to Apache 2.4 htpasswd file (bcrypt only)" placeholder:"FILE"`
    ```

- **TASK-002**: Replace the existing two-provider mutual-exclusion check in `New()` with a three-provider check `[✅ Done]`
  - Files: `internal/config/config.go`
  - Current code (lines ~141-143):
    ```go
    if f.AzureTenant != "" && len(f.WellKnownURL) > 0 {
        slog.Error("config: only one auth-provider can be used (azure or well-known)")
        os.Exit(1)
    }
    ```
  - Replace with:
    ```go
    authCount := 0
    if f.AzureTenant != "" {
        authCount++
    }
    if len(f.WellKnownURL) > 0 {
        authCount++
    }
    if f.BasicAuthFile != "" {
        authCount++
    }
    if authCount > 1 {
        slog.Error("config: only one auth-provider may be configured (AZURE_TENANT, WELLKNOWN_OIDC, BASIC_AUTH_FILE)")
        os.Exit(1)
    }
    ```

### Implementation Phase 2 — New `internal/basicauth` Package

- **GOAL-002**: Create the `internal/basicauth` package with three files: core provider, htpasswd parser, and file watcher.

- **TASK-003**: Create `internal/basicauth/basicauth.go` — `BasicAuthProvider` struct, `New()`, `Authenticate()`, and `WWWAuthenticate()` `[✅ Done]`
  - Files: `internal/basicauth/basicauth.go` (new file)
  - Key types:
    ```go
    type credMap = map[string]string  // username → bcrypt hash

    type BasicAuthProvider struct {
        filePath   string
        creds      atomic.Pointer[credMap]
        permissive bool
        watcher    *fsnotify.Watcher
    }
    ```
  - `New(filePath string, permissive bool) *BasicAuthProvider`:
    - Calls `loadFile(filePath)` — returns nil and logs error if file fails to load or has no valid entries
    - Starts `fsnotify` watcher goroutine for hot-reload
    - Returns the provider
  - `Authenticate(info *types.Info, r *http.Request) error`:
    - If `info.Request.Auth == nil` or `!strings.EqualFold(info.Request.Auth.Kind, "basic")` → return nil (anonymous)
    - `defer func() { info.Request.Auth.Password = "" }()` — clears password unconditionally
    - Empty username → `handleFailure(b.permissive)`
    - Unknown user → `handleFailure(b.permissive)`
    - bcrypt mismatch → always `types.ErrAuthenticationFailed` (never permissive)
    - Success → return nil
  - `WWWAuthenticate() string` — returns `"Basic realm=\"rest-rego\""` (implements optional `AuthChallenger`)
  - `handleFailure(permissive bool) error`: returns nil when permissive, `types.ErrAuthenticationFailed` when strict

- **TASK-004**: Create `internal/basicauth/htpasswd.go` — file parser and bcrypt verifier `[✅ Done]`
  - Files: `internal/basicauth/htpasswd.go` (new file)
  - `loadFile(filePath string) (*credMap, error)`:
    - Reads file line by line
    - Skips empty lines and lines starting with `#`
    - Splits on first `:` — logs WARN and skips if no colon
    - Checks hash prefix:
      - `$2y$` or `$2b$` or `$2a$` → bcrypt; extract cost; reject (WARN) if cost < 10; warn if cost < 12
      - `$apr1$` → log `WARN basicauth: skipping MD5 hash`, skip
      - `{SHA}` → log `WARN basicauth: skipping SHA-1 hash`, skip
      - Other → log `WARN basicauth: skipping unsupported hash format`, skip
    - Returns `ErrNoValidCredentials` if resulting map is empty
    - On success: `slog.Info("basicauth: loaded N credentials from FILE")`
  - Import: `"golang.org/x/crypto/bcrypt"` — promote from indirect to direct in `go.mod` via `go get golang.org/x/crypto`

- **TASK-005**: Create `internal/basicauth/watcher.go` — `fsnotify`-based hot-reload `[✅ Done]`
  - Files: `internal/basicauth/watcher.go` (new file)
  - `startWatcher(b *BasicAuthProvider)` runs in a goroutine:
    - Watches for `fsnotify.Write` and `fsnotify.Create` events on `b.filePath`
    - On event: calls `loadFile(b.filePath)`
    - On success: `b.creds.Store(&newCreds)` + `slog.Info("basicauth: reloaded N credentials from FILE")`
    - On error: logs error, **does not** update `b.creds` (retains last valid set)

### Implementation Phase 3 — Wire Into Application

- **GOAL-003**: Register the new provider in the application switch and update the `AuthChallenger` interface support in the auth handler.

- **TASK-006**: Add `basicauth` case to the provider switch in `internal/application/app.go` `[✅ Done]`
  - Files: `internal/application/app.go`
  - Add import: `"github.com/AB-Lindex/rest-rego/internal/basicauth"`
  - Insert new case before the `default:` case:
    ```go
    case len(app.config.BasicAuthFile) > 0:
        slog.Debug("application: creating basic-auth-provider", "file", app.config.BasicAuthFile)
        app.auth = basicauth.New(app.config.BasicAuthFile, app.config.PermissiveAuth)
        if app.auth == nil {
            return nil, false
        }
    ```

- **TASK-007**: Add optional `AuthChallenger` interface to `internal/types/types.go` `[✅ Done]`
  - Files: `internal/types/types.go`
  - Append after the existing `Validator` interface:
    ```go
    // AuthChallenger is optionally implemented by AuthProviders that require
    // a specific WWW-Authenticate challenge header on 401 responses.
    type AuthChallenger interface {
        WWWAuthenticate() string
    }
    ```

- **TASK-008**: Update `internal/router/auth.go` to use `AuthChallenger` when available `[✅ Done]`
  - Files: `internal/router/auth.go`
  - Replace the hardcoded `w.Header().Set("WWW-Authenticate", "Bearer")` with:
    ```go
    challenge := "Bearer"
    if c, ok := proxy.auth.(types.AuthChallenger); ok {
        challenge = c.WWWAuthenticate()
    }
    w.Header().Set("WWW-Authenticate", challenge)
    ```

### Implementation Phase 4 — Promote Dependency & go.mod

- **GOAL-004**: Ensure `golang.org/x/crypto` is a direct dependency in `go.mod`.

- **TASK-009**: Promote `golang.org/x/crypto` from indirect to direct dependency `[✅ Done]`
  - Files: `go.mod`, `go.sum`
  - Command: `go get golang.org/x/crypto`
  - Verify: `golang.org/x/crypto` line in `go.mod` no longer has `// indirect` comment

### Implementation Phase 5 — Tests

- **GOAL-005**: Verify correctness of the new package with unit tests.

- **TASK-010**: Create `internal/basicauth/htpasswd_test.go` — parser unit tests `[✅ Done]`
  - Files: `internal/basicauth/htpasswd_test.go` (new file)
  - Test cases:
    - Valid bcrypt entry loads correctly
    - Comment line skipped
    - Empty line skipped
    - MD5 hash line skipped with warning (test log output)
    - SHA-1 hash line skipped with warning
    - Entry with no colon skipped
    - File with zero valid entries returns `ErrNoValidCredentials`
    - bcrypt cost < 10 rejected
    - bcrypt cost = 11 loads but logs warning

- **TASK-011**: Create `internal/basicauth/basicauth_test.go` — `Authenticate` unit tests `[✅ Done]`
  - Files: `internal/basicauth/basicauth_test.go` (new file)
  - Test cases:
    - No Authorization header → nil error (anonymous)
    - Non-Basic Authorization header → nil error (anonymous)
    - Valid username + correct password → nil error; password field cleared
    - Valid username + wrong password → `ErrAuthenticationFailed`; permissive mode still returns `ErrAuthenticationFailed`
    - Unknown username + strict mode → `ErrAuthenticationFailed`
    - Unknown username + permissive mode → nil error
    - Password field is always empty string after `Authenticate` returns (including success case)

### Implementation Phase 6 — Documentation

- **GOAL-006**: Document the new authentication mode for operators.

- **TASK-012**: Add `BASIC_AUTH_FILE` entry to `docs/CONFIGURATION.md` `[✅ Done]`
  - Files: `docs/CONFIGURATION.md`
  - Add row in the existing env-var table covering: `BASIC_AUTH_FILE`, type (string/path), default (empty), description

- **TASK-013**: Create `docs/BASIC-AUTH.md` `[✅ Done]`
  - Files: `docs/BASIC-AUTH.md` (new file)
  - Sections: overview, generating htpasswd entries (`htpasswd -B -C 12`), supported hash formats, Kubernetes Secret mounting pattern, example Rego policy using `input.request.auth.user`

- **TASK-014**: Add Kubernetes example with htpasswd Secret to `examples/kubernetes/` `[✅ Done]`
  - Files: `examples/kubernetes/basic-auth/` directory with `deployment.yaml`, `secret.yaml`, `request.rego`, `README.md`
  - Shows htpasswd file mounted as a Kubernetes Secret volume at `/etc/rest-rego/users.htpasswd`

## 3. Alternatives

- **ALT-001**: Single-file implementation instead of three-file `internal/basicauth` package — rejected; separating parser (`htpasswd.go`), watcher (`watcher.go`), and provider (`basicauth.go`) keeps each file focused and testable in isolation.
- **ALT-002**: Extend `AuthProvider` interface directly with `WWWAuthenticate() string` — rejected; would require all existing providers (Azure, JWT) to implement the method, breaking backward compatibility.
- **ALT-003**: Store the credential watcher inside `pkg/filecache` — rejected; filecache is folder-based and pattern-matched; a single-file watcher is more appropriate here.

## 4. Dependencies

- **DEP-001**: `golang.org/x/crypto/bcrypt` — already in `go.mod` as indirect; promote to direct
- **DEP-002**: `github.com/fsnotify/fsnotify` — already a direct dependency in `go.mod`

## 5. Files

- **FILE-001**: `internal/config/config.go` — add `BasicAuthFile` field; replace mutual-exclusion check
- **FILE-002**: `internal/application/app.go` — add `basicauth` provider case; add import
- **FILE-003**: `internal/types/types.go` — add optional `AuthChallenger` interface
- **FILE-004**: `internal/router/auth.go` — use `AuthChallenger` for `WWW-Authenticate` header
- **FILE-005**: `internal/basicauth/basicauth.go` (new) — provider struct, `New`, `Authenticate`, `WWWAuthenticate`
- **FILE-006**: `internal/basicauth/htpasswd.go` (new) — htpasswd parser, bcrypt verification, `loadFile`
- **FILE-007**: `internal/basicauth/watcher.go` (new) — `fsnotify` hot-reload goroutine
- **FILE-008**: `internal/basicauth/htpasswd_test.go` (new) — parser tests
- **FILE-009**: `internal/basicauth/basicauth_test.go` (new) — authenticate tests
- **FILE-010**: `go.mod` / `go.sum` — promote `golang.org/x/crypto` to direct dependency
- **FILE-011**: `docs/CONFIGURATION.md` — add `BASIC_AUTH_FILE` entry
- **FILE-012**: `docs/BASIC-AUTH.md` (new) — operator documentation
- **FILE-013**: `examples/kubernetes/basic-auth/` (new) — Kubernetes example

## 6. Testing

- **TEST-001**: `htpasswd_test.go` — valid bcrypt entry round-trip (load → verify)
- **TEST-002**: `htpasswd_test.go` — unsupported hash formats are skipped, not fatal
- **TEST-003**: `htpasswd_test.go` — empty file / all-comment file returns `ErrNoValidCredentials`
- **TEST-004**: `htpasswd_test.go` — bcrypt cost enforcement (reject < 10, warn < 12)
- **TEST-005**: `basicauth_test.go` — anonymous passthrough (no header)
- **TEST-006**: `basicauth_test.go` — correct credentials → success, password cleared
- **TEST-007**: `basicauth_test.go` — wrong password → always 401, even in permissive mode
- **TEST-008**: `basicauth_test.go` — unknown username in strict / permissive modes
- **TEST-009**: Manual — `go test ./...` passes with no race conditions (`-race` flag)
- **TEST-010**: Manual — `BASIC_AUTH_FILE` + `AZURE_TENANT` simultaneously → `os.Exit(1)` at startup

## 7. Risks & Assumptions

- **RISK-001**: bcrypt verification is CPU-intensive; a high request rate with bcrypt cost 12 credentials may add latency. Mitigation: document recommended cost (12) and note that cost 10 is the minimum; operators accept the trade-off.
- **RISK-002**: `fsnotify` behaves differently across OS/container environments (e.g., inotify limits in Kubernetes). Mitigation: same risk already accepted for policy hot-reload; no additional mitigation needed.
- **ASSUMPTION-001**: `NewInfo()` in `internal/types/request.go` already decodes Basic auth credentials (user/password from base64) — confirmed by code review; no change needed.
- **ASSUMPTION-002**: `golang.org/x/crypto` version already present in `go.sum` — confirmed; `go get` only updates the `go.mod` direct/indirect annotation.
- **ASSUMPTION-003**: Permissive mode semantics for Basic auth match those of JWT: missing/malformed header → anonymous; wrong password → always 401.

## 8. Related Specifications / Further Reading

- [Feature spec: Basic HTTP Authentication Support](/.specs/features/basic-auth-support.md)
- [golang.org/x/crypto/bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt)
- [Apache httpd htpasswd format](https://httpd.apache.org/docs/2.4/programs/htpasswd.html)
- [fsnotify documentation](https://pkg.go.dev/github.com/fsnotify/fsnotify)

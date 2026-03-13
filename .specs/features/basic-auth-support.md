---
type: "feature"
feature: "basic-auth-support"
repository_type: "single-product"
status: "proposed"
priority: "medium"
complexity: "standard"
technology_stack: ["go"]
azure_services: []
external_services: []
on_premises_dependencies: []
multi_instance_support: "compatible"
observability: "basic"
related_prd: "PRD.md"
cross_product_dependencies: []
---

# Feature: Basic HTTP Authentication Support

## Problem Statement

rest-rego currently requires either Azure Entra ID or OIDC/JWT to be configured as the authentication provider — there is no option for simpler setups. This creates friction for:

1. **Internal tooling**: Services consumed only by known automated processes where a shared secret is sufficient
2. **Development and staging environments**: Where a full OIDC stack is unavailable or overkill
3. **Migration scenarios**: Gradually introducing rest-rego into systems that currently use `Authorization: Basic` credentials
4. **Air-gapped networks**: No external OIDC provider reachable, JWT infrastructure not yet in place

The product must remain **secure by default**: no auth-provider configured means hard failure at startup, and a misconfigured Basic auth setup must also fail closed.

## Goals

- Add Basic HTTP authentication as a first-class auth provider alongside Azure and JWT
- Use the Apache 2.4 htpasswd bcrypt file format as the credential store (widely understood, tooling available)
- Fail at startup if both Basic auth and JWT/Azure are configured simultaneously
- Never expose passwords to the Rego policy engine
- Continue the implicit auto-detection pattern already established in the codebase

## Non-Goals

- Support for MD5 (`$apr1$`) or SHA-1 (`{SHA}`) htpasswd hash formats — bcrypt only
- Dynamic credential updates without file reload (file-watch hot-reload is sufficient)
- Built-in user management commands (operators use standard `htpasswd` tooling)
- Digest authentication (RFC 7616)

---

## User Stories

### US-001: Configure Basic Auth via htpasswd File

**As an** operator  
**I want** to point rest-rego at an Apache-style htpasswd file  
**So that** I can use standard `htpasswd` tooling to manage credentials and protect services without an OIDC provider

**Acceptance Criteria**:
- Set `BASIC_AUTH_FILE=/etc/rest-rego/users.htpasswd` (or `--basic-auth-file`)
- File uses Apache 2.4 bcrypt format: `username:$2y$cost$salt+hash`
- rest-rego loads the file at startup and logs the number of entries loaded
- Valid credentials → request proceeds to the Rego policy with `input.request.auth.user` populated
- Invalid credentials → `401 Unauthorized` with `WWW-Authenticate: Basic realm="rest-rego"`
- Missing `Authorization` header → treated as anonymous (same as JWT providers)

**Example htpasswd file** (`users.htpasswd`):
```
# rest-rego service accounts
deployer:$2y$12$K5b4Gjp4uzrSaxKGh3Tz0.nFQoO/TEnbw/kSdX5Sg4Ae0ZRY.gEe2
monitor:$2y$12$7nLJQf3V5jcHkZqMXY0KoODAkBP8eJ4nf7y1TnXwRJzHL9WvYQ7km
```

Generate entries with standard tooling:
```bash
htpasswd -B -C 12 users.htpasswd deployer
```

**Example Rego policy** using the authenticated username:
```rego
package request.rego

default allow := false

allow if {
    input.request.auth.kind == "basic"
    input.request.auth.user == "deployer"
    input.request.path[0] == "deploy"
}
```

### US-002: Startup Fails When Basic Auth and JWT/Azure Are Both Configured

**As a** platform engineer  
**I want** rest-rego to refuse to start when `BASIC_AUTH_FILE` is set alongside `WELLKNOWN_OIDC` or `AZURE_TENANT`  
**So that** misconfiguration is caught immediately rather than silently favouring one provider

**Acceptance Criteria**:
- If `BASIC_AUTH_FILE` is set together with `WELLKNOWN_OIDC`, startup fails with:
  ```
  ERROR config: BASIC_AUTH_FILE and WELLKNOWN_OIDC are mutually exclusive
  ```
- If `BASIC_AUTH_FILE` is set together with `AZURE_TENANT`, startup fails with:
  ```
  ERROR config: BASIC_AUTH_FILE and AZURE_TENANT are mutually exclusive
  ```
- Exit code is non-zero (existing `os.Exit(1)` pattern)
- No partial startup — check occurs before any file loading

### US-003: Passwords Are Never Exposed to the Policy Engine

**As a** security engineer  
**I want** to be certain that plaintext passwords are cleared before the request reaches the Rego policy  
**So that** a policy bug or debug logging cannot inadvertently leak credentials

**Acceptance Criteria**:
- `input.request.auth.password` is always empty string / absent when the Basic auth provider is active
- The password is verified in the auth provider, then the field is cleared on the `RequestAuth` struct before policy evaluation
- `--debug` mode (which logs the full policy input) never prints the password
- Structured log lines from the Basic auth provider never include the password value

### US-004: Permissive Mode for Anonymous Requests

**As an** operator  
**I want** requests without an `Authorization` header to pass through as anonymous when `--permissive-auth` is set  
**So that** I can write policies that selectively require authentication only for certain paths

**Acceptance Criteria**:
- No `Authorization` header → `info.Request.Auth.Kind == ""`, `input.request.auth.user == ""`; request proceeds to policy
- `Authorization: Basic <invalid-base64>` → `401 Unauthorized` in strict mode, anonymous passthrough in permissive mode
- Correct header but unknown username → `401 Unauthorized` in strict mode, anonymous passthrough in permissive mode
- Correct header, known username, wrong password → **always** `401 Unauthorized` (never anonymous, regardless of permissive mode)

### US-005: Hot-Reload on htpasswd File Changes

**As an** operator  
**I want** rest-rego to pick up changes to the htpasswd file without restarting  
**So that** I can add or revoke credentials with zero downtime

**Acceptance Criteria**:
- File changes are detected via `fsnotify` (consistent with policy hot-reload)
- Updated credentials are loaded atomically — in-flight requests use the previous credential set
- Reload is logged at INFO level including the number of entries loaded
- A file that becomes unreadable or malformed does not crash the process; last valid credential set is retained and an error is logged

---

## Technical Design

### New Configuration Field

Add to `internal/config/config.go` (`Fields` struct):

```go
BasicAuthFile string `arg:"--basic-auth-file,env:BASIC_AUTH_FILE" help:"path to Apache 2.4 htpasswd file (bcrypt only)" placeholder:"FILE"`
```

### Mutual Exclusion Validation

Extend the existing `Validate()` method in `config.go`:

```go
// Check mutual exclusion of auth providers
authCount := 0
if len(f.AzureTenant) > 0    { authCount++ }
if len(f.WellKnownURL) > 0   { authCount++ }
if len(f.BasicAuthFile) > 0  { authCount++ }
if authCount > 1 {
    slog.Error("config: only one auth provider may be configured (AZURE_TENANT, WELLKNOWN_OIDC, BASIC_AUTH_FILE)")
    os.Exit(1)
}
```

### Provider Selection in `app.go`

The new case slots naturally into the existing switch before the `default` hard-fail:

```go
case len(app.config.BasicAuthFile) > 0:
    slog.Debug("application: creating basic-auth-provider", "file", app.config.BasicAuthFile)
    app.auth = basicauth.New(app.config.BasicAuthFile, app.config.PermissiveAuth)
    if app.auth == nil {
        return nil, false
    }
```

### New Package: `internal/basicauth`

| File | Responsibility |
|------|---------------|
| `basicauth.go` | `BasicAuthProvider` struct, `New()`, `Authenticate()` |
| `htpasswd.go` | htpasswd file parser and bcrypt verification |
| `watcher.go` | `fsnotify`-based hot-reload |

#### `BasicAuthProvider` struct

```go
type BasicAuthProvider struct {
    filePath   string
    creds      atomic.Pointer[credMap]  // safe concurrent reads during hot-reload
    permissive bool
}

type credMap map[string]string  // username → bcrypt hash
```

#### `Authenticate` logic

```go
func (b *BasicAuthProvider) Authenticate(info *types.Info, r *http.Request) error {
    auth := info.Request.Auth

    // No Authorization header → anonymous
    if auth == nil || !strings.EqualFold(auth.Kind, "basic") {
        return nil  // anonymous passthrough
    }

    // Always clear password before returning — never reaches Rego
    defer func() { auth.Password = "" }()

    // Missing username in header
    if auth.User == "" {
        return handleFailure(b.permissive, types.ErrAuthenticationFailed)
    }

    creds := b.creds.Load()
    hash, known := (*creds)[auth.User]

    // Unknown user
    if !known {
        return handleFailure(b.permissive, types.ErrAuthenticationFailed)
    }

    // Wrong password — never permissive
    if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(auth.Password)); err != nil {
        return types.ErrAuthenticationFailed
    }

    return nil  // authenticated
}
```

### htpasswd File Format

Only lines matching `username:$2y$...` are loaded. Supported line types:

| Line | Action |
|------|--------|
| Empty line | Skipped |
| Line starting with `#` | Skipped (comment) |
| `user:$2y$N$...` | Loaded (bcrypt) |
| `user:$apr1$...` | Rejected — MD5, not supported |
| `user:{SHA}...` | Rejected — SHA-1, not supported |
| No colon | Rejected — malformed |

Unsupported hash formats log a warning per-line at startup but do not abort loading. At least one valid bcrypt entry must be present for startup to succeed.

### Policy Input

The `input.request.auth` object available in Rego is populated by the existing `NewInfo()` in `internal/types/request.go` (no change required):

| Field | Value when authenticated | Value when anonymous |
|-------|--------------------------|----------------------|
| `input.request.auth.kind` | `"basic"` | `""` / absent |
| `input.request.auth.user` | username string | `""` / absent |
| `input.request.auth.password` | **always `""`** (cleared) | `""` / absent |
| `input.request.auth.token` | raw base64 value | `""` / absent |

### 401 Response Headers

The `authHandler` in `internal/router/auth.go` already sets `WWW-Authenticate: Bearer` on failure. The header value must be provider-aware. The `AuthProvider` interface should be extended or the router should conditionally set the realm:

```
WWW-Authenticate: Basic realm="rest-rego"
```

Two options for implementation:
- Extend `types.AuthProvider` with a `WWWAuthHeader() string` method
- Set the header in the `BasicAuthProvider` via the existing `w http.ResponseWriter` (requires interface change)

Recommended: extend `AuthProvider` to optionally return a challenge string, keeping backward compatibility via an optional interface:
```go
type AuthChallenger interface {
    WWWAuthenticate() string
}
```

---

## Security Properties

| Property | Behaviour |
|----------|-----------|
| No auth configured | Hard fail at startup — unchanged |
| Two providers configured | Hard fail at startup — new |
| Missing `Authorization` header | Anonymous (Rego decides) |
| Invalid credentials (strict mode) | `401 Unauthorized` |
| Invalid credentials (permissive mode) | Anonymous (except wrong password — always 401) |
| Password in policy input | Never — cleared before eval |
| Password in debug logs | Never — cleared before eval |
| bcrypt cost | Minimum 10 enforced at load time; warn if below 12 |
| MD5/SHA-1 htpasswd hashes | Rejected at load time |

---

## Dependencies

- [`golang.org/x/crypto/bcrypt`](https://pkg.go.dev/golang.org/x/crypto/bcrypt) — already a transitive dependency via `lestrrat-go/jwx`; check if direct import is needed
- `github.com/fsnotify/fsnotify` — already used in `pkg/filecache`

No new external dependencies expected.

---

## Observability

- Startup: `INFO basicauth: loaded N credentials from FILE`
- Hot-reload: `INFO basicauth: reloaded N credentials from FILE`
- Malformed line: `WARN basicauth: skipping unsupported hash format user=X file=FILE`
- Auth failure: `WARN basicauth: authentication failed user=X` (no path logged to avoid PII leakage in shared log streams — operator may enable via `--debug`)
- Wrong password: no username logged at WARN level in production mode; included only at DEBUG level

---

## Documentation Updates

- `docs/CONFIGURATION.md` — add `BASIC_AUTH_FILE` entry
- `docs/` — new `BASIC-AUTH.md` explaining htpasswd format, `htpasswd` command examples, Kubernetes Secret mounting pattern
- `examples/kubernetes/` — add example with htpasswd file mounted from a Kubernetes Secret

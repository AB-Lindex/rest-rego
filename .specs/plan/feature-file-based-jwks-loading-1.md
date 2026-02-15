---
goal: Implement File-Based JWKS and Well-Known Configuration Loading
version: 1.0
date_created: 2026-02-11
last_updated: 2026-02-15
owner: AB-Lindex Team
status: 'In Progress'
tags: [feature, authentication, jwt, testing, air-gapped, offline]
---

# Introduction

![Status: In Progress](https://img.shields.io/badge/status-In%20Progress-yellow)

This plan adds support for loading JWKS and OIDC well-known configurations from local `file:` URLs in addition to the existing HTTP(S) endpoints. This enables testing without external OIDC providers, air-gapped deployments, and faster CI/CD pipelines.

The implementation is split into 7 small, independently testable phases that build on each other. Each phase produces a working commit with its own tests.

**Phase 1 completed**: File URL utility functions with comprehensive unit tests.

**Update each task's status tag as work progresses.**

## 1. Requirements & Constraints

- **REQ-001**: Parse `file:`, `file:/path`, `file:///path`, and `file://localhost/path` URL formats
- **REQ-002**: Load well-known JSON from local files when a `file:` URL is configured
- **REQ-003**: Load JWKS from local files when `jwks_uri` in well-known uses a `file:` URL
- **REQ-004**: Authenticate JWTs using statically-loaded file-based JWKS (no cache refresh)
- **REQ-005**: Support mixed file and HTTP sources across different issuers in `WELLKNOWN_OIDC`
- **REQ-006**: Reject source-type mismatch within a single issuer (file well-known → HTTP jwks_uri or vice versa)
- **REQ-007**: Provide clear structured error messages for all file-loading failures
- **REQ-008**: Support relative file paths resolved from the working directory
- **SEC-001**: Validate and clean file paths to prevent path traversal; explicitly reject paths containing `..` components after normalization
- **SEC-002**: Respect Unix file permissions; log permission errors clearly
- **CON-001**: No breaking changes — all existing HTTP-based configurations must work identically
- **CON-002**: File-based JWKS are loaded once at startup; no auto-refresh or file watching
- **CON-003**: Phase 1 utility functions go into a new `internal/jwtsupport/support.go`; Phases 2-5 modify `internal/jwtsupport/jwt.go`; documentation files are updated separately
- **GUD-001**: Use `log/slog` structured logging consistently, matching existing patterns
- **GUD-002**: Keep utility functions small, pure, and independently testable
- **PAT-001**: Follow existing error-and-continue pattern in `LoadWellKnowns()` and `LoadJWKS()` loops

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD**: `/.specs/PRD.md`
- **Feature Spec**: `/.specs/features/file-based-jwks-loading.md`
- **Technology Stack**: Go
- **Cross-Product Dependencies**: None

## 2. Implementation Steps

### Phase 1 — File URL utility functions

- **GOAL-001**: Add pure utility functions for detecting and reading `file:` URLs in a new `support.go` file, fully covered by unit tests.

- **TASK-001**: Create `isFileURL(url string) bool` in `internal/jwtsupport/support.go` `[✅ Completed]`
  - Files: `internal/jwtsupport/support.go` (new file)
  - Returns `true` when the string starts with `file:` (case-sensitive)
  - Returns `false` for empty strings, `https://`, `http://`, and other schemes

- **TASK-002**: Create `fileURLToPath(fileURL string) (string, error)` in `internal/jwtsupport/support.go` `[✅ Completed]`
  - Files: `internal/jwtsupport/support.go`
  - Uses `net/url.Parse()` to parse the URL and extract the path
  - Returns error for malformed URLs
  - Uses `filepath.Clean()` on the resulting path to normalize it
  - **Path Traversal Prevention**: After cleaning, checks if the path contains `..` separator or starts with `..` — if so, returns error: `"path traversal not allowed"`
  - Supports both absolute paths (e.g., `/config/jwks.json`) and relative paths (e.g., `config/jwks.json`, `./config/jwks.json`)
  - If path is relative, it will be resolved relative to the working directory when passed to `os.ReadFile()`
  - Returns the cleaned path (absolute or relative)

- **TASK-003**: Create `readFileURL(fileURL string) ([]byte, error)` in `internal/jwtsupport/support.go` `[✅ Completed]`
  - Files: `internal/jwtsupport/support.go`
  - Calls `fileURLToPath()` then `os.ReadFile()`
  - Wraps errors with `fmt.Errorf` context including the original URL
  - Logs at `slog.Debug` level with the resolved path
  - Dependencies: TASK-002

- **TASK-004**: Create `sourceType(url string) string` helper in `internal/jwtsupport/support.go` `[✅ Completed]`
  - Files: `internal/jwtsupport/support.go`
  - Returns `"file"` if `isFileURL()` is true, otherwise `"http"`
  - Used for logging and error messages

- **TASK-005**: Write unit tests for TASK-001 through TASK-004 in `internal/jwtsupport/support_test.go` `[✅ Completed]`
  - Files: `internal/jwtsupport/support_test.go` (new file)
  - `TestIsFileURL`: table-driven test with cases: `file:/path` → true, `file:///path` → true, `file://localhost/path` → true, `https://example.com` → false, `http://example.com` → false, `""` → false, `ftp://` → false
  - `TestFileURLToPath`: table-driven test with cases:
    - Valid paths: `file:///tmp/a.json` → `/tmp/a.json`, `file:/tmp/a.json` → `/tmp/a.json`, `file://localhost/tmp/a.json` → `/tmp/a.json`
    - Relative paths: `file:config/test.json` → `config/test.json`, `file:./config/test.json` → `config/test.json`
    - Path traversal (should error): `file:../etc/passwd` → error, `file:config/../../etc/passwd` → error, `file:///config/../../../etc/passwd` → error
    - Malformed → error
  - `TestReadFileURL`: create temp file with known content, read via `file:///` URL, assert content matches; also test non-existent file → error; test relative path resolution
  - `TestSourceType`: `file:///x` → `"file"`, `https://x` → `"http"`
  - Estimated effort: 1-2 hours

### Phase 2 — File-based well-known loading

- **GOAL-002**: Extend `LoadWellKnowns()` to load from `file:` URLs while preserving the existing HTTP path unchanged.

- **TASK-006**: Add `encoding/json` import and file-branch to `LoadWellKnowns()` `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt.go`
  - At the top of the `for` loop body, add: `if isFileURL(wellKnown) { ... }`
  - Inside the file branch: call `readFileURL(wellKnown)`, then `json.Unmarshal(data, &wc)`
  - On success: `slog.Info("jwtsupport: loaded well-known from file", "url", wellKnown)`
  - On any error: `slog.Error(...)` then `continue` (matches existing pattern)
  - The existing HTTP path moves into an `else` block, entirely unchanged
  - Dependencies: TASK-001, TASK-003

- **TASK-007**: Write unit tests for file-based `LoadWellKnowns()` `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt_test.go` (new file)
  - Create a temp `well-known.json` file with valid JSON containing `jwks_uri` and `id_token_signing_alg_values_supported`
  - Construct a `JWTSupport` with `wellKnowns: []string{"file:///tmp/.../well-known.json"}`
  - Call `LoadWellKnowns()` and assert `wellknownList` has 1 entry with correct `JwksURI`
  - Test invalid JSON file → `wellknownList` remains empty
  - Test non-existent file → `wellknownList` remains empty
  - Dependencies: TASK-006

### Phase 3 — File-based JWKS loading

- **GOAL-003**: Extend `LoadJWKS()` to load JWKS from `file:` URLs, apply `PostFetch()`, and store as static `jwk.Set`.

- **TASK-008**: Restructure `LoadJWKS()` to handle file-based JWKS `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt.go`
  - Move cache creation (`jwk.NewCache(...)`) to only run when at least one non-file URL exists (or always create it — simpler; the cache being unused is harmless)
  - Inside the loop over `wellknownList`, add `if isFileURL(wk.JwksURI) { ... }` branch
  - File branch: `readFileURL(wk.JwksURI)` → `jwk.Parse(data)` → `wk.PostFetch(wk.JwksURI, set)` → append to `j.JWKS`
  - Log: `slog.Info("jwtsupport: loaded jwks from file", "url", wk.JwksURI, "keys", set.Len())`
  - Existing HTTP branch goes into the `else` block, unchanged
  - Dependencies: TASK-006

- **TASK-009**: Write unit tests for file-based `LoadJWKS()` `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt_test.go`
  - Note: tests for `LoadJWKS()` and `LoadWellKnowns()` go in `jwt_test.go`, separate from the support utility tests in `support_test.go`
  - Create temp JWKS file (use the structure from `temp/duende/jwks.json` as reference)
  - Set up `JWTSupport` with a `wellknownList` entry whose `JwksURI` is a `file:///` URL
  - Call `LoadJWKS()` and assert `j.JWKS` has 1 entry with correct key count
  - Test invalid JWKS JSON → `j.JWKS` stays empty
  - Test `PostFetch` algorithm enrichment: provide JWKS without `alg` field, set `SupportedAlgorithms` on the `wellKnownData`, verify `PostFetch` adds the algorithm
  - Dependencies: TASK-008
  - Estimated effort: 1-2 hours

### Phase 4 — Source-type mismatch validation

- **GOAL-004**: Reject configurations where a single issuer has mismatched source types (file well-known ↔ HTTP jwks_uri).

- **TASK-010**: Track the original well-known URL per `wellknownList` entry `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt.go`
  - Add a `sourceURL string` field to `wellKnownData` struct to record the original well-known URL used to load it
  - Add an `isLocalFile bool` field to explicitly track whether the source is file-based or HTTP-based
  - Set `wc.sourceURL = wellKnown` and `wc.isLocalFile = true/false` in `LoadWellKnowns()` after successful load (both file and HTTP paths)
  - These fields are unexported and used only internally for logging/validation/authentication

- **TASK-011**: Add source-type validation at the start of the `LoadJWKS()` loop `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt.go`
  - Before loading the JWKS for each `wellknownList` entry, compare `isFileURL(wk.sourceURL)` with `isFileURL(wk.JwksURI)`
  - If they differ: `slog.Error("jwtsupport: source type mismatch", "well-known", wk.sourceURL, "well-known-type", sourceType(wk.sourceURL), "jwks-uri", wk.JwksURI, "jwks-type", sourceType(wk.JwksURI), "error", "well-known and jwks_uri must use matching source types (both file or both http)")` then `continue`
  - Dependencies: TASK-010

- **TASK-012**: Write unit tests for source-type mismatch `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt_test.go`
  - Create a well-known file with `jwks_uri: "https://..."` → call `LoadJWKS()` → assert `j.JWKS` is empty
  - Create an HTTP-sourced `wellknownList` entry with `jwks_uri: "file:///..."` → call `LoadJWKS()` → assert `j.JWKS` is empty
  - Create matching source types (both file) → assert `j.JWKS` has 1 entry
  - Dependencies: TASK-011
  - Estimated effort: 1 hour

### Phase 5 — Authenticate with file-based JWKS

- **GOAL-005**: Make `Authenticate()` work with statically-loaded file-based JWKS instead of requiring cache lookups.

- **TASK-013**: Refactor `Authenticate()` to support both static and cached JWKS `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt.go`, `internal/jwtsupport/jwt_test.go`
  - Add `isLocalFile bool` field to `wellKnownData` struct to explicitly track source type
  - Update `LoadWellKnowns()` to set `wc.isLocalFile = true` for file-based sources and `wc.isLocalFile = false` for HTTP sources
  - Change the `Authenticate()` loop to iterate with index: `for i, wc := range j.wellknownList`
  - Add explicit if-statement checking `wc.isLocalFile`:
    - If true (file-based): use `j.JWKS[i]` directly (the static set loaded at startup, no refresh)
    - If false (HTTP-based): call `j.cache.Get(ctx, wc.JwksURI)` to fetch fresh keys with automatic refresh
  - This design keeps file and HTTP behaviors explicit and separate, ensuring HTTP sources always get fresh token validation
  - Update unit tests to verify `isLocalFile` flag is set correctly for both file and HTTP sources
  - Dependencies: TASK-008

- **TASK-014**: Write integration-style unit test for `Authenticate()` with file-based keys `[✅ Completed]`
  - Files: `internal/jwtsupport/jwt_test.go`
  - Note: this test file is shared with Phase 2 and Phase 3 tests
  - Generate an RSA key pair in-test using `crypto/rsa` and `crypto/rand`
  - Create a JWKS file containing the public key
  - Create a well-known file pointing to the JWKS file
  - Sign a JWT token with the private key using `github.com/lestrrat-go/jwx/v2/jwt` and `jwa.RS256`
  - Construct a `JWTSupport` via constructing manually (not via `New()` which calls `os.Exit`)
  - Call `LoadWellKnowns()` and `LoadJWKS()`
  - Create a mock `types.Info` with the token as `Auth.Token`, `Auth.Kind = "bearer"`
  - Call `Authenticate()` and assert it returns nil (success)
  - Also assert `info.JWT` map contains expected claims (e.g., `aud`)
  - Test with invalid token → assert error
  - Dependencies: TASK-013

### Phase 6 — Documentation updates

- **GOAL-006**: Create comprehensive documentation for file-based JWKS loading as a standalone guide.

- **TASK-015**: Create new `docs/FILE-BASED-JWKS.md` documentation `[✅ Completed]`
  - Files: `docs/FILE-BASED-JWKS.md` (new file)
  - Structure following the markdown instructions template
  - Sections:
    1. **Overview**: Purpose (testing, air-gapped, CI/CD), benefits over HTTP-based JWKS
    2. **Use Cases**: Local development, air-gapped deployments, CI/CD pipelines, integration testing
    3. **File URL Formats**: `file:///absolute/path`, `file:/absolute/path`, `file:relative/path` syntax with examples
    4. **Configuration**: Basic setup, well-known structure, JWKS structure, environment variable examples
    5. **Source Type Consistency**: Explain the requirement that well-known and jwks_uri must match types (both file or both HTTP)
    6. **Security Considerations**: File permissions, path traversal prevention, no auto-refresh behavior
    7. **Limitations**: Static loading (no auto-refresh), no file watching, requires restart for changes
    8. **Examples**: Local filesystem, Docker volume mount, Kubernetes ConfigMap
    9. **Troubleshooting**: Common errors (file not found, permission denied, malformed JSON, source mismatch)
    10. **Testing Workflow**: Generate keys, create JWKS, sign tokens, validate with rest-rego
  - Include complete working examples (well-known.json and jwks.json) that can be copy-pasted
  - Add mermaid diagram showing file-based vs HTTP-based flow comparison
  - Estimated effort: 2-3 hours

- **TASK-016**: Add cross-reference in `docs/JWT.md` `[✅ Completed]`
  - Files: `docs/JWT.md`
  - Add minimal note in "Standard OIDC Configuration" section (after line showing `WELLKNOWN_OIDC=https://...`)
  - Content: Brief paragraph mentioning file-based alternative for testing/offline scenarios with link to `FILE-BASED-JWKS.md`
  - Keep JWT.md focused on production HTTP-based usage
  - Estimated effort: 15 minutes

- **TASK-017**: Update `docs/CONFIGURATION.md` JWT authentication section `[✅ Completed]`
  - Files: `docs/CONFIGURATION.md`
  - Locate the `WELLKNOWN_OIDC` parameter description in "JWT Authentication" section
  - Update description to: "OIDC well-known configuration URL. Supports `https://`, `http://`, and `file://` URLs."
  - Add example showing file URL: `WELLKNOWN_OIDC=file:///config/well-known.json`
  - Add note: "File-based sources must have matching source types (see FILE-BASED-JWKS.md)"
  - Estimated effort: 30 minutes

- **TASK-018**: Update `README.md` features and examples `[✅ Completed]`
  - Files: `README.md`
  - Add row to Key Features table: "**File-Based JWKS**" / "Load JWKS from local files for testing and air-gapped deployments"
  - Add new subsection under Quick Start or Examples: "Local Development / Testing" with file-based configuration snippet
  - Link to `docs/FILE-BASED-JWKS.md` for details
  - Estimated effort: 30 minutes

- **TASK-019**: Create Kubernetes ConfigMap example `[✅ Completed]`
  - Files: `examples/kubernetes/file-based-jwks/` (new directory)
  - `configmap-jwks.yaml`: ConfigMap with embedded well-known.json and jwks.json (use multiline YAML strings)
  - `deployment.yaml`: Deployment showing volume mount pattern, environment variable configuration
  - `request.rego`: Simple allow-all policy for testing
  - `README.md`: Step-by-step instructions (apply ConfigMap, deploy, test with sample JWT)
  - Include commands to generate test RSA key pair and create JWKS file
  - Estimated effort: 1-2 hours

### Phase 7 — End-to-end validation

- **GOAL-007**: Create test fixtures and manual test scenarios to validate the full flow.

- **TASK-020**: Add test fixture files under `temp/file-jwks/` `[📋 Planned]`
  - Files: `temp/file-jwks/well-known.json`, `temp/file-jwks/jwks.json`
  - Well-known with `jwks_uri: "file:///...path.../jwks.json"` and `id_token_signing_alg_values_supported: ["RS256"]`
  - JWKS from existing `temp/duende/jwks.json` (or generated test key)
  - These serve as quick manual-test assets and examples for documentation
  - Coordinate with TASK-015 to ensure consistency between docs and fixtures

- **TASK-021**: Add manual test `.http` file for file-based JWKS `[📋 Planned]`
  - Files: `tests/file-jwks.http`
  - Document how to start rest-rego with `WELLKNOWN_OIDC=file:///...` and test a request
  - Include expected log output showing "loaded well-known from file" and "loaded jwks from file"
  - Reference the `docs/FILE-BASED-JWKS.md` guide for detailed testing instructions
  - Dependencies: All previous phases
  - Estimated effort: 1 hour

## 3. Alternatives

- **ALT-001**: Use file watching (fsnotify) to auto-reload JWKS files on change. Rejected for MVP — adds complexity; ConfigMap updates in Kubernetes trigger pod restarts anyway. Can be added later.
- **ALT-002**: Add a new separate config flag (e.g., `JWKS_FILE`) instead of reusing `WELLKNOWN_OIDC` with `file:` URLs. Rejected — the `file:` URL approach is cleaner, requires no new config fields, and is consistent with how many tools handle file vs HTTP.
- **ALT-003**: Support `file:` URLs only for `jwks_uri` (not well-known). Rejected — users need to provide well-known files too since they contain the JWKS URI and supported algorithms.
- **ALT-004**: Allow cross-source within a single issuer (file well-known → HTTP jwks_uri). Rejected — creates confusing partial-failure scenarios in air-gapped environments and mixed trust levels.

## 4. Dependencies

- **DEP-001**: `github.com/lestrrat-go/jwx/v2/jwk` — already in go.mod; `jwk.Parse([]byte)` used for file-based JWKS parsing
- **DEP-002**: `encoding/json` — Go stdlib; needed for `json.Unmarshal` of well-known files
- **DEP-003**: `net/url` — Go stdlib; needed for parsing `file:` URLs
- **DEP-004**: `path/filepath` — Go stdlib; needed for `filepath.Clean()` path sanitization
- **DEP-005**: `os` — Go stdlib; already imported; `os.ReadFile()` for reading files
- **DEP-006**: No new external dependencies required

## 5. Files

- **FILE-001**: `internal/jwtsupport/support.go` — New file: pure utility functions (`isFileURL`, `fileURLToPath`, `readFileURL`, `sourceType`)
- **FILE-002**: `internal/jwtsupport/support_test.go` — New file: unit tests for utility functions
- **FILE-003**: `internal/jwtsupport/jwt.go` — Modified: `LoadWellKnowns()`, `LoadJWKS()`, and `Authenticate()` with file-based branches
- **FILE-004**: `internal/jwtsupport/jwt_test.go` — New file: unit tests for modified loading and authentication functions
- **FILE-005**: `docs/FILE-BASED-JWKS.md` — New: Comprehensive standalone guide for file-based JWKS loading
- **FILE-006**: `docs/JWT.md` — Minor: Add cross-reference to FILE-BASED-JWKS.md in "Standard OIDC Configuration" section
- **FILE-007**: `docs/CONFIGURATION.md` — Update `WELLKNOWN_OIDC` documentation with `file:` URL support
- **FILE-008**: `README.md` — Update features table and add local development example
- **FILE-009**: `examples/kubernetes/file-based-jwks/configmap-jwks.yaml` — New: Kubernetes ConfigMap example
- **FILE-010**: `examples/kubernetes/file-based-jwks/deployment.yaml` — New: Kubernetes Deployment example
- **FILE-011**: `examples/kubernetes/file-based-jwks/request.rego` — New: Simple test policy
- **FILE-012**: `examples/kubernetes/file-based-jwks/README.md` — New: Step-by-step guide with key generation instructions
- **FILE-013**: `temp/file-jwks/well-known.json` — New: test fixture well-known
- **FILE-014**: `temp/file-jwks/jwks.json` — New: test fixture JWKS
- **FILE-015**: `tests/file-jwks.http` — New: manual test file

## 6. Testing

- **TEST-001**: `TestIsFileURL` — Table-driven: various URL schemes → correct boolean (Phase 1)
- **TEST-002**: `TestFileURLToPath` — Table-driven: file URL formats → correct path or error; includes path traversal attack tests with `..` notation (Phase 1)
- **TEST-003**: `TestReadFileURL` — Temp file round-trip read; non-existent file error; relative path resolution (Phase 1)
- **TEST-004**: `TestSourceType` — Verify string output for file vs HTTP URLs (Phase 1)
- **TEST-005**: `TestLoadWellKnowns_File` — Valid/invalid/missing well-known files (Phase 2)
- **TEST-006**: `TestLoadJWKS_File` — Valid/invalid JWKS files, PostFetch algorithm enrichment (Phase 3)
- **TEST-007**: `TestLoadJWKS_SourceTypeMismatch` — File↔HTTP mismatch rejected; matching types accepted (Phase 4)
- **TEST-008**: `TestAuthenticate_FileBasedKeys` — Full JWT sign → validate round-trip with file-based keys (Phase 5)
- **TEST-009**: `TestAuthenticate_FileBasedKeys_InvalidToken` — Bad token rejected with file-based keys (Phase 5)
- **TEST-010**: Existing tests pass unchanged — `go test ./...` green after each phase (all phases)

## 7. Risks & Assumptions

- **RISK-001**: The `Authenticate()` refactor (TASK-013) touches the hot path for all requests. Mitigation: the change adds an explicit `isLocalFile` flag check that branches to either use static `j.JWKS[i]` for files or call `j.cache.Get()` for HTTP sources, preserving existing behavior for HTTP-based authentication. Verified with existing tests — all pass unchanged.
- **RISK-002**: `jwk.Parse([]byte)` may behave slightly differently than keys fetched via `jwk.Cache`. Mitigation: use the same `PostFetch()` processing and validate in TEST-006 and TEST-008.
- **RISK-003**: Index correspondence between `wellknownList[i]` and `JWKS[i]` could drift if an entry is skipped (e.g., source mismatch). Mitigation: when an entry is skipped in `LoadJWKS()`, also remove it from `wellknownList` or use a map-based lookup. Review during TASK-008 and TASK-011 implementation.
- **RISK-004**: Path traversal attacks using `..` notation could expose sensitive system files. Mitigation: explicitly validate and reject paths containing `..` after normalization with `filepath.Clean()`, even though config is deployment-controlled.
- **ASSUMPTION-001**: File-based JWKS files are small (<100KB) and can be read fully into memory at startup without concern.
- **ASSUMPTION-002**: The `file:` URL scheme is universally understood by operators configuring the system.
- **ASSUMPTION-003**: Windows support is not required immediately; the `file:` URL path handling focuses on Unix-style paths.
- **ASSUMPTION-004**: Relative paths are resolved from the working directory where rest-rego is started, which is typically the application root in containerized deployments.

## 8. Related Specifications / Further Reading

- [Feature specification](/.specs/features/file-based-jwks-loading.md)
- [PRD](/.specs/PRD.md) — Sections 6.2 (Authentication), 8.1 (Deployment), 9.7 (Testing)
- [RFC 8089 — The "file" URI Scheme](https://datatracker.ietf.org/doc/html/rfc8089)
- [lestrrat-go/jwx v2 — jwk.Parse documentation](https://pkg.go.dev/github.com/lestrrat-go/jwx/v2/jwk#Parse)
- [Go net/url.Parse documentation](https://pkg.go.dev/net/url#Parse)

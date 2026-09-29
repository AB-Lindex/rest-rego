---
goal: Restore single-key JWKS bypass for HTTP-cached issuers to fix v1.6.0 401 authentication regression
version: "1.0"
date_created: 2026-09-29
owner: rest-rego team
status: 'Planned'
tags: [bug, regression, jwt, auth, security]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

`v1.6.0` introduced a regression in `internal/jwtsupport/jwt.go` that causes `401 Unauthorized` for every request validated against an HTTP(S)-fetched JWKS (`jwks_uri` served over `http://`/`https://`) that contains exactly one signing key. This is the QA/Internal-BFF configuration described in [.specs/issues/2026-09-29 Andreas-1.md](../issues/2026-09-29%20Andreas-1.md).

**Root cause**: the `07d888d` refactor ("pre-computed JWT parse options", see [optimize-jwt-options-precompute-1.md](optimize-jwt-options-precompute-1.md)) preserved the pre-`v1.6.0` single-key shortcut (`jwt.WithKey(alg, key)`, which validates against the sole key without requiring a matching `kid`) only for `wc.isLocalFile == true` (static file-based JWKS). For HTTP-cached issuers it unconditionally uses `jwt.WithKeySet(ks)`. In `lestrrat-go/jwx/v2`, `jwt.WithKeySet` defaults `requireKid = true` and `useDefault = false` ([jws/options.go](https://pkg.go.dev/github.com/lestrrat-go/jwx/v2/jws) `WithKeySet`), so verification now fails whenever the token's `kid` (or lack of one) does not exactly match a key in the set — even when the set has only one key. Before `v1.6.0`, this case was verified directly against the sole key, bypassing `kid` matching entirely.

This plan reproduces the failure with an automated test first, then applies the minimal fix, then confirms the fix removes the failure without breaking existing behavior.

## 1. Requirements & Constraints

- **REQ-001**: HTTP-cached JWKS with exactly one key MUST authenticate a validly-signed token regardless of the token's `kid` (or its absence), matching `v1.5.0` behavior.
- **REQ-002**: HTTP-cached JWKS with two or more keys MUST continue to require `kid` matching (`jwt.WithKeySet`), unchanged from current behavior.
- **REQ-003**: The single-key-vs-keyset decision for HTTP-cached issuers MUST be evaluated per-request against the live `jwk.Set` snapshot returned by `j.cache.Get(...)`, not precomputed at startup, because the background `jwk.Cache` refresh (`jwk.WithRefreshWindow(2*time.Minute)`) can change the key count between requests (e.g. during key rotation).
- **REQ-004**: File-based (`isLocalFile == true`) JWKS behavior MUST remain unchanged (already correct).
- **SEC-001**: The fix must not weaken `kid` matching for multi-key sets; only the pre-existing single-key bypass is restored.
- **CON-001**: All existing tests in `internal/jwtsupport/jwt_test.go` must continue to pass unchanged.
- **CON-002**: No changes to the pre-computed static options (`jwt.WithValidate(true)`, `jwt.WithVerify(true)`, audience options) built in `buildParseOptions()` — only the key-selection option for the HTTP-cached branch changes.
- **GUD-001**: Reuse the exact same `ks.Len() == 1` branch logic already present for the file-based path in `buildParseOptions()`, applied at request time in `Authenticate()` instead of at startup.

Update the status tag on each task (`[📋 Planned]` → `[⏳ In Progress]` → `[✅ Completed: YYYY-MM-DD]`) as work progresses.

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD**: [/.specs/PRD.md](../PRD.md)
- **Related issue**: [.specs/issues/2026-09-29 Andreas-1.md](../issues/2026-09-29%20Andreas-1.md)
- **Related prior plan (introduced the regression)**: [.specs/plan/optimize-jwt-options-precompute-1.md](optimize-jwt-options-precompute-1.md)
- **Technology Stack**: Go, `github.com/lestrrat-go/jwx/v2` (v2.1.6, see `go.mod`)
- **Affected files**: [internal/jwtsupport/jwt.go](../../internal/jwtsupport/jwt.go)
- **Key type**: `JWTSupport` struct in [internal/jwtsupport/jwt.go](../../internal/jwtsupport/jwt.go)
- **Key method**: `Authenticate()` — HTTP-cached branch inside `for i, wc := range j.wellknownList` / `for audIndex, aud := range j.audiences`

## 2. Implementation Steps

### Implementation Phase 1 — Verify: reproduce the error

- **GOAL-001**: Prove the regression exists with a deterministic, automated test that fails against the current `main` branch, before any fix is applied.

- **TASK-001**: Add a failing reproduction test `TestAuthenticate_HTTPKeySet_SingleKey_KidMismatch` to [internal/jwtsupport/jwt_test.go](../../internal/jwtsupport/jwt_test.go) `[✅ Completed: 2026-09-29]`
  - Start an `httptest.NewServer` (`jwksServer`) that serves a JWKS JSON body containing exactly **one** RSA public key with `kid: "jwks-key-1"` and `alg: "RS256"` at a path such as `/jwks.json`.
  - Start a second `httptest.NewServer` (`wellKnownServer`) that serves a well-known/OIDC-discovery JSON body: `{"jwks_uri": "<jwksServer.URL>/jwks.json", "id_token_signing_alg_values_supported": ["RS256"]}`. Using two separate HTTP servers (not `file://`) is required so `wc.isLocalFile == false`, exercising the same code path as QA's HTTP-fetched well-known.
  - Construct `j := jwtsupport.New([]string{wellKnownServer.URL}, "aud", []string{"test-audience"}, "bearer", false)` so the real `LoadWellKnowns()` → `LoadJWKS()` → `buildParseOptions()` sequence runs exactly as it does in production.
  - Generate an RSA key pair for signing. Set the **private** signing key's `kid` to `"jwks-key-2"` — deliberately different from the JWKS key's `kid` (`"jwks-key-1"`), simulating a gateway-issued token whose key id does not match QA's single-key JWKS (e.g. after key rotation on the issuer side that the JWKS document hasn't caught up with, or a differently-configured signer).
  - Sign a valid token (`aud: "test-audience"`, valid `exp`) with that private key via `jwt.Sign(tok, jwt.WithKey(jwa.RS256, privateJWK))`.
  - Call `j.Authenticate(info, req)` with the signed token in `info.Request.Auth`.
  - **Expected assertion for this task**: the call currently returns `types.ErrAuthenticationFailed` (a 401). Assert this explicitly with a comment noting it captures the `v1.6.0` regression: `// This currently fails (401) due to the v1.6.0 HTTP-keyset regression; TASK-003 fixes it.`
  - Run: `go test ./internal/jwtsupport/... -run TestAuthenticate_HTTPKeySet_SingleKey_KidMismatch -v`
  - Confirm the test **passes** (i.e., it correctly asserts today's broken behavior) — this is the reproduction, not the fix.

- **TASK-002**: Capture the exact underlying `jwx` error for the issue record `[✅ Completed: 2026-09-29]`
  - Temporarily log or `t.Logf("%v", err)` the error returned by `jwt.Parse` in the reproduction test.
  - Expected error text: `failed to find key with key ID "jwks-key-2" in key set` (confirms `keySetProvider.FetchKeys` in `jws/key_provider.go` is rejecting the token due to `kid` mismatch against a single-key set).
  - No code changes in this task — observation only, to be referenced in the fix commit message / PR description.
  - **Confirmed**: captured log line reads exactly `error="key provider 0 failed: failed to find key with key ID \"jwks-key-2\" in key set"`, matching the expected text.

### Implementation Phase 2 — Fix

- **GOAL-002**: Restore the single-key bypass for HTTP-cached JWKS, evaluated per-request against the live keyset.

- **TASK-003**: Update the HTTP-cached branch in `Authenticate()` `[✅ Completed: 2026-09-29]`
  - File: [internal/jwtsupport/jwt.go](../../internal/jwtsupport/jwt.go), inside the `for audIndex, aud := range j.audiences` loop (currently around lines 317–333).
  - Replace:
    ```go
    } else {
        // For HTTP-cached issuers, we need to inject the live keyset snapshot reference.
        // Prepend a fresh jwt.WithKeySet(ks) option to a copy of the pre-built options
        // (the first element [0] is the nil/static placeholder WithKeySet).
        options = make([]jwt.ParseOption, len(j.parseopts[i][audIndex]))
        copy(options, j.parseopts[i][audIndex])
        options[0] = jwt.WithKeySet(ks)
    }
    ```
  - With:
    ```go
    } else {
        // For HTTP-cached issuers the keyset can change between requests
        // (background refresh), so the single-key-vs-keyset decision must be
        // made per-request against the live snapshot, mirroring the
        // file-based branch's startup-time logic.
        options = make([]jwt.ParseOption, len(j.parseopts[i][audIndex]))
        copy(options, j.parseopts[i][audIndex])
        options[0] = jwt.WithKeySet(ks)
        if ks.Len() == 1 {
            if key, ok := ks.Key(0); ok {
                options[0] = jwt.WithKey(key.Algorithm(), key)
            }
        }
    }
    ```
  - Dependencies: TASK-001 must exist first so this task's completion can be verified against it.

- **TASK-004**: Review the placeholder comment in `buildParseOptions()` for accuracy `[✅ Completed: 2026-09-29]`
  - File: [internal/jwtsupport/jwt.go](../../internal/jwtsupport/jwt.go), `else` branch under `if wc.isLocalFile` (around lines 124–135).
  - The `jwt.WithKeySet(nil)` placeholder for non-local (HTTP) issuers is unused after TASK-003 (its `options[0]` is always overwritten at request time). No functional change required; add a one-line comment noting the placeholder value is always replaced in `Authenticate()`.

### Implementation Phase 3 — Confirm: verify the fix works

- **GOAL-003**: Confirm the reproduction test now passes with the corrected outcome, the full suite is green, and the fix is safe to release.

- **TASK-005**: Flip the reproduction test's expected outcome `[✅ Completed: 2026-09-29]`
  - File: [internal/jwtsupport/jwt_test.go](../../internal/jwtsupport/jwt_test.go)
  - Update `TestAuthenticate_HTTPKeySet_SingleKey_KidMismatch` (from TASK-001) so it now asserts **success**: `j.Authenticate(...)` returns `nil` and `info.JWT` is populated, even though the token's `kid` (`"jwks-key-2"`) does not match the JWKS key's `kid` (`"jwks-key-1"`).
  - Rename the test to `TestAuthenticate_HTTPKeySet_SingleKey_BypassesKidMismatch` to reflect the confirmed, fixed behavior (or keep the original name and add a second assertion phase — either is acceptable as long as the final assertion checks success).
  - Run: `go test ./internal/jwtsupport/... -run TestAuthenticate_HTTPKeySet_SingleKey -v`
  - Acceptance: test passes.

- **TASK-006**: Add a regression guard for the multi-key HTTP case `[✅ Completed: 2026-09-29]`
  - File: [internal/jwtsupport/jwt_test.go](../../internal/jwtsupport/jwt_test.go)
  - Add `TestAuthenticate_HTTPKeySet_MultipleKeys_KidMismatchStillRejected`: same setup as TASK-001 but the JWKS serves **two** keys, and the token's `kid` matches neither. Assert `Authenticate` still returns `types.ErrAuthenticationFailed`.
  - This proves REQ-002 (multi-key `kid` enforcement is untouched by the fix).
  - Acceptance: test passes.

- **TASK-007**: Run the full unit test suite `[✅ Completed: 2026-09-29]`
  - Command: `go test ./...`
  - Acceptance: all tests pass, no regressions in `internal/jwtsupport`, `internal/router`, `internal/application`, or other packages.

- **TASK-008**: Run the race detector on the affected package `[⚠️ Blocked: no C compiler available (cgo required for -race), no sudo to install gcc)]`
  - Command: `go test -race ./internal/jwtsupport/...`
  - Acceptance: no data races reported (the per-request `ks.Len()`/`ks.Key(0)` calls read a `jwk.Set` snapshot returned by `cache.Get`, which must be safe for concurrent read access — confirm via the race detector rather than assumption).
  - **Note**: `CGO_ENABLED=0` and neither `gcc` nor `cc` is installed in this environment; installing one requires `sudo` which is unavailable here. Re-run this task in an environment with a C toolchain (e.g. CI) before merging.

- **TASK-009**: Manual/staging confirmation against the real QA scenario `[❌ Cancelled: skipped by user request]`
  - Prerequisite: cluster access to the QA Internal-BFF sidecar (as described in the linked issue).
  - Build and deploy a QA-only image tagged from this fix branch (do not push to `v1`).
  - Run the same failing journey (External BFF → Internal BFF: cart add, favorites, `v2/customer`, `markets/detail`) against the fix image.
  - Acceptance: `kubectl logs <internal-bff-qa-pod> -c <restrego-container>` shows successful authentication (no `401`/`failed to find key with key ID` entries) and the journeys return `200`.
  - This is the final, environment-level confirmation that complements the automated tests in TASK-005–TASK-008.

## 3. Alternatives

- **ALT-001**: Use `jwt.WithKeySet(ks, jws.WithRequireKid(false))` instead of switching to `jwt.WithKey(alg, key)` for the single-key case. This still requires `alg` to match and lets the library try the sole key without requiring `kid` presence/match. Not chosen because it changes the exact verification code path relative to `v1.5.0` (introduces `jws.WithRequireKid` semantics not previously exercised) where the goal here is a minimal, behavior-preserving regression fix; can be reconsidered later as a deliberate hardening change with its own plan.
- **ALT-002**: Recompute `parseopts[i][audIndex][0]` at cache-refresh time (e.g. via a `jwk.Cache` refresh callback) instead of per-request. Rejected: `jwk.Cache` in this version does not expose a refresh-completion callback suitable for this without additional polling/locking complexity; the per-request `ks.Len()` check is O(1) and already required to obtain `ks` itself.
- **ALT-003**: Revert `07d888d` entirely instead of a targeted fix. Rejected: would also discard the legitimate allocation-reduction work for `jwt.WithValidate`, `jwt.WithVerify`, and audience options that is unaffected by this bug.

## 4. Dependencies

- **DEP-001**: `github.com/lestrrat-go/jwx/v2` v2.1.6 — no version change required; fix uses existing `jwt.WithKey` / `jwt.WithKeySet` APIs already imported.

## 5. Files

- **FILE-001**: [internal/jwtsupport/jwt.go](../../internal/jwtsupport/jwt.go) — `Authenticate()` HTTP-cached branch (fix), `buildParseOptions()` (comment only)
- **FILE-002**: [internal/jwtsupport/jwt_test.go](../../internal/jwtsupport/jwt_test.go) — new reproduction test, flipped assertion, new regression guard test

## 6. Testing

- **TEST-001**: `TestAuthenticate_HTTPKeySet_SingleKey_KidMismatch` / renamed `..._BypassesKidMismatch` — reproduces then confirms the fix (Phase 1 / Phase 3)
- **TEST-002**: `TestAuthenticate_HTTPKeySet_MultipleKeys_KidMismatchStillRejected` — confirms no regression for multi-key sets
- **TEST-003**: `go test ./...` — full suite
- **TEST-004**: `go test -race ./internal/jwtsupport/...` — concurrency safety
- **TEST-005**: Manual QA staging verification per TASK-009

## 7. Risks & Assumptions

- **RISK-001**: If QA's real JWKS key count changes to more than one key before the fix ships, the manual confirmation in TASK-009 would need re-validation against the actual production key count at deploy time.
- **RISK-002**: Restoring the single-key bypass reintroduces the same trust model as `v1.5.0` (accepting a validly-signed token regardless of `kid` when only one key is published). This is a deliberate, scoped restoration of prior behavior, not a new weakening — flagged here for reviewer awareness (SEC-001).
- **ASSUMPTION-001**: The QA Internal-BFF's OIDC well-known/JWKS endpoint is reachable over HTTP(S) (not `file://`), consistent with the issue's description of a routine QA environment; the fix targets the `isLocalFile == false` branch specifically.
- **ASSUMPTION-002**: The gateway's tokens are validly signed by a key that matches the algorithm of the sole published JWKS key; the regression is about `kid` matching, not signature validity itself.

## 8. Related Specifications / Further Reading

[.specs/issues/2026-09-29 Andreas-1.md](../issues/2026-09-29%20Andreas-1.md) — original incident report and hypothesis
[.specs/plan/optimize-jwt-options-precompute-1.md](optimize-jwt-options-precompute-1.md) — plan that introduced the regression
[lestrrat-go/jwx v2 jws.WithKeySet](https://pkg.go.dev/github.com/lestrrat-go/jwx/v2/jws#WithKeySet) — documents `requireKid`/`useDefault` defaults

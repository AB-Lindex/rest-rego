---
goal: Pre-compute JWT parse options at startup to eliminate per-request allocations
version: "1.0"
date_created: 2026-06-08
owner: rest-rego team
status: 'Planned'
tags: [performance, memory, jwt, investigation]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

Every call to `JWTSupport.Authenticate()` rebuilds a `[]jwt.ParseOption` slice inside the audience loop. For a single issuer with one audience this allocates a new slice on each request; with three audiences the slice is rebuilt three times. The `jwt.WithKey`, `jwt.WithKeySet`, `jwt.WithValidate`, `jwt.WithVerify`, and `jwt.WithAudience` / `jwt.WithClaimValue` option values are pure functions of configuration that is fixed at startup — they can be computed once and stored.

**Measured baseline** (arm64):

| Benchmark                                               | ns/op   | B/op   | allocs/op |
|---------------------------------------------------------|---------|--------|-----------|
| `BenchmarkAuthenticate` (1 audience)                    | 71 553  | 18 478 | 246       |
| `BenchmarkAuthenticate_MultipleAudiences` (3 audiences) | 179 949 | 51 361 | 702       |

The allocation count scales **linearly with the number of audiences** (702 ÷ 246 ≈ 2.85× for 3×), which confirms the per-audience-per-call construction is the driver.

Key sites from `go tool pprof -alloc_space` heap-dumps/jwt-auth.out:

| Site                                                   | MB   | % total          |
|--------------------------------------------------------|------|------------------|
| `encoding/json.(*Decoder).refill` (inside `jwt.Parse`) | 63.6 | 32.6%            |
| `encoding/json.NewDecoder`                             | 17.0 | 8.7%             |
| `lestrrat-go/option.New`                               | —    | per option value |

Note: the `json.Decoder` allocations are internal to `jwt.Parse()` and cannot be eliminated without replacing the jwx library. The option-slice reallocation is the part under our control.

## 1. Requirements & Constraints

- **REQ-001**: `[]jwt.ParseOption` slices are constructed once per (issuer, audience) pair during `LoadJWKS()` / `New()` and stored on `JWTSupport`
- **REQ-002**: File-based (static) JWKS and HTTP-cached JWKS must both be supported; the key option (`jwt.WithKey` vs `jwt.WithKeySet`) differs between them
- **REQ-003**: `BenchmarkAuthenticate -benchmem` must show a measurable reduction in `allocs/op`
- **REQ-004**: All existing `jwtsupport` tests must pass unchanged
- **CON-001**: The HTTP-cached JWKS (`j.cache.Get(...)`) returns a `jwk.Set` that may be refreshed at any time by the jwx background goroutine; the _set reference_ stored in the pre-built option must be treated as a snapshot — either re-snapshot at refresh time or use `jwt.WithKeySet` with a live reference (see RISK-001)
- **CON-002**: `JWTSupport` is constructed once at startup and never mutated after `New()` returns (confirmed: no setters exist); pre-built options are safe to read concurrently
- **GUD-001**: Pre-built options must not be modified after construction; `jwt.ParseOption` values are immutable once created

Update the status tag on each task (`[📋 Planned]` → `[⏳ In Progress]` → `[✅ Completed: YYYY-MM-DD]`) as work progresses.

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **Technology Stack**: Go, `github.com/lestrrat-go/jwx/v2`
- **Affected files**: [internal/jwtsupport/jwt.go](internal/jwtsupport/jwt.go)
- **Key type**: `JWTSupport` struct in [internal/jwtsupport/jwt.go](internal/jwtsupport/jwt.go)
- **Key method**: `Authenticate()` — inner loop starting at `for _, aud := range j.audiences`

## 2. Implementation Steps

### Preparation — Baseline Recording

- **GOAL-001**: Record a before-snapshot immediately before making any code changes.

- **TASK-001**: Record a before-snapshot immediately before making any code changes `[📋 Planned]`
  - Command:
    ```
    ./e2e-tests/bench-snapshot.sh --label=before-jwt-opts
    ```
  - This writes profiles and a summary to heap-dumps/<timestamp>_before-jwt-opts.*
  - Note the exact prefix printed by the script — it is needed in TASK-006

### Implementation Phase 1 — Data structure

- **GOAL-002**: Add a field to `JWTSupport` that holds pre-built parse option slices indexed by issuer and audience.

- **TASK-002**: Add `parseopts` field to `JWTSupport` struct `[📋 Planned]`
  - File: [internal/jwtsupport/jwt.go](internal/jwtsupport/jwt.go)
  - Add field: `parseopts [][]jwt.ParseOption` — outer index = well-known issuer index, inner index = audience index
  - The slice is populated in Phase 2 and consumed in Phase 3

### Implementation Phase 2 — Construction

- **GOAL-003**: Populate `parseopts` after JWKS loading is complete so all key material is available.

- **TASK-003**: Add `buildParseOptions()` method to `JWTSupport` `[📋 Planned]`
  - File: [internal/jwtsupport/jwt.go](internal/jwtsupport/jwt.go)
  - Signature: `func (j *JWTSupport) buildParseOptions()`
  - For each well-known index `i` and each audience `aud`:
    - If `j.wellknownList[i].isLocalFile`: build key option from `j.JWKS[i]` using same single-key / keyset branch as current `Authenticate()`
    - If HTTP-cached: build option using `jwt.WithKeySet` with a nil placeholder — see RISK-001 for the deferred handling
    - Append `jwt.WithValidate(true)`, `jwt.WithVerify(true)`
    - Append `jwt.WithAudience(aud)` or `jwt.WithClaimValue(j.audienceKey, aud)` depending on `j.audienceKey`
    - Store as `j.parseopts[i][audIndex]`
  - Call `j.buildParseOptions()` at the end of `New()`, after `LoadJWKS()` completes

### Implementation Phase 3 — Use pre-built options

- **GOAL-004**: Replace the per-call option construction in `Authenticate()` with a lookup into `j.parseopts`.

- **TASK-004**: Simplify the inner loop in `Authenticate()` `[📋 Planned]`
  - File: [internal/jwtsupport/jwt.go](internal/jwtsupport/jwt.go)
  - Replace the block that builds `var options []jwt.ParseOption` with a lookup: `options := j.parseopts[i][audIndex]`
  - For HTTP-cached issuers: prepend a fresh `jwt.WithKeySet(ks)` option to a copy of the base slice (the live JWKS snapshot must still be injected per-call; only the static options are pre-built)
  - Result: static options are never re-allocated; only the per-call HTTP key option is allocated when needed

### Implementation Phase 4 — Verification

- **GOAL-005**: Confirm measurable allocation reduction using the snapshot script so results are reproducible and comparable.

- **TASK-005**: Run full test suite `[📋 Planned]`
  - Command: `go test ./...`
  - Run this after the code changes in Preparation and Phases 1–3 are complete

- **TASK-006**: Record an after-snapshot and compare `[📋 Planned]`
  - Command:
    ```
    ./e2e-tests/bench-snapshot.sh --label=after-jwt-opts
    ```
  - Compare the raw numbers in the two .bench.txt files:
    - `BenchmarkAuthenticate-N`: expected reduction in `allocs/op`
    - `BenchmarkAuthenticate_MultipleAudiences-N`: expected proportionally larger reduction
  - Diff the pprof profiles interactively to confirm `lestrrat-go/option.New` sites shrink:
    ```
    go tool pprof -diff_base heap-dumps/<before>.jwt-auth.out heap-dumps/<after>.jwt-auth.out
    go tool pprof -diff_base heap-dumps/<before>.jwt-multi.out heap-dumps/<after>.jwt-multi.out
    ```
  - Acceptance: `allocs/op` decreases for both benchmark variants; `lestrrat-go/option.New` is no longer in the pprof top-20 for the file-based issuer path

## 3. Alternatives

- **ALT-001**: Pool `[]jwt.ParseOption` slices with `sync.Pool` — reduces GC pressure without requiring structural changes, but does not eliminate the allocations inside each `jwt.With*` constructor call; pre-computation is strictly better for static options
- **ALT-002**: Replace jwx with a lower-allocation JWT library — high risk, significant rework; deferred
- **ALT-003**: Pre-build options only for file-based issuers and leave HTTP-cached issuers unchanged — partial win, simpler implementation; consider as a fallback if RISK-001 is blocking

## 4. Dependencies

- **DEP-001**: `github.com/lestrrat-go/jwx/v2` — already present; `jwt.ParseOption` values are treated as immutable; no version change needed

## 5. Files

- **FILE-001**: [internal/jwtsupport/jwt.go](internal/jwtsupport/jwt.go) — `JWTSupport` struct, `New()`, new `buildParseOptions()`, `Authenticate()`
- **FILE-002**: [internal/jwtsupport/jwt_test.go](internal/jwtsupport/jwt_test.go) — existing tests must pass; benchmarks already present

## 6. Testing

- **TEST-001**: `go test ./internal/jwtsupport/...` — all existing unit tests
- **TEST-002**: `go test -bench='^BenchmarkAuthenticate' -benchmem ./internal/jwtsupport/` — before/after comparison
- **TEST-003**: Verify concurrent safety: `go test -race ./internal/jwtsupport/...`

## 7. Risks & Assumptions

- **RISK-001**: For HTTP-cached JWKS, the `jwk.Cache.Get()` returns a `jwk.Set` snapshot. Storing it inside a pre-built option would pin a stale keyset. Mitigation: for HTTP issuers, pre-build only the static options (validate, verify, audience) into a base slice and prepend the freshly-fetched key option per call — this is one fewer allocation than today but not zero
- **RISK-002**: `jwt.WithKey(alg, key)` may capture the key by value or by interface; if by interface it boxes the key on every construction. Verify with pprof after implementation
- **ASSUMPTION-001**: `JWTSupport` is not mutated after `New()` returns — confirmed by code inspection; no method modifies `wellknownList`, `JWKS`, `audiences`, or `audienceKey` after construction

## 8. Related Specifications / Further Reading

[lestrrat-go/jwx v2 jwt.ParseOption](https://pkg.go.dev/github.com/lestrrat-go/jwx/v2/jwt#ParseOption)
Benchmark profile — JWT auth top allocators: heap-dumps/jwt-auth.out (local, generated during investigation)

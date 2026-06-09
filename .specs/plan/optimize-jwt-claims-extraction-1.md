---
goal: Reduce JWT claim extraction allocations by replacing token.AsMap with direct field iteration
version: "1.0"
date_created: 2026-06-08
owner: rest-rego team
status: 'Planned'
tags: [performance, memory, jwt, investigation]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

After a JWT is validated in `Authenticate()`, its claims are extracted via `token.AsMap(ctx)`. Internally, `AsMap` calls `token.makePairs()` to build an intermediate `[]Pair` slice, then copies each pair into a new `map[string]any`. Both the pairs slice and the map are heap-allocated on every successful authentication.

The pprof profile shows this path contributes:

| Site | MB | % total |
|---|---|---|
| `jwt.(*stdToken).makePairs` | 5.5 | 2.8% |
| `jwt.(*stdToken).Iterate` | 11.0 | 5.6% |
| `reflect.mapassign_faststr0` (map insertions) | 4.5 | 2.3% |

The alternative is to use `token.Fields()` (returns claim names as `[]string`) paired with direct `token.Get(name)` calls to build the map incrementally, or to implement the `jwt.Visitor` interface with `token.Walk()` to avoid the pairs intermediate entirely. The resulting `map[string]any` (stored in `info.JWT`) is still necessary because Rego policies access JWT claims by key.

This is the **lowest-impact** of the three optimisations: it affects ~10% of the JWT-path bytes. Implement it last, after OPT-002 (pre-computed parse options), to ensure the benchmark numbers are correctly attributed.

**Measured baseline** (arm64, `BenchmarkAuthenticate`):

| Metric | Value |
|---|---|
| `B/op` | 18 478 |
| `allocs/op` | 246 |
| `makePairs` + `Iterate` contribution | ~16.6 MB / ~8.4% of total bytes |

## 1. Requirements & Constraints

- **REQ-001**: `info.JWT` must remain a `map[string]any` (the Rego input structure expects this type; changing it would break all existing policies)
- **REQ-002**: All standard JWT registered claims (`sub`, `aud`, `iss`, `iat`, `exp`, `nbf`, `jti`) and all private claims must be included in the extracted map
- **REQ-003**: `BenchmarkAuthenticate -benchmem` must show a measurable reduction in `allocs/op` after the change
- **REQ-004**: All existing `jwtsupport` tests must pass unchanged
- **CON-001**: jwx v2 `token.AsMap` is the only documented way to get all claims as `map[string]any`; using `token.Walk` or `token.Iterate` is internal API — verify stability before committing
- **CON-002**: The `aud` claim in jwx v2 is stored as `[]string` internally but exposed as `jwt.ClaimPair` in iteration; the extracted map must preserve the type that Rego policies already depend on
- **GUD-001**: Measure with pprof after implementation to confirm the `makePairs` site disappears from the top-20 allocation list

Update the status tag on each task (`[📋 Planned]` → `[⏳ In Progress]` → `[✅ Completed: YYYY-MM-DD]`) as work progresses.

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **Technology Stack**: Go, `github.com/lestrrat-go/jwx/v2`
- **Affected file**: `internal/jwtsupport/jwt.go`
- **Current line**: `fields, _ := token.AsMap(r.Context())` (after successful parse in `Authenticate()`)
- **Prerequisite**: Implement after OPT-002 (pre-computed parse options) to isolate attribution

## 2. Implementation Steps

### Implementation Phase 1 — Research

- **GOAL-001**: Confirm the correct jwx v2 API to replace `token.AsMap` without the intermediate pairs allocation.

- **TASK-001**: Evaluate `token.Fields()` + `token.Get()` vs `token.Walk()` `[📋 Planned]`
  - File: `internal/jwtsupport/jwt.go` (read-only for this task)
  - Check if `jwt.Token` exposes `Fields() []string` in jwx v2 — if yes, iterate and call `Get(name)`
  - Check if `token.Walk(ctx, visitor)` is available and stable in jwx v2 — the `jwt.Visitor` interface has `VisitField(name string, value interface{}) error`
  - Select the approach that avoids the `makePairs` intermediate; document the choice in the PR

### Implementation Phase 2 — Implementation

- **GOAL-002**: Replace `token.AsMap` with the lower-allocation extraction path identified in Phase 1.

- **TASK-002**: Replace `token.AsMap` call in `Authenticate()` `[📋 Planned]`
  - File: `internal/jwtsupport/jwt.go`
  - Replace:
    ```go
    fields, _ := token.AsMap(r.Context())
    info.JWT = fields
    ```
  - With (preferred — Walk approach):
    ```go
    fields := make(map[string]any, 10) // pre-size for typical JWT claim count
    _ = token.Walk(r.Context(), jwt.VisitorFunc(func(claim string, value interface{}) error {
        fields[claim] = value
        return nil
    }))
    info.JWT = fields
    ```
  - Or (Fields approach, if Walk is unavailable):
    ```go
    fields := make(map[string]any, 10)
    for _, name := range token.Fields() {
        v, _ := token.Get(name)
        fields[name] = v
    }
    info.JWT = fields
    ```
  - Note: `jwt.VisitorFunc` may not exist in jwx v2; verify the visitor pattern and adjust accordingly

### Implementation Phase 3 — Verification

- **GOAL-003**: Confirm allocation reduction and no regression.

- **TASK-003**: Run benchmarks before and after `[📋 Planned]`
  - Command: `go test -run='^$' -bench='^BenchmarkAuthenticate' -benchmem ./internal/jwtsupport/ 2>/dev/null`
  - Also generate pprof: `go test -run=NOMATCH -bench='^BenchmarkAuthenticate$' -memprofile=heap-dumps/jwt-auth-v2.out ./internal/jwtsupport/ 2>/dev/null`
  - Confirm `jwt.(*stdToken).makePairs` is no longer in pprof top-20

- **TASK-004**: Run existing tests and race detector `[📋 Planned]`
  - Commands: `go test ./internal/jwtsupport/...` and `go test -race ./internal/jwtsupport/...`

- **TASK-005**: Manually verify `aud` claim type in extracted map `[📋 Planned]`
  - Existing test `TestAuthenticate_FileBasedKeys` checks `info.JWT["aud"]`; confirm it still passes and the type matches what policies expect (`[]string` or `string`)

## 3. Alternatives

- **ALT-001**: Keep `token.AsMap` but pre-allocate a map and pass it as a sink — not possible; `AsMap` does not accept a destination map
- **ALT-002**: Cache the extracted claims map keyed by the raw token bytes — the token is a byte slice received per-request; caching would require a map with string keys and an eviction strategy; over-engineering for the allocation saving available here
- **ALT-003**: Accept the current `AsMap` allocation and not implement this optimisation — valid given this only contributes ~10% of JWT-path bytes; revisit only if OPT-001 + OPT-002 are insufficient

## 4. Dependencies

- **DEP-001**: `github.com/lestrrat-go/jwx/v2` — already present; `jwt.Token.Walk()` and `jwt.Token.Fields()` availability must be confirmed against the pinned version in `go.mod` before TASK-001 is marked complete
- **DEP-002**: OPT-002 (`optimize-jwt-options-precompute-1.md`) — should be implemented first to isolate attribution

## 5. Files

- **FILE-001**: `internal/jwtsupport/jwt.go` — `Authenticate()` method, claim extraction block only
- **FILE-002**: `internal/jwtsupport/jwt_test.go` — no changes expected; existing tests validate claim extraction

## 6. Testing

- **TEST-001**: `go test ./internal/jwtsupport/...` — all existing unit tests including audience and claim type checks
- **TEST-002**: `go test -bench='^BenchmarkAuthenticate' -benchmem ./internal/jwtsupport/` — before/after comparison
- **TEST-003**: `go test -run=TestAuthenticate_FileBasedKeys -v ./internal/jwtsupport/` — explicit claim-type regression check

## 7. Risks & Assumptions

- **RISK-001**: `jwt.Token.Walk()` in jwx v2 may not be part of the stable public API; if it is internal or removed in a future version, the Fields+Get approach is the fallback
- **RISK-002**: The `aud` claim is handled specially in jwx v2 (stored as `[]string` even for a single audience); the walk/iterate path must preserve this type to avoid breaking policies that check `input.jwt.aud[_]`
- **RISK-003**: The saving is smaller than OPT-001 or OPT-002; if Phase 1 research shows the replacement API is unstable, defer this optimisation
- **ASSUMPTION-001**: A pre-size hint of `10` for the extracted claims map is sufficient for typical JWT payloads (sub, iss, aud, iat, exp, nbf, jti + 3 custom claims); adjust if profiling shows re-growth

## 8. Related Specifications / Further Reading

[lestrrat-go/jwx v2 jwt.Token interface](https://pkg.go.dev/github.com/lestrrat-go/jwx/v2/jwt#Token)
[Benchmark profile — JWT auth top allocators](heap-dumps/jwt-auth.out) (local, generated during investigation)
[OPT-002 pre-computed parse options](.specs/plan/optimize-jwt-options-precompute-1.md)

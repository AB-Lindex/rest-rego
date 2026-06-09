---
goal: Reduce per-eval OPA allocations by disabling internal instrumentation
version: "1.0"
date_created: 2026-06-08
owner: rest-rego team
status: 'Planned'
tags: [performance, memory, rego, investigation]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

Profiling the `BenchmarkValidate` benchmark revealed that **52% of all bytes allocated** per `Validate()` call originate inside `rego.preparedQuery.newEvalContext`, which allocates a fresh `metrics.Metrics` object (timers, hasher maps) on every evaluation even though the results are never consumed by rest-rego.

Adding `rego.EvalInstrument(false)` to the `Eval()` call inside `regocache.Validate()` is a single-line change that tells OPA to skip those per-eval metric structures entirely.

**Measured baseline** (arm64, `BenchmarkValidate`):

| Variant | ns/op | B/op | allocs/op |
|---|---|---|---|
| Simple policy | 22 878 | 10 766 | 204 |
| Large input | 30 134 | 14 953 | 320 |

Top allocating sites from `go tool pprof -alloc_space`:

| Site | MB | % total |
|---|---|---|
| `rego.preparedQuery.newEvalContext` | 334 | 52% |
| `metrics.(*metrics).Timer` | 22 | 3.5% |
| `metrics.New` | 11.5 | 1.8% |

## 1. Requirements & Constraints

- **REQ-001**: Allocations per `Validate()` call must decrease after the change; verify with `BenchmarkValidate -benchmem`
- **REQ-002**: All existing `regocache` tests must continue to pass
- **REQ-003**: The `debug` print-hook path must remain unaffected (it is orthogonal to instrumentation)
- **CON-001**: `EvalInstrument(false)` is available since OPA v1 (confirmed in `pkg.go.dev/github.com/open-policy-agent/opa/v1/rego#EvalInstrument`); no dependency upgrade required
- **CON-002**: OPA instrumentation data is never read by rest-rego, so disabling it is safe
- **GUD-001**: A before/after benchmark table must be included as a comment in the commit or PR description

Update the status tag on each task (`[📋 Planned]` → `[⏳ In Progress]` → `[✅ Completed: YYYY-MM-DD]`) as work progresses.

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **Technology Stack**: Go, OPA v1
- **Affected file**: `pkg/regocache/rego.go` — `Validate()` method, the `query.Eval(...)` call
- **OPA EvalOption API**: `rego.EvalInstrument(bool)` — `func EvalInstrument(instrument bool) EvalOption`

## 2. Implementation Steps

### Implementation Phase 1 — Code change

- **GOAL-001**: Add `rego.EvalInstrument(false)` to the `query.Eval()` call in `Validate()`.

- **TASK-001**: Edit `pkg/regocache/rego.go` — `Validate()` method `[📋 Planned]`
  - File: `pkg/regocache/rego.go`
  - Locate the line: `rs, err := query.Eval(context.Background(), rego.EvalInput(input), rego.EvalPrintHook(r))`
  - Change to: `rs, err := query.Eval(context.Background(), rego.EvalInput(input), rego.EvalPrintHook(r), rego.EvalInstrument(false))`
  - No other changes needed

### Implementation Phase 2 — Verification

- **GOAL-002**: Confirm allocation reduction with benchmarks.

- **TASK-002**: Run benchmarks before and after and record results `[📋 Planned]`
  - Command: `go test -run='^$' -bench='^BenchmarkValidate' -benchmem ./pkg/regocache/ 2>/dev/null`
  - Expected: `B/op` and `allocs/op` decrease; `ns/op` may decrease slightly or stay similar
  - Acceptance: at minimum `newEvalContext`-attributed bytes disappear from `pprof -alloc_space` top-20

- **TASK-003**: Run full test suite `[📋 Planned]`
  - Command: `go test ./...`
  - Expected: all tests pass

## 3. Alternatives

- **ALT-001**: Pass a shared no-op `metrics.Metrics` implementation via `rego.EvalMetrics(noop)` — more complex to implement; `EvalInstrument(false)` achieves the same goal with a single bool
- **ALT-002**: Pre-allocate a reusable `metrics.Metrics` instance per `RegoCache` and pass it via `rego.EvalMetrics` — allows metrics to be surfaced if needed later; deferred until there is a requirement to expose OPA evaluation metrics to Prometheus

## 4. Dependencies

- **DEP-001**: `github.com/open-policy-agent/opa/v1` — already present in `go.mod`; no version change required

## 5. Files

- **FILE-001**: `pkg/regocache/rego.go` — only the `Validate()` method

## 6. Testing

- **TEST-001**: `go test ./pkg/regocache/...` — all existing unit tests
- **TEST-002**: `go test -bench='^BenchmarkValidate' -benchmem ./pkg/regocache/` — before/after allocation comparison

## 7. Risks & Assumptions

- **RISK-001**: OPA may internally guard `EvalInstrument(false)` in a way that does not eliminate the `newEvalContext` allocation; verify via pprof before closing this task
- **ASSUMPTION-001**: The `metrics` structures allocated inside `newEvalContext` are not shared across evaluations and are fully GC-eligible after each call returns — confirmed by the linear growth pattern in the benchmark profile

## 8. Related Specifications / Further Reading

[OPA EvalInstrument API](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/rego#EvalInstrument)
[Benchmark profile — rego eval top allocators](heap-dumps/rego.out) (local, generated during investigation)

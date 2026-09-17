---
goal: Implement Policy-Driven Custom Metric Labels
version: 1.0
date_created: 2026-09-16
last_updated: 2026-09-16
owner: AB-Lindex Team
status: 'Planned'
tags: [feature, observability, metrics, policy, security]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

This plan implements policy-driven custom Prometheus metric labels in rest-rego. Operators declare a fixed set of custom label names at startup (`METRIC_LABELS`); the Rego policy supplies per-request label **values** via a `labels` result map. Values are sanitised (printable-ASCII only, length-bounded) and defaulted so every registered label position is always filled, matching Prometheus's fixed-label-set requirement while keeping attacker-controlled input safe.

## 1. Requirements & Constraints

- **REQ-001**: Add `MetricLabels []string`, `MetricLabelMaxLength int`, `MetricLabelDefault string` fields to `config.Fields`.
- **REQ-002**: Validate each `MetricLabels` entry against the Prometheus label-name grammar `^[a-zA-Z_][a-zA-Z0-9_]*$`; fail-fast (`os.Exit(1)`) on invalid grammar, collision with reserved names (`method`, `code`, `url`), or duplicates within the list.
- **REQ-003**: Validate `MetricLabelMaxLength > 0`; fail-fast otherwise.
- **REQ-004**: Add `Labels map[string]string` field to `types.Info` to carry raw policy-provided values (mirrors `URL string`).
- **REQ-005**: In `router/policy.go`, after the existing `url` extraction, extract an optional `labels` map from the policy result; copy only string-valued entries into `info.Labels`.
- **REQ-006**: `metrics.New()` accepts the custom label names, max length, and default value; it must build the vector label set as `append([]string{"method", "code", "url"}, customNames...)` and register it for all four HTTP metrics (`http_requests_total`, `http_request_duration_seconds`, `http_request_size_bytes`, `http_response_size_bytes`).
- **REQ-007**: `metrics.Wrap()` must append, in the same order as the registered custom names, the sanitised value read from `info.Labels[name]` (or the default when missing/empty) to the label-value slice passed to every `WithLabelValues` call.
- **REQ-008**: Implement `sanitizeLabelValue(v string, maxLen int, def string) string`: strip non-printable/non-ASCII runes (keep only `0x20`-`0x7E` printable), truncate to `maxLen`, substitute `def` if the result is empty.
- **REQ-009**: When `MetricLabels` is empty, metric label sets and recording path must be byte-identical to current behaviour (no new labels, no sanitisation call).
- **CON-001**: Label **names** are never sanitised or rewritten — invalid names are rejected at startup, not coerced.
- **CON-002**: Label **values** are always coerced to a safe value (never rejected), since Prometheus requires every label position filled on every request.
- **CON-003**: Max length and default value are single global settings shared by all custom labels; no per-label overrides.
- **SEC-001**: Client-controlled values (e.g., headers) must never reach Prometheus label values without passing through `sanitizeLabelValue`.
- **GUD-001**: Follow the existing `url`-label precedent (`resultMap["url"]`) for reading policy results in `policy.go`.
- **GUD-002**: Reuse the existing cardinality warning pattern used for `URLMetricsLevel < 0` when custom labels are configured.
- **PAT-001**: Zero-overhead-when-disabled pattern — no allocation or extra work in `Wrap()` when `len(customNames) == 0`.

Update the status of each task below as the plan progresses.

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD**: `/.specs/PRD.md`
- **Features**: `/.specs/features/policy-driven-metric-labels.md`
- **Technology Stack**: Go
- **Cross-Product Dependencies**: None

## 2. Implementation Steps

### Implementation Phase 1: Configuration

- **GOAL-001**: Add and validate the three new configuration fields.

- **TASK-001**: Add `MetricLabels []string`, `MetricLabelMaxLength int` (default `20`), `MetricLabelDefault string` (default `"-"`) fields to `Fields` in [internal/config/config.go](../../internal/config/config.go), following existing `arg` tag conventions (env: `METRIC_LABELS`, `METRIC_LABEL_MAX_LENGTH`, `METRIC_LABEL_DEFAULT`; flags: `--metric-labels`, `--metric-label-max-length`, `--metric-label-default`). `[✅ Completed: 2026-09-16]`
  - Files: `internal/config/config.go`

- **TASK-002**: Add `validateMetricLabels()` method on `Fields` performing: (a) `MetricLabelMaxLength > 0` check (`os.Exit(1)` + `slog.Error` otherwise), (b) per-name grammar check `^[a-zA-Z_][a-zA-Z0-9_]*$`, (c) reserved-name collision check against `method`, `code`, `url`, (d) duplicate-name check within `MetricLabels`. Call it from `New()` alongside `validateEnvsubst()`/`validateTimeouts()`. `[✅ Completed: 2026-09-16]`
  - Files: `internal/config/config.go`
  - Dependencies: TASK-001

- **TASK-003**: In `New()`, emit a `slog.Warn` cardinality warning when `len(f.MetricLabels) > 0`, mirroring the existing `URLMetricsLevel < 0` warning. `[✅ Completed: 2026-09-16]`
  - Files: `internal/config/config.go`
  - Dependencies: TASK-001

- **TASK-004**: Unit tests in `internal/config/config_test.go` (create if absent) covering: valid label names accepted; invalid grammar rejected; reserved-name collision rejected; duplicate names rejected; `MetricLabelMaxLength <= 0` rejected; defaults (`20`, `"-"`) applied when unset. Since `New()` calls `os.Exit(1)` on failure, test the extracted validation logic as a standalone function/method that returns an `error` instead of exiting directly, OR run failure-path assertions in a subprocess per existing Go testing conventions used elsewhere in this repo — inspect `internal/config` for an existing subprocess-test pattern before choosing an approach. `[✅ Completed: 2026-09-16]`
  - Files: `internal/config/config_test.go`
  - Dependencies: TASK-001, TASK-002, TASK-003

### Implementation Phase 2: Value Transport (types.Info)

- **GOAL-002**: Carry policy-provided label values from the policy handler to the metrics middleware.

- **TASK-005**: Add `Labels map[string]string` field (tag `json:"-"`) to `Info` in [internal/types/request.go](../../internal/types/request.go), placed alongside the existing `URL string` field. `[✅ Completed: 2026-09-16]`
  - Files: `internal/types/request.go`

- **TASK-006**: Unit test in `internal/types/request_test.go` verifying `Info{}` zero-value has a nil `Labels` map and that the field round-trips correctly when set (no JSON marshalling expected, since it is tagged `json:"-"`). `[✅ Completed: 2026-09-16]`
  - Files: `internal/types/request_test.go`
  - Dependencies: TASK-005

### Implementation Phase 3: Policy Result Extraction

- **GOAL-003**: Extract the optional `labels` map from the policy result into `info.Labels`.

- **TASK-007**: In [internal/router/policy.go](../../internal/router/policy.go), after the existing `url` rewrite block (`if url, ok := resultMap["url"].(string); ok && url != ""`), add extraction of `resultMap["labels"].(map[string]interface{})`: for each entry, copy only string-valued entries into a new `map[string]string` assigned to `info.Labels`. Non-string values and a missing/absent `labels` key must be silently ignored (no error, no log). `[✅ Completed: 2026-09-16]`
  - Files: `internal/router/policy.go`
  - Dependencies: TASK-005

- **TASK-008**: Unit tests in `internal/router/policy_test.go` covering: policy result with no `labels` key leaves `info.Labels` nil/empty; `labels` map with string values populates `info.Labels` exactly; `labels` map with non-string values drops those entries while keeping valid ones; `labels` present but not a `map[string]interface{}` is ignored without error. `[✅ Completed: 2026-09-16]`
  - Files: `internal/router/policy_test.go`
  - Dependencies: TASK-007

### Implementation Phase 4: Metrics Registration and Recording

- **GOAL-004**: Register custom label names on all four HTTP metrics and record sanitised values on every request.

- **TASK-009**: Implement `sanitizeLabelValue(v string, maxLen int, def string) string` in [internal/metrics/metrics.go](../../internal/metrics/metrics.go): iterate runes, keep only those with `r < utf8.RuneSelf && unicode.IsPrint(r)`, build a byte slice, truncate to `maxLen`, return `def` if the result is empty. Add `unicode` and `unicode/utf8` imports. `[✅ Completed: 2026-09-16]`
  - Files: `internal/metrics/metrics.go`

- **TASK-010**: Change `New()` signature to accept custom label names (`[]string`), max length (`int`), and default value (`string`) — e.g. `New(customLabels []string, maxLen int, def string)`. Store `customLabels`, `maxLen`, `def` in the package-level `metrics` struct. Build the vector label set as `append([]string{"method", "code", "url"}, customLabels...)` and use it for all four `promauto` vector registrations (`requestsTotal`, `requestDuration`, `requestSize`, `responseSize`). When `customLabels` is empty, the resulting label set must be identical to today's `[]string{"method", "code", "url"}`. `[✅ Completed: 2026-09-16]`
  - Files: `internal/metrics/metrics.go`
  - Dependencies: TASK-009

- **TASK-011**: Update the call site in [internal/application/mgmt.go](../../internal/application/mgmt.go) (`metrics.New()`) to pass `app.config.MetricLabels`, `app.config.MetricLabelMaxLength`, `app.config.MetricLabelDefault`. `[✅ Completed: 2026-09-16]`
  - Files: `internal/application/mgmt.go`
  - Dependencies: TASK-010

- **TASK-012**: Update all other `metrics.New()` call sites (test setup code, e.g. `internal/router/cleanup_test.go`) to pass matching arguments (empty slice, default max length `20`, default value `"-"`, or values under test). `[✅ Completed: 2026-09-16]`
  - Files: `internal/router/cleanup_test.go`
  - Dependencies: TASK-010

- **TASK-013**: Update `Wrap()` in `internal/metrics/metrics.go` to build the label-value slice as `append(make([]string, 0, 3+len(metrics.customLabels)), r.Method, strconv.Itoa(w2.Status()), info.URL)`, then for each name in `metrics.customLabels`, append `sanitizeLabelValue(info.Labels[name], metrics.maxLen, metrics.def)`. Use this slice for all four `WithLabelValues` calls. When `metrics.customLabels` is empty, behaviour and allocations must match the current implementation. `[✅ Completed: 2026-09-16]`
  - Files: `internal/metrics/metrics.go`
  - Dependencies: TASK-010

- **TASK-014**: Unit tests in `internal/metrics/metrics_test.go` (create if absent) covering: `sanitizeLabelValue` — non-printable stripping, control-character stripping, non-ASCII stripping, truncation at the boundary (exact `maxLen`, one over), empty input returns default, already-valid input passes through unchanged; `New()`+`Wrap()` integration — with no custom labels the exposed metric label set is unchanged (`method`, `code`, `url` only); with custom labels configured, `Wrap()` records the correct sanitised value per label in the declared order; a request with no `info.Labels` entry for a registered name records the default value; an unregistered key present in `info.Labels` is never emitted as a label. `[✅ Completed: 2026-09-16]`
  - Files: `internal/metrics/metrics_test.go`
  - Dependencies: TASK-013, TASK-011, TASK-012

### Implementation Phase 5: Documentation & Examples

- **GOAL-005**: Document the feature and provide runnable examples, once all code and tests from Phases 1-4 are complete.

- **TASK-015**: Add a "Custom Metric Labels" section to [docs/METRICS.md](../../docs/METRICS.md) describing `METRIC_LABELS`, `METRIC_LABEL_MAX_LENGTH`, `METRIC_LABEL_DEFAULT`, the `labels` policy result field, the sanitisation rules, and a high-cardinality warning reusing the phrasing of the existing `url`-label warning. `[✅ Completed: 2026-09-16]`
  - Files: `docs/METRICS.md`
  - Dependencies: TASK-014

- **TASK-016**: Add rows for `METRIC_LABELS`, `METRIC_LABEL_MAX_LENGTH`, `METRIC_LABEL_DEFAULT` to [docs/CONFIGURATION.md](../../docs/CONFIGURATION.md) and [docs/ENV-VARS.md](../../docs/ENV-VARS.md), matching the existing table format for other config fields (env var, flag, type, default, description). `[✅ Completed: 2026-09-16]` — scope adjusted per user: `docs/ENV-VARS.md` documents only the envsubst-in-policies feature (no general config table), so it is skipped; rows added to `docs/CONFIGURATION.md` only.
  - Files: `docs/CONFIGURATION.md`, `docs/ENV-VARS.md`
  - Dependencies: TASK-014

- **TASK-017**: Add a "Custom Metric Labels (`labels` result)" subsection to [docs/POLICY.md](../../docs/POLICY.md) alongside the existing `url` result documentation, including the example policy from the feature spec (`labels := {"client_version": v} if { v := input.request.headers["X-Client-Version"] }`). `[✅ Completed: 2026-09-16]`
  - Files: `docs/POLICY.md`
  - Dependencies: TASK-014

- **TASK-018**: Update or add an example under [examples/](../../examples/) demonstrating a policy that sets a custom `labels` result together with the corresponding `METRIC_LABELS` configuration (e.g., extend `examples/no-auth/` or add a new `examples/metric-labels/` folder with its own README section, following the structure of existing examples). `[✅ Completed: 2026-09-16]`
  - Files: `examples/README.md`, new example folder under `examples/`
  - Dependencies: TASK-017

- **TASK-019**: Cross-link the new feature from [docs/POLICY.md](../../docs/POLICY.md) and [docs/METRICS.md](../../docs/METRICS.md) to `.specs/features/policy-driven-metric-labels.md` and to `url-metrics-level.md` (companion cardinality-control feature), matching the existing "Related features" cross-referencing style. `[✅ Completed: 2026-09-16]`
  - Files: `docs/METRICS.md`, `docs/POLICY.md`
  - Dependencies: TASK-015, TASK-017

## 3. Alternatives

- **ALT-001**: Allow the policy to declare label names dynamically at runtime. Rejected — Prometheus requires a fixed label-name set per metric registered at startup; dynamic names would require re-registering vectors per unique name, causing unbounded memory growth and registration races.
- **ALT-002**: Per-label max length and default value configuration. Rejected as unnecessary complexity for the current requirement; a single global pair keeps configuration surface small and matches the feature spec's stated constraint.
- **ALT-003**: Sanitise (rather than reject) invalid `METRIC_LABELS` names. Rejected — silently rewriting an operator-declared name would make emitted series diverge from dashboards/policies referencing that name; fail-fast surfaces the misconfiguration immediately.

## 4. Dependencies

- **DEP-001**: `github.com/prometheus/client_golang` (existing) — `promauto`, `prometheus.CounterVec`/`HistogramVec`/`SummaryVec` support variable label sets already; no version change required.
- **DEP-002**: `github.com/alexflint/go-arg` (existing) — used for `[]string` env/flag parsing of `MetricLabels`, consistent with existing `WellKnownURL []string` and `Audiences []string` fields.
- **DEP-003**: Standard library `unicode` and `unicode/utf8` packages for the sanitisation helper.

## 5. Files

- **FILE-001**: `internal/config/config.go` — new fields, validation, warning.
- **FILE-002**: `internal/config/config_test.go` — validation unit tests.
- **FILE-003**: `internal/types/request.go` — `Labels` field on `Info`.
- **FILE-004**: `internal/types/request_test.go` — `Info.Labels` unit tests.
- **FILE-005**: `internal/router/policy.go` — `labels` result extraction.
- **FILE-006**: `internal/router/policy_test.go` — extraction unit tests.
- **FILE-007**: `internal/metrics/metrics.go` — `sanitizeLabelValue`, `New()` signature change, `Wrap()` recording logic.
- **FILE-008**: `internal/metrics/metrics_test.go` — sanitisation and recording unit tests.
- **FILE-009**: `internal/application/mgmt.go` — updated `metrics.New()` call site.
- **FILE-010**: `internal/router/cleanup_test.go` — updated `metrics.New()` call site.
- **FILE-011**: `docs/METRICS.md`, `docs/CONFIGURATION.md`, `docs/ENV-VARS.md`, `docs/POLICY.md` — documentation updates.
- **FILE-012**: `examples/` — new or updated example demonstrating custom labels.

## 6. Testing

- **TEST-001**: Config validation — accepts valid label names; rejects invalid grammar, reserved-name collisions, duplicates, and non-positive max length.
- **TEST-002**: `types.Info.Labels` — zero value is nil; field is excluded from JSON marshalling.
- **TEST-003**: Policy extraction — populates `info.Labels` from string-valued `labels` map entries only; ignores missing/malformed `labels` key without error.
- **TEST-004**: `sanitizeLabelValue` — non-printable/control/non-ASCII stripping, truncation boundary (`maxLen` and `maxLen+1`), empty-after-sanitisation falls back to default, already-clean input unchanged.
- **TEST-005**: Metrics registration/recording — label set unchanged when no custom labels configured; correct ordered label values recorded when configured; default substitution for missing per-request values; unregistered `info.Labels` keys never emitted.
- **TEST-006**: Full request-path regression — run `go test ./...` to confirm no existing test (e.g. `internal/router`, `internal/application`) breaks due to the `metrics.New()` signature change.

## 7. Risks & Assumptions

- **RISK-001**: Changing `metrics.New()`'s signature is a breaking change for any external caller; mitigated because it is only called from `internal/application/mgmt.go` and test files within this repository (confirmed via workspace search).
- **RISK-002**: Operators may still configure a low-cardinality-unsafe label (e.g., a raw user ID header) despite the documented warning; mitigated by documentation only, consistent with the existing `url`-label precedent — cardinality is explicitly the operator's responsibility per the feature spec.
- **ASSUMPTION-001**: The Rego policy result is already deserialized into `map[string]interface{}` before reaching `policy.go` (confirmed from current `url`-handling code), so `labels` extraction follows the same pattern with no additional deserialization step needed.
- **ASSUMPTION-002**: No other package outside `internal/metrics`, `internal/router`, `internal/config`, `internal/types`, and `internal/application` references the changed symbols (`metrics.New`, `types.Info`), based on workspace search results.

## 8. Related Specifications / Further Reading

- [.specs/features/policy-driven-metric-labels.md](../features/policy-driven-metric-labels.md)
- [.specs/features/url-metrics-level.md](../features/url-metrics-level.md)
- [docs/METRICS.md](../../docs/METRICS.md)
- [docs/POLICY.md](../../docs/POLICY.md)
- [docs/CONFIGURATION.md](../../docs/CONFIGURATION.md)
- [docs/ENV-VARS.md](../../docs/ENV-VARS.md)

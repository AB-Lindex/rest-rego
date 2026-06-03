---
type: "feature"
feature: "url-metrics-level"
status: "proposed"
priority: "medium"
complexity: "simple"
---

# Feature: URL Metrics Level

## Problem Statement

Prometheus metrics in rest-rego use the request path as the `url` label on all HTTP request metrics. When dynamic path segments are present (e.g. resource IDs, user identifiers), every unique path creates a new time series. This leads to unbounded cardinality growth, excessive memory usage in both rest-rego and Prometheus, and potential out-of-memory crashes.

Rego policies can already override the `url` label by returning a `url` value, but this requires policy authors to be aware of the problem and handle every route. A server-side fallback is needed that works without any policy changes.

## User Stories

- As an operator, I want to limit metric cardinality without modifying my Rego policies, so that Prometheus stays healthy in production.
- As a policy author, I want a safe default URL label so that I can refine cardinality control incrementally per route.

## Requirements

### Functional

1. `URL_METRICS_LEVEL` (env) / `--url-metrics-level` (flag) controls how the `url` label is derived from the request path before being recorded in metrics.
2. Behaviour per value:
   - `< 0` — use the full request path (current behaviour, opt-in only)
   - `0` — suppress the path entirely; use an empty string or a fixed placeholder (e.g. `"/"`)
   - `> N` — use only the first N path segments (e.g. `2` → `/orders/items` from `/orders/items/42/detail`)
3. The policy-returned `url` value always takes precedence over the level-based truncation.
4. Default value is `0` (no path in label).

### Non-Functional

- No measurable latency impact on the hot request path.
- Behaviour is documented alongside the existing [URL label documentation](../../docs/METRICS.md).

## Technical Design

- **Config**: `URLMetricsLevel int` already declared in `internal/config/config.go` (line 38). No config changes required.
- **Info population**: `info.URL` is set to `r.URL.Path` in `internal/types/request.go`, immediately after `i.Request.Path` is populated by splitting that same path. The truncation should happen at this point, re-using `i.Request.Path` rather than splitting again.
- **Metrics recording**: `internal/metrics/metrics.go` `Wrap()` reads `info.URL` directly — no changes needed there.
- **Policy override**: `internal/router/policy.go` sets `info.URL` from the policy result. This already overrides whatever base value was set, so policy-based control is preserved automatically.

### Truncation logic (pseudocode)

```
// i.Request.Path is already []string{"orders","items","42","detail"}
func truncateURLFromSegments(path string, segments []string, level int) string:
    if level < 0:
        return path                                 // full path (original r.URL.Path)
    if level == 0:
        return "/"                                  // no detail
    return "/" + strings.Join(segments[:min(level, len(segments))], "/")

// In NewInfo(), replace:
//   i.URL = r.URL.Path
// with:
//   i.URL = truncateURLFromSegments(r.URL.Path, i.Request.Path, level)
```

## Implementation Phases

1. **MVP**: Implement truncation and wire `URLMetricsLevel` into the `Info` population in `internal/types/request.go`. Update METRICS.md.
2. **Enhancement**: Add a validation warning in `config.New()` when level is `< 0` (full path) reminding operators of the cardinality risk.

## Integration

- **Docs impact**: [docs/METRICS.md](../../docs/METRICS.md) — add a section describing `URL_METRICS_LEVEL` alongside the existing policy-based `url` override.
- **Config reference**: [docs/CONFIGURATION.md](../../docs/CONFIGURATION.md) / [docs/ENV-VARS.md](../../docs/ENV-VARS.md) — add the new env var.

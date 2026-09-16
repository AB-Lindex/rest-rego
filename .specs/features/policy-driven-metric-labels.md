---
type: "feature"
feature: "policy-driven-metric-labels"
status: "proposed"
priority: "medium"
complexity: "standard"
---

# Feature: Policy-Driven Custom Metric Labels

## Problem Statement

rest-rego exposes Prometheus HTTP request metrics with a fixed label set of `method`, `code`, and `url`. Operators frequently want to segment these metrics by a dimension derived from the request — for example an `X-Client-Version` header — to understand which client versions are calling a service and how they behave (error rates, latency, traffic share).

Today there is no way to add such a dimension. The `url` label can be rewritten by a Rego policy, which establishes a precedent for policy-controlled metric labels, but no mechanism exists for arbitrary additional labels.

Two constraints shape the solution:

1. **Prometheus requires a fixed set of label names per metric.** Label *names* cannot vary per request, so the set of custom labels must be declared once at startup. Only the label *values* can be dynamic per request.
2. **Label values are attacker-influenceable.** Headers such as `X-Client-Version` are client-controlled, so raw values must be length-bounded and sanitised before becoming Prometheus label values to limit cardinality blow-up and reject malformed/binary input.

## User Stories

- As an operator, I want to add custom metric labels (e.g. client version) whose values are computed by my Rego policy, so that I can segment traffic without changing rest-rego's source.
- As an operator, I want a global maximum length and a global default value for all custom labels, so that malformed or missing values cannot destabilise my metrics pipeline.
- As a policy author, I want to derive a label value from any request data available in the policy input (headers, JWT claims, path), so that I control exactly what dimension is recorded.

## Requirements

### Functional

1. A configuration parameter declares the fixed set of custom label **names** to register on the HTTP request metrics. Without this, no custom labels are added and behaviour is unchanged.
2. Label **names** are validated at startup and **rejected** (fail-fast, `os.Exit(1)`) if they are not valid Prometheus label names, collide with a reserved base label, or duplicate another custom name. Names are never silently sanitised or rewritten (see [Label-name handling](#label-name-handling-reject-never-sanitise)).
3. The Rego policy returns a `labels` object (name → value) in its result. rest-rego reads only the keys matching the registered label names; any unregistered keys are ignored, and a policy can never introduce a new label name at runtime.
3. For every registered label name, on every request, a value is recorded (Prometheus requires all label positions to be filled). The value is resolved as follows:
   1. Take the policy-provided value for that label name.
   2. If it is missing, not a string, or empty after sanitisation, use the **default value**.
   3. Otherwise use the sanitised value.
4. **Sanitisation** (applied identically to every custom label value):
   - Strip all characters that are not printable ASCII (byte range `0x20`–`0x7E`).
   - Truncate the result to the configured **maximum length**.
   - If the result is empty after these steps, substitute the default value.
5. **Maximum length** is a single global config parameter that applies to all custom labels. Default: `20` characters.
6. **Default value** is a single global config parameter that applies to all custom labels. Default: `"-"`.
7. The maximum length and default value are **global** — the same values apply to every custom label. They are not configurable per individual label.
8. When no custom label names are configured, the metric label set and recording path are unchanged from current behaviour (zero overhead, no new labels).

### Non-Functional

- **Performance**: Sanitisation runs on the hot request path once per configured label per request. It must be allocation-light and add no measurable latency when custom labels are unconfigured, and negligible latency when configured.
- **Security (Lindex standard)**:
  - Client-controlled values are always length-bounded and reduced to printable ASCII before use, preventing control-character injection into the metrics exposition format and limiting cardinality growth.
  - Cardinality remains the operator's responsibility: bounding length does **not** bound the number of distinct values. Documentation must carry the same high-cardinality warning as the `url` label.
- **Observability**: Applies to all HTTP request metrics uniformly (`http_requests_total`, `http_request_duration_seconds`, `http_request_size_bytes`, `http_response_size_bytes`).
- **Backwards compatibility**: Default configuration produces byte-identical metric label sets to the current release.

## Technical Design

- **Architecture**: Go reverse-proxy sidecar; single container; no external dependencies added.
- **Technology stack**: Go (existing), `github.com/prometheus/client_golang` (existing).

### Configuration (`internal/config/config.go`)

Three new fields on `Fields`:

| Field | Env / Flag | Type | Default | Purpose |
|-------|------------|------|---------|---------|
| `MetricLabels` | `METRIC_LABELS` / `--metric-labels` | `[]string` | *(empty)* | Fixed list of custom label names to register on HTTP metrics. Empty disables the feature. |
| `MetricLabelMaxLength` | `METRIC_LABEL_MAX_LENGTH` / `--metric-label-max-length` | `int` | `20` | Global maximum length for every custom label value. |
| `MetricLabelDefault` | `METRIC_LABEL_DEFAULT` / `--metric-label-default` | `string` | `"-"` | Global fallback value used when a label is missing, non-string, or empty after sanitisation. |

Validation in `config.New()`:

- `MetricLabelMaxLength` must be `> 0`; otherwise log and `os.Exit(1)`.
- Warn (do not fail) that custom labels can cause unbounded cardinality, mirroring the existing `url-metrics-level` warning.

#### Label-name handling (reject, never sanitise)

Label **names** are operator-supplied via `METRIC_LABELS` and are declared once at startup, so they are validated with a **fail-fast** policy rather than silently rewritten. Silently mangling a name (e.g. `client version` → `client_version`) would make the emitted series diverge from what the operator wrote in dashboards and policies, so any invalid name is rejected outright. Each entry in `MetricLabels` is validated in `config.New()`:

- Must match the Prometheus label-name grammar `^[a-zA-Z_][a-zA-Z0-9_]*$`; otherwise log and `os.Exit(1)`.
- Must not collide with the reserved base labels `method`, `code`, `url`; otherwise log and `os.Exit(1)`.
- Must not duplicate another configured custom label; otherwise log and `os.Exit(1)`.
- Names are used verbatim — no trimming, case-folding, or character substitution is applied.

This contrasts deliberately with label **values** (see [sanitisation](#metrics-registration-and-recording-internalmetricsmetricsgo) below), which originate from untrusted request data and are always coerced to a safe value rather than rejected — because Prometheus requires every label position to be filled on every request.

#### Policy-supplied keys

The keys in the policy's `labels` result map are **matched** against the registered names, never used as names themselves. A key that is not in `MetricLabels` is ignored; a registered name absent from the map resolves to the default value. The policy therefore cannot introduce a new or malformed label name at runtime.

### Value transport (`internal/types/request.go`)

Add a field to `Info` to carry policy-provided label values from the policy handler to the metrics middleware, mirroring how `URL` is carried:

```go
type Info struct {
    Request RequestInfo `json:"request"`
    JWT     interface{} `json:"jwt,omitempty"`
    User    interface{} `json:"user,omitempty"`
    Result  interface{} `json:"result,omitempty"`

    URL    string            `json:"-"`
    Labels map[string]string `json:"-"` // raw policy-provided custom label values
}
```

### Policy result handling (`internal/router/policy.go`)

After the existing `url` override block, extract the optional `labels` map from the policy result and store the raw values on `info`:

```go
if raw, ok := resultMap["labels"].(map[string]interface{}); ok {
    labels := make(map[string]string, len(raw))
    for k, v := range raw {
        if s, ok := v.(string); ok {
            labels[k] = s
        }
    }
    info.Labels = labels
}
```

Non-string values are dropped here and will resolve to the default during sanitisation.

### Metrics registration and recording (`internal/metrics/metrics.go`)

- `New()` gains parameters for the custom label names, max length, and default value (or reads them from the passed config). It stores them in package state and builds the vector label set as `append([]string{"method", "code", "url"}, customNames...)`.
- `Wrap()` builds the label value slice by appending, in the same order as the registered names, the sanitised value for each custom label read from `info.Labels`.

Sanitisation helper (single global rules, same for every label):

```go
// sanitizeLabelValue reduces v to printable ASCII, truncates to maxLen,
// and falls back to def when the result is empty.
func sanitizeLabelValue(v string, maxLen int, def string) string {
    b := make([]byte, 0, len(v))
    for _, r := range v {
        // utf8.RuneSelf (128) bounds to ASCII; unicode.IsPrint excludes control chars and DEL
        if r < utf8.RuneSelf && unicode.IsPrint(r) {
            b = append(b, byte(r))
        }
    }
    s := string(b)
    if len(s) > maxLen { // safe: output is pure ASCII, so byte length == char count
        s = s[:maxLen]
    }
    if s == "" {
        return def
    }
    return s
}
```

Recording loop (conceptual):

```go
labels := append(make([]string, 0, 3+len(customNames)),
    r.Method, strconv.Itoa(w2.Status()), info.URL)
for _, name := range customNames {
    labels = append(labels, sanitizeLabelValue(info.Labels[name], maxLen, def))
}
```

Because `Wrap()` records after the middleware chain (including the policy handler) has run, `info.Labels` is already populated — identical timing to the existing `info.URL` flow.

### Example policy

```rego
package policies

default allow := false
default url := ""
default labels := {}

allow if {
    input.jwt.appid == "11112222-3333-4444-5555-666677778888"
}

# Expose the client version header as a custom metric label.
labels := {"client_version": v} if {
    v := input.request.headers["X-Client-Version"]
}
```

With `METRIC_LABELS=client_version`, requests carrying `X-Client-Version: 2.4.1` record `client_version="2.4.1"`; requests without the header (or with the empty header) record `client_version="-"`.

## Implementation Phases

1. **MVP**:
   - Add the three config fields and validation.
   - Register custom labels in `metrics.New()` and record them in `Wrap()` with the sanitisation helper.
   - Add the `Labels` field to `Info` and populate it in `policy.go`.
   - Update [docs/METRICS.md](../../docs/METRICS.md) and config docs.
2. **Enhancement**:
   - Optional metric/log counter for values that were truncated or replaced by the default, to help operators detect misbehaving clients. (out-of-scope)

## Integration

- **Docs impact**:
  - [docs/METRICS.md](../../docs/METRICS.md) — new section describing policy-driven custom labels, the `labels` policy result field, sanitisation rules, and a reused high-cardinality warning.
  - [docs/CONFIGURATION.md](../../docs/CONFIGURATION.md) and [docs/ENV-VARS.md](../../docs/ENV-VARS.md) — add `METRIC_LABELS`, `METRIC_LABEL_MAX_LENGTH`, `METRIC_LABEL_DEFAULT`.
  - [docs/POLICY.md](../../docs/POLICY.md) — document the optional `labels` result field alongside `allow` and `url`.
- **Related features**: Complements [url-metrics-level.md](url-metrics-level.md); both control metric cardinality, one for the `url` label and one for custom labels.
- **Testing**:
  - Unit tests for `sanitizeLabelValue` (non-printable stripping, truncation boundary, empty → default, non-ASCII, control characters).
  - Unit tests for `policy.go` label extraction (missing map, non-string values, unregistered keys ignored).
  - Config validation tests for label-name rejection (invalid grammar, reserved-name collision, duplicate name).
  - Metrics recording test asserting the label set order and default substitution.

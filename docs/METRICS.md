# Metrics

rest-rego exposes a [Prometheus](https://prometheus.io/) metrics endpoint on the management port (default `8182`) at `/metrics`.

## Available Metrics

### HTTP Request Metrics

These metrics are labelled with `method`, `code`, and `url` (see [URL label](#the-url-label) below).

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Total number of HTTP requests processed |
| `http_request_duration_seconds` | Histogram | Request latency in seconds |
| `http_request_size_bytes` | Summary | Size of incoming request bodies in bytes |
| `http_response_size_bytes` | Summary | Size of outgoing response bodies in bytes |

### Blocked Headers Metrics

These metrics relate to the [blocked headers](BLOCKED-HEADERS.md) feature.

| Metric | Type | Description |
|--------|------|-------------|
| `restrego_blocked_headers_exposed` | Gauge | `1` if `EXPOSE_BLOCKED_HEADERS` is enabled, `0` otherwise |
| `restrego_blocked_headers_captured_total` | Counter | Total number of individual `X-Restrego-*` headers captured |
| `restrego_requests_with_blocked_headers_total` | Counter | Total number of requests that contained `X-Restrego-*` headers |

### Go Runtime Metrics

Standard Go runtime and process metrics are also exposed, including `go_*` and `process_*` series from the Prometheus Go collector.

## The URL Label

The `url` label on HTTP request metrics is used to group requests by logical endpoint. The default behaviour is controlled by `URL_METRICS_LEVEL`.

### URL_METRICS_LEVEL

`URL_METRICS_LEVEL` (env) / `--url-metrics-level` (flag) controls how much of the request path is included in the `url` label before the policy has a chance to override it.

| Value | Behaviour |
|-------|-----------|
| `< 0` | Full request path — use with caution, may cause unbounded cardinality |
| `0` *(default)* | Path is suppressed; label is always `"/"` |
| `N > 0` | First N path segments only (e.g. `2` → `/orders/items` from `/orders/items/42/detail`) |

The policy-returned `url` value (see below) always takes precedence over the level-based value.

### Rewriting the URL Label from Policy

Your Rego policy can override the `url` label by returning a `url` string in the policy result. This is the primary mechanism for controlling metric cardinality.

```rego
package policies

default allow := false
default url := ""

allow if {
    input.jwt.appid == "11112222-3333-4444-5555-666677778888"
}

# Normalize user paths so /user/alice and /user/bob both map to /user/--
url := "/user/--" if {
    input.request.path[0] == "user"
}
```

When the policy returns a non-empty `url` value, that value is used as the `url` label for all HTTP metrics for that request.

Common uses:

- Replace dynamic path segments with a placeholder (e.g., `/orders/123` → `/orders/--`)
- Anonymise paths for GDPR compliance

## High Cardinality Warning

> **Warning:** If the `url` label is allowed to contain dynamic values — such as resource IDs, user identifiers, or query parameters — the number of unique label combinations will grow without bound. This causes Prometheus to create a new time series for every unique value, leading to:
>
> - Excessive memory usage in both rest-rego and your Prometheus server
> - Slow query performance and scrape timeouts
> - Potential out-of-memory crashes under sustained traffic

Always normalise the `url` label in your policy before deploying to production. If you are unsure whether a path contains dynamic segments, set a safe static fallback:

```rego
default url := "/unknown"
```

Then add specific rules that return normalised values for each known route pattern.

## Custom Metric Labels

In addition to rewriting the `url` label, you can register your own custom Prometheus labels on all four HTTP request metrics (`http_requests_total`, `http_request_duration_seconds`, `http_request_size_bytes`, `http_response_size_bytes`) and populate their values per-request from your Rego policy. This is useful for segmenting metrics by dimensions such as client version, tenant, or API version.

### Configuration

| Option | Env Variable | Default | Description |
|--------|--------------|---------|-------------|
| `--metric-labels` | `METRIC_LABELS` | *(empty)* | Fixed list of custom label names to register on the HTTP request metrics. Empty disables the feature. |
| `--metric-label-max-length` | `METRIC_LABEL_MAX_LENGTH` | `20` | Maximum length of a custom metric label value, applied globally to every custom label. |
| `--metric-label-default` | `METRIC_LABEL_DEFAULT` | `-` | Fallback value used when a label is missing, non-string, or empty after sanitisation. |

Label **names** are declared once at startup and validated against the Prometheus label-name grammar (`^[a-zA-Z_][a-zA-Z0-9_]*$`). Names that are invalid, collide with the reserved base labels (`method`, `code`, `url`), or are duplicated cause rest-rego to fail fast (`os.Exit(1)`) at startup — names are never silently sanitised or rewritten.

### The `labels` Policy Result

Your Rego policy supplies per-request label **values** by returning a `labels` object (name → string) in its result:

```rego
package policies

default allow := false
default labels := {}

allow if {
    input.jwt.appid == "11112222-3333-4444-5555-666677778888"
}

# Expose the client version header as a custom metric label.
labels := {"client_version": v} if {
    v := input.request.headers["X-Client-Version"]
}
```

With `METRIC_LABELS=client_version`, requests carrying `X-Client-Version: 2.4.1` record `client_version="2.4.1"`; requests without the header record the default value (`client_version="-"`).

Only keys matching a registered label name are used — unregistered keys in the `labels` map are ignored, and a registered name absent from the map (or missing from the policy entirely) resolves to the default value. A policy can never introduce a new label name at runtime.

### Value Sanitisation

Every custom label value is sanitised identically before being recorded:

1. Strip all characters that are not printable ASCII (byte range `0x20`–`0x7E`).
2. Truncate the result to `METRIC_LABEL_MAX_LENGTH`.
3. If the result is empty after these steps, substitute `METRIC_LABEL_DEFAULT`.

This prevents attacker-controlled values (e.g. from request headers) from injecting control characters into the metrics exposition format or producing unbounded label lengths.

### High Cardinality Warning

> **Warning:** Custom metric label values are still attacker- or client-influenceable. If a label is allowed to contain a large number of distinct values — such as raw user identifiers or unbounded free-text — the number of unique label combinations will grow without bound, causing the same cardinality problems described in the [URL label warning](#high-cardinality-warning) above: excessive memory usage, slow queries, scrape timeouts, and potential out-of-memory crashes.

Choose custom label values with a small, bounded set of expected values (e.g. a client version scheme, a known tenant list) rather than raw, unbounded user input.

### Related Features

- [Policy-Driven Custom Metric Labels](../.specs/features/policy-driven-metric-labels.md) — feature specification for this capability.
- [URL Metrics Level](../.specs/features/url-metrics-level.md) — companion cardinality-control feature for the `url` label.


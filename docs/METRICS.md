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

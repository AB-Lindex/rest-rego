# Custom Metric Labels Example

This example shows how to populate a custom Prometheus metric label (`client_version`) from a Rego policy, using the `X-Client-Version` request header.

## Files

| File           | Description                                                                       |
|----------------|------------------------------------------------------------------------------------|
| `request.rego` | Policy that allows a specific JWT `appid` and returns a `labels` result            |

## Required Environment Variables

| Variable          | Description                                                              |
|-------------------|---------------------------------------------------------------------------|
| `METRIC_LABELS`   | Set to `client_version` to register the custom label at startup           |
| `ALLOWED_APP_ID`  | JWT `appid` value permitted to access the backend                        |
| `BACKEND_PORT`    | Port of the upstream service                                              |
| `WELLKNOWN_OIDC`  | OIDC well-known URL for JWT verification                                  |
| `JWT_AUDIENCES`   | Expected JWT audience(s)                                                   |

See [docs/METRICS.md](../../docs/METRICS.md#custom-metric-labels) for the full configuration and sanitisation reference, including `METRIC_LABEL_MAX_LENGTH` and `METRIC_LABEL_DEFAULT`.

## How It Works

- Requests are allowed only when the JWT `appid` claim matches `ALLOWED_APP_ID`.
- The policy returns `labels := {"client_version": v}` where `v` is taken from the `X-Client-Version` request header.
- Because `METRIC_LABELS=client_version` registers `client_version` as a Prometheus label, every HTTP request metric (`http_requests_total`, etc.) is emitted with a `client_version` label: the sanitised header value when present, or `METRIC_LABEL_DEFAULT` (`-` by default) when the header is missing.

## Quick Start

```bash
export METRIC_LABELS=client_version
export ALLOWED_APP_ID=11112222-3333-4444-5555-666677778888
export BACKEND_PORT=8080
export WELLKNOWN_OIDC=https://login.microsoftonline.com/YOUR-TENANT-ID/v2.0/.well-known/openid-configuration
export JWT_AUDIENCES=api://your-app-id

./restrego
```

```bash
# Requests carrying X-Client-Version are segmented in metrics by that value
curl -H "Authorization: Bearer $TOKEN" -H "X-Client-Version: 2.4.1" http://localhost:8181/api/endpoint

# Check the emitted label on the management port
curl http://localhost:8182/metrics | grep client_version
```

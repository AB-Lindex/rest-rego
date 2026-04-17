# No-Auth Example

This example shows how to deploy rest-rego with `NO_AUTH=true`, where authentication is disabled and the Rego policy is the sole access control mechanism. `input.jwt` and `input.user` are `null` for every request.

## Files

| File           | Description                                                           |
|----------------|-----------------------------------------------------------------------|
| `request.rego` | Policy allowing read-only methods and gating mutations on `X-Api-Key` |

## Required Environment Variables

| Variable           | Description                                                             |
|--------------------|-------------------------------------------------------------------------|
| `NO_AUTH`          | Set to `true` to enable no-auth mode                                    |
| `BACKEND_PORT`     | Port of the upstream service                                            |
| `EXPECTED_API_KEY` | Shared secret required for mutating requests (POST, PUT, PATCH, DELETE) |

## How It Works

- `GET`, `HEAD`, and `OPTIONS` requests are allowed unconditionally.
- All other methods (`POST`, `PUT`, `PATCH`, `DELETE`, …) require the caller to supply the correct shared secret in the `X-Api-Key` request header.
- Any request whose method is not read-only and whose `X-Api-Key` header does not match is denied with `403 Forbidden`.

## Quick Start

```bash
export NO_AUTH=true
export BACKEND_PORT=8080
export EXPECTED_API_KEY=super-secret

./restrego
```

## Security Considerations

See [docs/NO-AUTH.md](../../docs/NO-AUTH.md) for a full discussion of trade-offs and recommended compensating controls such as Kubernetes `NetworkPolicy` and service-mesh mTLS.

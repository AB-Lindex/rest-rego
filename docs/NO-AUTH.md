# No-Auth Mode

No-auth mode (`NO_AUTH=true`) disables all authentication and passes every request directly to the Rego policy engine. `input.jwt` and `input.user` are `null` for every request. The Rego policy becomes the **sole** access control mechanism.

## Table of Contents

- [Overview](#overview)
- [Enabling No-Auth Mode](#enabling-no-auth-mode)
- [Startup Warning](#startup-warning)
- [Policy Input](#policy-input)
- [Policy Patterns](#policy-patterns)
  - [Method Filtering](#method-filtering)
  - [Shared-Secret Header](#shared-secret-header)
- [Mutual Exclusion](#mutual-exclusion)
- [Security Trade-offs and Compensating Controls](#security-trade-offs-and-compensating-controls)

## Overview

In a typical deployment, rest-rego validates a credential (JWT, Azure Graph token, or htpasswd) before the policy runs. No-auth mode removes that step entirely:

```
Normal mode                            No-auth mode (NO_AUTH=true)
──────────────────────────────         ──────────────────────────────────
Request → Auth validation              Request → Policy → Backend
              ↓                                 ↑
          Policy → Backend             input.jwt  == null
                                       input.user == null
```

**Deny-by-default is preserved.** Without an explicit `allow := true` rule in the policy the request is rejected with `403 Forbidden`, exactly as in any other mode.

## Enabling No-Auth Mode

```bash
# Environment variable
export NO_AUTH=true
rest-rego

# Command-line flag
rest-rego --no-auth
```

`NO_AUTH` defaults to `false`. There is no implicit activation — misconfiguration alone cannot enable no-auth mode.

## Startup Warning

When no-auth mode is active, rest-rego logs a warning at startup:

```
WARN noauth: no-auth mode enabled — policy is the sole access control
```

This warning appears in structured JSON logs and in Kubernetes pod logs, making it visible in log-aggregation systems.

## Policy Input

The policy input structure is identical to other auth modes, except that the authentication fields are `null`:

```jsonc
{
  "request": {
    "method": "GET",
    "path": ["api", "resource"],
    "headers": {
      "X-Api-Key": "secret",
      "Content-Type": "application/json"
    },
    "auth": null,   // null or omitted — no credential was presented
    "size": 0
  },
  "jwt":  null,     // null or omitted — no token was validated
  "user": null      // null or omitted — no identity was resolved
}
```

Because `input.jwt` and `input.user` are `null`, any policy rule that dereferences those fields (e.g., `input.jwt.roles`) evaluates to `undefined` and does not grant access.

## Policy Patterns

### Method Filtering

Allow read-only HTTP methods unconditionally while denying all mutating methods:

```rego
package policies

import rego.v1

read_only_methods := ["GET", "HEAD", "OPTIONS"]

default allow := false

allow if {
    input.request.method in read_only_methods
}
```

### Shared-Secret Header

Gate mutating methods on a shared secret supplied in a request header. The value is substituted at deploy time:

```rego
package policies

import rego.v1

default allow := false

read_only_methods := ["GET", "HEAD", "OPTIONS"]

allow if {
    input.request.method in read_only_methods
}

allow if {
    not input.request.method in read_only_methods
    input.request.headers["X-Api-Key"] == "$(EXPECTED_API_KEY)"
}
```

`$(EXPECTED_API_KEY)` is automatically expanded to the value of the `EXPECTED_API_KEY` environment variable when the policy is loaded — set the variable in the container environment and rest-rego substitutes it at load time. See [ENV-VARS.md](ENV-VARS.md) for details and [examples/no-auth/](../examples/no-auth/) for a complete working example.

## Mutual Exclusion

`PERMISSIVE_AUTH=true` combined with `NO_AUTH=true` also causes a startup error — permissive mode has no meaning when there is no token to validate.

## Security Trade-offs and Compensating Controls

No-auth mode shifts the entire trust decision to the Rego policy. Consider the following trade-offs and controls before enabling it in production:

| Trade-off                                        | Compensating Control                                                             |
|--------------------------------------------------|----------------------------------------------------------------------------------|
| No cryptographic identity verification           | Restrict inbound traffic with a Kubernetes `NetworkPolicy` to known sources only |
| Shared secrets in headers are visible in logs    | Redact sensitive headers via the `url` rule or upstream log filtering            |
| Policy bugs directly grant access                | Enable `DEBUG=true` in staging to inspect `input` for every request              |
| No `WWW-Authenticate` challenge is sent on `403` | Document the expected header in your API contract                                |

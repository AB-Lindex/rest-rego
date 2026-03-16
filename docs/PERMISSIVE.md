# Permissive Authentication Mode

Permissive mode (`PERMISSIVE_AUTH=true`) allows requests that fail authentication to pass through as anonymous rather than being rejected with `401 Unauthorized`. This is useful for migration scenarios, gradual rollouts, or APIs that serve both authenticated and unauthenticated clients.

## Table of Contents

- [Overview](#overview)
- [Configuration](#configuration)
- [Behavior by Auth Provider](#behavior-by-auth-provider)
- [Policy Input for Anonymous Requests](#policy-input-for-anonymous-requests)
- [Detecting Anonymous Requests in the Backend](#detecting-anonymous-requests-in-the-backend)
- [Use Cases](#use-cases)
- [Security Considerations](#security-considerations)

## Overview

In strict mode (default), any request with missing or invalid credentials is immediately rejected by rest-rego before reaching the policy engine or the backend. In permissive mode, those requests are instead forwarded to the Rego policy with an empty authentication context. The policy then decides whether to allow or deny the request.

```
PERMISSIVE_AUTH=false (default)           PERMISSIVE_AUTH=true
─────────────────────────────             ──────────────────────────────
Request → Auth → [401 if invalid]         Request → Auth → Policy → Backend
                      ↓                                        ↑
                 Policy → Backend              anonymous input passed through
```

**The policy is always the final gatekeeper.** Permissive mode only changes what happens before the policy runs — it does not bypass the policy.

## Configuration

```bash
export PERMISSIVE_AUTH=true
rest-rego
```

`PERMISSIVE_AUTH` is a single flag shared by all authentication providers. It is only meaningful when an auth provider is configured (`WELLKNOWN_OIDC`, `AZURE_TENANT`, or `BASIC_AUTH_FILE`).

A warning is logged at startup when permissive mode is enabled:

```
WARN config: permissive authentication mode enabled - invalid tokens will be treated as anonymous
```

## Behavior by Auth Provider

| Auth Provider | Credential situation | Strict mode (`false`) | Permissive mode (`true`) |
|---|---|---|---|
| **JWT** | No `Authorization` header | `null` auth, passes to policy | `null` auth, passes to policy |
| **JWT** | Invalid / expired token | `401 Unauthorized` | `null` auth, passes to policy |
| **Azure Graph** | No `Authorization` header | `null` auth, passes to policy | `null` auth, passes to policy |
| **Azure Graph** | Token for unknown app | `401 Unauthorized` | `null` auth, passes to policy |
| **Basic Auth** | No `Authorization` header | `null` auth, passes to policy | `null` auth, passes to policy |
| **Basic Auth** | Unknown username | `401 Unauthorized` | `null` auth, passes to policy |
| **Basic Auth** | Wrong password | `401 Unauthorized` | `401 Unauthorized` |

**Wrong passwords always return `401 Unauthorized` regardless of permissive mode.** This prevents credential-stuffing attacks from silently downgrading an authenticated session to anonymous access.

## Policy Input for Anonymous Requests

When a request is anonymous (no credentials, or credentials discarded in permissive mode), all authentication-related fields in the policy input are absent or `null`:

| Field | Authenticated | Anonymous |
|---|---|---|
| `input.request.auth` | `{"kind": "...", "user": "..."}` | `null` |
| `input.jwt` | JWT claims object | absent |
| `input.user` | Azure app object | absent |

A minimal Rego check to detect an anonymous request:

```rego
is_anonymous if {
    input.request.auth == null
}
```

For JWT and Azure modes `input.request.auth` is `null` when no valid token was presented. You can also check for the absence of the provider-specific top-level field:

```rego
# JWT mode
is_anonymous if { not input.jwt }

# Azure Graph mode
is_anonymous if { not input.user }
```

## Detecting Anonymous Requests in the Backend

Rego policy results are forwarded to the backend as `X-Restrego-*` headers. Any named variable in the policy (other than `allow` and `url`) is converted to a header:

| Policy variable | Backend header |
|---|---|
| `is_anonymous := true` | `X-Restrego-Is-Anonymous: true` |
| `caller := "alice"` | `X-Restrego-Caller: alice` |

Use this to signal the authentication state to the backend service:

```rego
package request.rego

import rego.v1

default allow := false

# Allow both authenticated and anonymous requests
allow if {
    input.request.auth.kind == "basic"
    input.request.auth.user != ""
}
allow if {
    input.request.auth == null
}

# Tell the backend who is calling (empty string for anonymous)
caller := input.request.auth.user if {
    input.request.auth != null
}
caller := "" if {
    input.request.auth == null
}

# Signal anonymous access explicitly
is_anonymous := "true" if {
    input.request.auth == null
}
```

The backend receives:

- Authenticated request: `X-Restrego-Caller: alice`, no `X-Restrego-Is-Anonymous` header
- Anonymous request: `X-Restrego-Caller:` (empty), `X-Restrego-Is-Anonymous: true`

The same pattern works for JWT and Azure Graph modes, substituting `input.request.auth == null` with `not input.jwt` or `not input.user` as appropriate.

### Complete JWT Example

```rego
package request.rego

import rego.v1

default allow := false

# Authenticated: specific app IDs only
allow if {
    input.jwt.appid in {
        "11112222-3333-4444-5555-666677778888",
    }
}

# Anonymous: public read-only paths only
allow if {
    not input.jwt
    input.request.method == "GET"
    input.request.path[0] in {"public", "health"}
}

# Forward app ID to backend (empty for anonymous)
appid := input.jwt.appid if { input.jwt }
appid := "" if { not input.jwt }

# Signal anonymous access
is_anonymous := "true" if { not input.jwt }
```

Backend headers for an anonymous GET to `/public/docs`:

```
X-Restrego-Appid:
X-Restrego-Is-Anonymous: true
```

## Use Cases

| Scenario | Description |
|---|---|
| **Zero-downtime migration** | Enable permissive mode when rolling out authentication to an existing API. Anonymous requests pass through while authenticated ones are validated. The policy can rate-limit or restrict anonymous access. |
| **Public + private endpoints** | Allow unauthenticated access to public paths while requiring authentication for protected paths, all within a single policy. |
| **Gradual rollout** | Deploy permissive mode first to observe which clients are authenticating (`PERMISSIVE_AUTH=true` with logging in the policy), then switch to strict mode once all clients comply. |
| **Backend feature flags** | The backend reads `X-Restrego-Is-Anonymous` to apply different business logic (e.g., lower rate limits, read-only mode) for anonymous callers. |

## Security Considerations

- **Policies must handle null auth explicitly.** A policy with `allow if { input.jwt.appid != "" }` will simply evaluate to `false` for anonymous requests — it will not error or allow. Verify that your `default allow := false` covers unauthenticated traffic.
- **Wrong passwords are never permissive.** A request with a recognized username but wrong password always returns `401 Unauthorized`. This is by design.
- **X-Restrego-* headers are spoofing-protected.** Incoming `X-Restrego-*` headers from clients are stripped by rest-rego before processing, so a client cannot forge `X-Restrego-Is-Anonymous`.
- **Log at startup.** The startup warning makes it easy to detect accidental permissive deployments in log monitoring.
- **Prefer strict mode in production.** Use permissive mode only during controlled migration windows or for explicitly mixed-access APIs. Treat it as a transitional state rather than a permanent configuration.

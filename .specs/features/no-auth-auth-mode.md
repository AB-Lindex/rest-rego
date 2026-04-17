---
type: "feature"
feature: "no-auth-policy-only-mode"
repository_type: "single-product"
status: "active"
priority: "medium"
complexity: "simple"
technology_stack: ["go", "opa-rego"]
azure_services: []
external_services: []
on_premises_dependencies: []
multi_instance_support: "compatible"
observability: "basic"
related_prd: "PRD.md"
cross_product_dependencies: []
---

# Feature: No-Auth Policy-Only Mode

## Problem Statement

rest-rego currently requires exactly one authentication provider (Azure Graph, JWT/OIDC, or Basic Auth) to be configured — startup fails hard if none is present. This blocks a legitimate class of use cases where identity is irrelevant and the policy alone is sufficient to control access:

1. **Method filtering** — block all mutating methods (`POST`, `PUT`, `DELETE`) at the proxy layer without an identity provider
2. **Shared-secret header enforcement** — gate access on a static API key header (`X-Api-Key`) without a full auth stack
3. **Internal service meshes** — the calling service is already trusted at the network level; only the request shape needs validation
4. **Policy prototyping** — develop and iterate on Rego policies without standing up an OIDC provider or Azure tenant

The product must remain **secure by default**: enabling no-auth mode should require an explicit opt-in flag, produce a prominent startup warning, and deny all requests unless the policy explicitly allows them (existing deny-by-default model is preserved).

## Goals

- Introduce an explicit `--no-auth` / `NO_AUTH=true` flag that bypasses the authentication step entirely
- Pass all requests directly to the Rego policy engine with `input.jwt` and `input.user` as `null`
- Require explicit opt-in — omitting all auth configuration continues to fail at startup (no silent change in behaviour)
- Emit a prominent startup warning when no-auth mode is active
- Provide an example policy demonstrating method filtering and header-value checking

## Non-Goals

- Removing the requirement to have a policy — a no-auth deployment without a policy file is still a misconfiguration
- Disabling the deny-by-default model — `allow` still defaults to `false`
- Supporting partial-auth (no-auth on some paths, auth on others) — that is a policy concern, not a config concern
- Permissive-auth (`PERMISSIVE_AUTH`) overlap — no-auth replaces auth entirely; permissive-auth still validates tokens when present

---

## User Stories

### US-001: Method Filtering Without an Identity Provider

**As an** operator of an internal service  
**I want** to run rest-rego with only a policy — no authentication provider  
**So that** I can enforce HTTP method restrictions (e.g. read-only access) at the proxy layer without deploying an OIDC stack

**Acceptance Criteria**:
- Setting `NO_AUTH=true` (or `--no-auth`) starts rest-rego without any auth provider
- A `WARN`-level log entry is emitted at startup: `no-auth mode enabled — policy is the sole access control`
- All requests reach the Rego policy with `input.jwt == null` and `input.user == null`
- A policy that allows only `GET` and `HEAD` blocks all other methods with `403`
- The existing deny-by-default remains: a policy that defines no `allow` rule denies everything

### US-002: Static Header Value Enforcement

**As an** operator  
**I want** to require a specific HTTP header value on all mutating requests  
**So that** I can add a lightweight shared-secret gate without an identity provider

**Acceptance Criteria**:
- The Rego policy can read `input.request.headers["X-Api-Key"]` and compare it to an expected value
- The expected value can be injected via env-var expansion in the policy (e.g. `$(API_KEY)`)
- A missing or wrong header value results in `403` (policy denies)
- A correct header value results in the request being forwarded to the backend

### US-003: Explicit Opt-In and Mutual Exclusion

**As a** developer maintaining the deployment configuration  
**I want** `NO_AUTH` to be mutually exclusive with all other auth providers (including the PERMISSIVE_AUTH mode)
**So that** a misconfigured deployment (e.g. both `NO_AUTH=true` and `WELLKNOWN_OIDC` set) fails at startup rather than silently ignoring one setting

**Acceptance Criteria**:
- Configuring `NO_AUTH=true` together with any of `AZURE_TENANT`, `WELLKNOWN_OIDC`, or `BASIC_AUTH_FILE` exits at startup with an error
- Configuring no auth provider at all (and `NO_AUTH` not set) continues to exit at startup with an error (existing behaviour unchanged)

---

## Requirements

### Functional

1. **New config flag** — `--no-auth` CLI flag and `NO_AUTH` environment variable, boolean, default `false`
2. **Mutual exclusion** — startup validation rejects any combination of `NO_AUTH` with other auth flags
3. **Pass-through auth provider** — `internal/noauth` package exposing a named `Provider` struct constructed via `noauth.New(permissive bool)`, implementing `types.AuthProvider`; `Authenticate` always returns `nil` (success/anonymous)
4. **Startup warning** — `slog.Warn` at startup when no-auth mode is active
5. **Example policy** — a documented sample policy showing method filtering and header-value checking patterns

### Non-Functional

- **Performance**: No measurable overhead compared to other auth providers — the no-op provider performs no I/O or computation
- **Security**:
  - Opt-in only — no implicit activation
  - Deny-by-default preserved — policy must explicitly allow
  - Startup warning surfaces in structured logs and Kubernetes pod logs
  - Documented clearly as a reduced-security configuration requiring compensating controls (network policy, Kubernetes NetworkPolicy)
- **Multi-instance**: Stateless; fully compatible with multiple replicas
- **Observability**: No additional metrics needed — existing `403` counters in Prometheus cover the deny path

---

## Technical Design

- **New package**: `internal/noauth` — single file `noauth.go`
  - Exports a named `NoAuthProvider` struct that implements `types.AuthProvider`
  - `func New(permissive bool) *NoAuthProvider` — constructor, consistent with other auth providers (`azure.New`, `jwtsupport.New`, `basicauth.New`)
    - If `permissive == true`: logs an error and returns `nil` (caller treats `nil` as a fatal initialisation failure)
    - If `permissive == false`: logs the startup `WARN` and returns the provider
  - `func (b *NoAuthProvider) Authenticate(info *types.Info, _ *http.Request) error` — always returns `nil`; leaves `info.JWT` and `info.User` unset
  - No additional methods; does **not** implement `types.AuthChallenger` (no `WWW-Authenticate` header needed)
- **Config change**: Add `NoAuth bool` field to `internal/config/config.Fields` with arg tag `--no-auth,env:NO_AUTH`
- **Validation change** in `config.New()`:
  - Increment `authCount` when `NoAuth == true`
  - Existing `authCount > 1` check covers mutual exclusion; error message: `config: only one auth-provider may be configured (AZURE_TENANT, WELLKNOWN_OIDC, BASIC_AUTH_FILE) or using NO_AUTH mode`
- **App wiring** in `internal/application/app.go`:
  - Add `case app.config.NoAuth:` branch before the `default` error branch
  - Branch body calls `noauth.New(app.config.PermissiveAuth)` and checks for `nil`
  - The startup `WARN` is emitted inside `noauth.New()`, not in `app.go`
- **Example policy**: `examples/no-auth/request.rego` demonstrating method filtering and `X-Api-Key` header check, using `import rego.v1` and set syntax

### Policy Input in No-Auth Mode

The `input` object available to the Rego policy is identical to authenticated mode:

| Field | Value |
|-------|-------|
| `input.request.method` | HTTP method (`GET`, `POST`, …) |
| `input.request.path` | Path segments as array |
| `input.request.headers` | All request headers |
| `input.request.size` | Content-Length |
| `input.jwt` | always `null` |
| `input.user` | always `null` |

### Example Policy Patterns

**Method filtering (read-only)**:
```rego
package policies

import rego.v1

read_only_methods := {"GET", "HEAD", "OPTIONS"}

default allow := false

allow if {
    input.request.method in read_only_methods
}
```

**Shared-secret header** (with env-var expansion):
```rego
package policies

import rego.v1

read_only_methods := {"GET", "HEAD", "OPTIONS"}

default allow := false

allow if {
    input.request.method in read_only_methods
}

allow if {
    not input.request.method in read_only_methods
    input.request.headers["X-Api-Key"] == "$(EXPECTED_API_KEY)"
}
```

---

## Implementation Phases

1. **MVP**: Config flag, pass-through provider, startup warning, mutual-exclusion validation, example policy
2. **Enhancement** *(out of scope for this feature)*: Per-path no-auth exemptions — better handled as a policy pattern

---

## Integration

- **PRD Link**: Extends section 1.2 "Pluggable authentication providers" — adds a fourth provider type (no-op)
- **README Impact**: New row in the authentication method table; new example link
- **Docs**: New `docs/NO-AUTH.md` documenting the mode, its trade-offs, and recommended compensating controls
- **Examples**: `examples/no-auth/` with `request.rego` and `README.md`
- **Cross-Product**: None

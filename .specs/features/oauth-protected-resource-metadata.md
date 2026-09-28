---
type: "feature"
feature: "oauth-protected-resource-metadata"
repository_type: "single-product"
status: "proposed"
priority: "medium"
complexity: "medium"
technology_stack: ["go", "opa-rego"]
azure_services: ["azure-ad"]
external_services: ["oidc-providers", "mcp-clients"]
on_premises_dependencies: []
multi_instance_support: "compatible"
observability: "basic"
related_prd: "PRD.md"
cross_product_dependencies: []
---

# Feature: OAuth 2.0 Protected Resource Metadata (RFC 9728)

## Problem Statement

MCP clients that implement the [MCP Authorization spec](https://modelcontextprotocol.io/specification/draft/basic/authorization)
(e.g. VS Code's built-in MCP client) expect a protected HTTP resource to advertise
*which* OAuth 2.0 authorization server issues tokens for it, per
[RFC 9728 (OAuth 2.0 Protected Resource Metadata)](https://www.rfc-editor.org/rfc/rfc9728).
Concretely, on an unauthenticated/denied request the client expects a
`401 Unauthorized` response carrying:

```
WWW-Authenticate: Bearer resource_metadata="https://<resource-origin>/.well-known/oauth-protected-resource"
```

rest-rego today has no such endpoint, and — more importantly — never returns
`401` for the specific case an MCP client probes first: a request with **no**
`Authorization` header at all. In JWT (OIDC) mode, `Authenticate()` treats a
missing auth header as anonymous and returns `nil` unconditionally
(`internal/jwtsupport/jwt.go`, "Case 1"), deferring the allow/deny decision
entirely to the Rego policy. A deny-by-default policy then rejects the
request with a flat `403 access denied` and no `WWW-Authenticate` header at
all (confirmed manually against a running instance — see
`mcp-test-go/SETUP.md` §7a in the `mcp-test-go` repo: *"no bearer token →
403"*).

Observed symptom (`mcp-test-go` project, rest-rego fronting an MCP server):
VS Code shows *"Dynamic Client Registration not supported — The
authorization server 'http://localhost:8181/' does not support automatic
client registration"*. VS Code has concluded that rest-rego's own origin
*is* the authorization server, because nothing told it otherwise — there is
no protected-resource metadata pointing at the real IdP (e.g. Entra ID), and
no `401` challenge to trigger discovery in the first place.

This is not just a missing-endpoint problem — it also means rest-rego
currently conflates two distinct HTTP semantics:

- **401 Unauthorized** — no valid credentials were presented; the caller
  should (re-)authenticate.
- **403 Forbidden** — credentials were presented and validated, but the
  policy denies the request; authenticating again will not help.

Today, *both* "anonymous request denied by policy" and "authenticated
request denied by policy" produce the same `403`, so there is no signal an
MCP (or any RFC 9728-aware) client can act on.

### Related precedent

A materially similar failure has already been documented against a
first-party Microsoft service in this workspace
(`mcp-servers/tests/devops-mcp-errors.md`, `mcp-servers/tests/mcp-issue.md`):
Entra ID does **not** serve `/.well-known/oauth-authorization-server/...`
(RFC 8414) — it 404s — only `/v2.0/.well-known/openid-configuration`. Any
`authorization_servers` value rest-rego advertises must be verified against
whatever discovery convention the connecting MCP client actually falls back
to; this is called out as an open risk below rather than assumed away.

Those same docs also show a second, independent Entra ID pitfall worth
carrying into this feature's acceptance criteria: the `resource` value
advertised by protected-resource metadata must exactly match a registered
`identifierUris` entry on the target app registration, or Entra ID rejects
token acquisition (`AADSTS9010010` / `AADSTS500011`).

## Goals

- Serve a spec-compliant `/.well-known/oauth-protected-resource` document
  (RFC 9728) on the main proxy listener (the same origin/port MCP/API
  clients call, i.e. `LISTEN_ADDR`, not `MGMT_ADDR`), advertising:
  - `resource` — the externally-reachable URL of the protected API
  - `authorization_servers` — the issuer(s) rest-rego already validates
    tokens against (derived from `WELLKNOWN_OIDC` / JWKS configuration)
  - `bearer_methods_supported: ["header"]`
- On a request with **no** credentials that the policy subsequently denies,
  return `401 Unauthorized` with a `WWW-Authenticate` header whose
  `resource_metadata` value is derived from `RESOURCE_URL` per the RFC
  9728/8414 well-known URI convention (well-known segment inserted before
  any path component of `RESOURCE_URL`, e.g. `RESOURCE_URL=https://host/mcp`
  → `resource_metadata="https://host/.well-known/oauth-protected-resource/mcp"`;
  see "`RESOURCE_URL` may include a path" below), instead of today's bare
  `403`.
- Preserve `403 Forbidden` (no `WWW-Authenticate`) for requests that
  presented credentials (valid or invalid) but were denied by policy —
  re-authentication would not change the outcome.
- Preserve today's "anonymous requests can be explicitly allowed by policy"
  model (e.g. public-path patterns in `docs/POLICY.md`) — this feature only
  changes the response for requests that are **denied**, never for requests
  the policy **allows**.
- Make the advertised `resource` value explicitly configurable (new env
  var), since rest-rego cannot reliably infer its own externally-reachable
  URL (it may sit behind a different host/port than `LISTEN_ADDR`, e.g.
  behind an ingress, port-forward, or Docker port mapping — as in the
  `mcp-test-go` local dev setup, `localhost:8181` mapped from a container).
- Make the well-known metadata path itself configurable (new env var,
  empty value keeps the RFC 9728 default `/.well-known/oauth-protected-resource`),
  since some ingresses/operators may prefer to route on a different fixed
  path rather than rewrite the RFC default — see "`RESOURCE_URL` may
  include a path" below.

### `RESOURCE_URL` may include a path

`RESOURCE_URL` is not guaranteed to be a bare origin (`scheme://host[:port]`)
— it may also carry a path component, e.g. `https://gateway.example.com/mcp-server`,
because an ingress sitting in front of rest-rego commonly strips or rewrites
that path segment before the request ever reaches rest-rego (path-based
routing to a backend service is a routine ingress pattern). rest-rego itself
never sees that path on its own incoming requests, but the metadata document
and `WWW-Authenticate` challenge must still reflect the full
externally-visible URL, including the path:

- The `resource` field in the metadata document is `RESOURCE_URL` verbatim
  (path included, unmodified).
- The well-known discovery URL (used both as the metadata endpoint's own
  conceptual identity and as the `resource_metadata` value in
  `WWW-Authenticate`) follows the RFC 9728 / RFC 8414 well-known URI
  convention: the `/.well-known/oauth-protected-resource` segment is
  inserted **before** `RESOURCE_URL`'s path, not appended after it — e.g.
  `RESOURCE_URL=https://gateway.example.com/mcp-server` produces
  `https://gateway.example.com/.well-known/oauth-protected-resource/mcp-server`.
- rest-rego's own `chi.Mux` route registration is unaffected by this and
  stays at the plain root `/.well-known/oauth-protected-resource` (see
  Technical Design) — the ingress has already stripped `RESOURCE_URL`'s path
  by the time the request reaches rest-rego, so rest-rego only needs the
  path *value* (from config) to build the strings it returns, not to route
  its own handler.
- **Operator-side ingress/HTTPRoute requirement**: because of the above,
  the ingress (or Gateway API `HTTPRoute`) in front of rest-rego must match
  **two** distinct paths and treat them differently:
  - the original resource path (e.g. `/mcp-server`) — routed to rest-rego
    exactly as it is today, unaffected by this feature
  - the well-known discovery path with the appended suffix (e.g.
    `/.well-known/oauth-protected-resource/mcp-server`) — must be
    **rewritten** by the ingress to rest-rego's configured plain well-known
    path (suffix stripped) before being forwarded to rest-rego, since
    rest-rego only ever answers on that plain path and has no knowledge of
    the suffix
  - this dual-path/rewrite rule is a deployment-time requirement rest-rego
    cannot enforce or implement itself; it must be documented (e.g. in
    `docs/DEPLOYMENT.md` and/or a Kubernetes `Ingress`/`HTTPRoute` example
    under `examples/kubernetes/`) as a required companion change whenever
    `RESOURCE_URL` carries a path
- **Configurable well-known path**: the plain path rest-rego listens on —
  and the path used as the rewrite target above — is itself a new env var
  (e.g. `RESOURCE_METADATA_PATH`), defaulting to
  `/.well-known/oauth-protected-resource` (RFC 9728 default) when unset or
  empty. This is useful when an operator's ingress cannot rewrite paths and
  instead needs rest-rego to answer on whatever fixed path the ingress
  already routes to it.

## Non-Goals

- **Dynamic Client Registration (RFC 7591)** — rest-rego is a resource
  server, not an authorization server; it cannot register OAuth clients on
  Entra ID's (or any IdP's) behalf. Clients without DCR support are expected
  to fall back to a manually-configured client ID against the real IdP
  (already-supported UX in VS Code, once it discovers the *real* IdP via
  this feature instead of assuming rest-rego itself is the AS).
- **Acting as an authorization server** — no `/authorize` or `/token`
  endpoints are added; the advertised `authorization_servers` always point
  at the actual upstream IdP (e.g. Entra ID), which serves those endpoints
  itself.
- **Azure Graph / Basic Auth / No-Auth modes** — this feature applies only
  to JWT (OIDC) mode, which is the only mode with a well-known issuer to
  advertise. Other auth modes continue to return `403`/`401` exactly as
  today; no protected-resource metadata is served if JWT mode is not
  configured.
- **Changing the `PERMISSIVE_AUTH` behavior** — permissive mode continues to
  treat invalid/missing credentials as anonymous and defers to policy,
  unaffected by this feature's status-code distinction (see Open
  Questions).

---

## User Stories

### US-001: MCP client discovers the real authorization server

**As a** developer connecting an MCP-aware client (e.g. VS Code) to an
MCP/API server fronted by rest-rego
**I want** rest-rego to advertise the real IdP as the authorization server
**So that** my client can perform the standard OAuth flow against that IdP
instead of assuming rest-rego is the authorization server and failing with
a "Dynamic Client Registration not supported" dead end

**Acceptance Criteria**:
- `GET /.well-known/oauth-protected-resource` on the proxy port returns
  `200` with a JSON body containing `resource` and a non-empty
  `authorization_servers` array, when JWT (OIDC) auth mode is configured
- The endpoint returns `404` (or is not registered at all) when rest-rego is
  not running in JWT (OIDC) auth mode
- An unauthenticated request to any policy-protected path that the policy
  denies returns `401` with a `WWW-Authenticate` header whose
  `resource_metadata` parameter is a fully-qualified URL to the above
  endpoint

### US-002: Authenticated-but-denied requests are unaffected

**As an** operator relying on today's policy-driven authorization
**I want** requests that already carried valid or invalid credentials and
were denied by policy to keep returning `403`
**So that** existing clients/policies that depend on today's `403` semantics
for "authenticated, not authorized" are not broken

**Acceptance Criteria**:
- A request with a valid JWT that the policy denies still returns `403`,
  with no `WWW-Authenticate` header
- A request with a malformed/expired JWT still returns the existing
  `401 invalid credentials` behavior (`types.ErrAuthenticationFailed` path),
  unchanged by this feature
- A policy that explicitly allows anonymous requests (e.g. a public path)
  continues to return `200`/proxied response — this feature only changes
  behavior on the deny path

### US-003: Operators control the advertised resource identifier

**As a** platform engineer deploying rest-rego behind port-forwarding,
Docker port mapping, or an ingress
**I want** to explicitly configure the externally-reachable resource URL
**So that** the advertised metadata is correct regardless of what
`LISTEN_ADDR` rest-rego binds to internally

**Acceptance Criteria**:
- A new config field (e.g. `RESOURCE_URL` / `--resource-url`) sets the
  `resource` value in the metadata document
- Startup fails clearly (or the metadata endpoint is disabled with a
  warning) if JWT mode is configured but `RESOURCE_URL` is not set —
  no guessing from `Host` headers, which are attacker-controllable and
  inconsistent across proxies/tunnels
- `RESOURCE_URL` may include a path component; the `resource_metadata`
  value advertised in `WWW-Authenticate` inserts the well-known segment
  before that path (RFC 9728/8414 convention), while rest-rego's own
  internal route stays at the plain root well-known path
- A second new config field (e.g. `RESOURCE_METADATA_PATH` /
  `--resource-metadata-path`) overrides the plain well-known path rest-rego
  listens on and advertises; an empty value keeps the RFC 9728 default
  `/.well-known/oauth-protected-resource`

---

## Requirements

### Functional

1. **New metadata endpoint** — `GET /.well-known/oauth-protected-resource`
   registered on the main proxy `chi.Mux` (not the management mux), so it is
   reachable at the same origin MCP/API clients already use. This internal
   route is always the plain configured path (`RESOURCE_METADATA_PATH`,
   defaulting to the RFC 9728 default), regardless of whether `RESOURCE_URL`
   carries a path component — see "`RESOURCE_URL` may include a path" above;
   the ingress/`HTTPRoute` in front of rest-rego must match the
   path-suffixed well-known URL it advertises and **rewrite** it down to
   this plain configured path before forwarding
2. **Conditional registration** — only registered when the active
   `AuthProvider` is JWT (OIDC) mode *and* `RESOURCE_URL` is configured;
   otherwise absent (`404`)
3. **Configurable well-known path** — `RESOURCE_METADATA_PATH` env var /
   `--resource-metadata-path` flag overrides the plain path rest-rego
   registers its handler on (and that the metadata/challenge strings are
   built from); empty value (the default) resolves to the RFC 9728 default
   `/.well-known/oauth-protected-resource`
4. **Metadata contents** — minimum viable RFC 9728 document:
   ```json
   {
     "resource": "https://your-service.example.com",
     "authorization_servers": ["https://login.microsoftonline.com/<tenant>/v2.0"],
     "bearer_methods_supported": ["header"]
   }
   ```
   `authorization_servers` is derived from the configured `WELLKNOWN_OIDC`
   issuer(s) — see Technical Design for how the issuer is extracted
5. **New config field** — `RESOURCE_URL` env var / `--resource-url` flag,
   validated as an absolute `https://` or `http://` URL at startup when JWT
   mode + this feature are both active
6. **Status-code split on deny** — the policy-deny path
   (`internal/router/policy.go`) distinguishes:
   - `info.JWT == nil && info.User == nil` (no credentials were ever
     validated) → `401` + `WWW-Authenticate: Bearer resource_metadata="..."`
   - otherwise (credentials were presented, valid or invalid, but policy
     still denies) → `403` (unchanged)
7. **`WWW-Authenticate` value** — reuses the existing
   `types.AuthChallenger` interface (already used by
   `internal/router/auth.go` for the `401 invalid credentials` case) so both
   call sites share one source of truth for the challenge string

### Non-Functional

- **Performance**: metadata document is static per-process (computed once at
  startup from config); serving it must not add measurable latency to the
  hot request path
- **Security**:
  - No secrets in the metadata document — only public issuer/resource URLs
  - `RESOURCE_URL` must be explicitly configured, never inferred from the
    `Host` header (spoofable) or `X-Forwarded-*` headers
  - Advertising the wrong `authorization_servers` value is a
    misconfiguration risk (token acquisition failures at the IdP), not a
    rest-rego security bug — document this tradeoff clearly, see the Entra
    ID `AADSTS9010010` precedent above
- **Multi-instance**: stateless; identical metadata served by every replica
  behind a given `RESOURCE_URL`
- **Observability**: log the computed metadata document once at startup
  (`slog.Info`) so operators can verify it without a manual `curl`

---

## Technical Design

- **New package or file**: `internal/router/metadata.go` (proxy-side, new
  handler) plus a small config addition; no new top-level package needed
- **Config change**: add `ResourceURL string` and `ResourceMetadataPath
  string` to `internal/config/config.Fields`:
  - `arg:"--resource-url,env:RESOURCE_URL" help:"externally-reachable URL of this protected resource, required to serve RFC 9728 metadata in JWT mode"`
  - `arg:"--resource-metadata-path,env:RESOURCE_METADATA_PATH" default:"/.well-known/oauth-protected-resource" help:"path this instance listens on (and advertises) for RFC 9728 metadata; empty keeps the RFC default"`
- **Issuer extraction**: `JWTSupport` already loads each `WELLKNOWN_OIDC`
  document into `wellKnownData` (`internal/jwtsupport/jwt.go`). That struct
  currently only captures `jwks_uri` and
  `id_token_signing_alg_values_supported` — it needs to also capture
  `issuer` from the same OIDC discovery document, so
  `authorization_servers` can be populated without a new HTTP round-trip.
  Expose it via a new method, e.g. `func (j *JWTSupport) Issuers() []string`,
  so `router.New` can build the metadata document without `internal/router`
  reaching into `jwtsupport` internals directly (keep the `AuthProvider`
  interface boundary intact — see Open Questions)
- **Route registration**: register
  `proxy.mux.Get(config.ResourceMetadataPath, proxy.protectedResourceHandler)`
  in `router.New` (`internal/router/router.go`), conditionally, based on
  whether the constructed `AuthProvider` exposes issuers (mirrors the
  existing optional-interface pattern used for `types.AuthChallenger`). This
  route is always registered at the plain configured path (default
  `/.well-known/oauth-protected-resource`) — `RESOURCE_URL`'s path component
  (if any) is never used for routing, only for building the returned
  metadata/challenge strings (see below)
- **Deny-path status code**: `internal/router/policy.go`'s existing
  ```go
  if !allowBool {
      http.Error(w, "access denied", http.StatusForbidden)
      return
  }
  ```
  becomes conditional on `info.JWT == nil && info.User == nil`, emitting
  `401` + the `WWW-Authenticate` header (built the same way
  `internal/router/auth.go` already does, via `types.AuthChallenger`) in the
  no-credentials case
- **Challenge string**: extend the existing default
  `WWWAuthenticate() string` implementations (or the `"Bearer"` fallback in
  `internal/router/auth.go`) to append a `resource_metadata="..."` parameter
  built by inserting `config.ResourceMetadataPath` (default
  `/.well-known/oauth-protected-resource`) **before** the path component of
  `RESOURCE_URL` (not simply appending it after the full `RESOURCE_URL`
  string), per the RFC 9728/8414 well-known URI convention — e.g.
  `RESOURCE_URL=https://host/mcp` yields
  `resource_metadata="https://host/.well-known/oauth-protected-resource/mcp"`.
  A small helper (e.g. `wellKnownURL(resourceURL, metadataPath string) string`,
  using `net/url` to split scheme+authority from path) computes this once at
  startup so both the `401 invalid credentials` path and the new `401` deny
  path advertise the same discovery URL

---

## Open Questions

- **Multiple `WELLKNOWN_OIDC` issuers**: rest-rego supports comma-separated
  well-knowns (multi-tenant/multi-IdP). RFC 9728's
  `authorization_servers` is an array, so this maps cleanly — but has this
  been tested against a real MCP client with more than one entry? Needs
  manual verification, not assumed to "just work" client-side.
- **Interaction with `PERMISSIVE_AUTH`**: in permissive mode, invalid tokens
  are already treated as anonymous before reaching the policy. Should a
  permissive-mode "invalid token, treated as anonymous, denied by policy"
  request also get the new `401` treatment, or should permissive mode be
  exempted entirely? Needs a decision before implementation, not covered by
  this spec.
- **Client discovery fallback behavior is unverified**: the "related
  precedent" section above documents that Entra ID does not serve RFC 8414
  metadata at the conventional path. Whether VS Code's MCP client (or other
  RFC 9728-aware clients) falls back to plain OIDC discovery
  (`/.well-known/openid-configuration`) when RFC 8414 discovery 404s is
  **not yet confirmed** and should be verified end-to-end against a real
  client before considering this feature complete — serving correct
  metadata does not guarantee the overall flow succeeds.

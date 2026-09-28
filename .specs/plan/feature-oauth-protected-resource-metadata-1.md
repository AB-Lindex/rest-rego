---
goal: Implement OAuth 2.0 Protected Resource Metadata (RFC 9728) for MCP Auth Discovery
version: "1.0"
date_created: 2026-09-28
last_updated: 2026-09-28
owner: rest-rego team
status: 'Planned'
tags: [feature, auth, mcp, rfc9728, jwt]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

Implement RFC 9728 (OAuth 2.0 Protected Resource Metadata) support so that MCP-aware
clients (e.g. VS Code's MCP client) fronted by rest-rego in JWT (OIDC) auth mode can
discover the real upstream authorization server instead of assuming rest-rego itself is
the authorization server. This adds a new `/.well-known/oauth-protected-resource`
endpoint, a `401 Unauthorized` + `WWW-Authenticate: Bearer resource_metadata="..."`
response for **fully anonymous** requests denied by policy, and two new config fields
(`RESOURCE_URL`, `RESOURCE_METADATA_PATH`). Scope is limited to JWT (OIDC) auth mode;
all other auth modes and the `PERMISSIVE_AUTH` credential-downgrade path are unaffected.

## 1. Requirements & Constraints

- **REQ-001**: New route `GET <RESOURCE_METADATA_PATH>` (default
  `/.well-known/oauth-protected-resource`) registered on the proxy `chi.Mux`
  (`internal/router/router.go`), never on the management mux
- **REQ-002**: The route is registered **only** when the active `AuthProvider` is JWT
  (OIDC) mode **and** `RESOURCE_URL` is configured; otherwise it is not registered at
  all (`404` for that path) — no hard startup failure (see CON-004)
- **REQ-003**: Metadata document body:
  ```json
  {
    "resource": "<RESOURCE_URL, verbatim, path included>",
    "authorization_servers": ["<issuer 1>", "<issuer 2>", "..."],
    "bearer_methods_supported": ["header"]
  }
  ```
  `authorization_servers` is the de-duplicated list of `issuer` values extracted from
  every configured `WELLKNOWN_OIDC` OIDC-discovery document
- **REQ-004**: New config field `RESOURCE_URL` (env `RESOURCE_URL`, flag
  `--resource-url`) — absolute URL, may include a path component
- **REQ-005**: New config field `RESOURCE_METADATA_PATH` (env
  `RESOURCE_METADATA_PATH`, flag `--resource-metadata-path`) — defaults to
  `/.well-known/oauth-protected-resource`; an explicitly-empty value also resolves to
  that default
- **REQ-006**: The well-known discovery URL used both as the route's own identity and
  as the `resource_metadata` value in `WWW-Authenticate` is built by inserting
  `RESOURCE_METADATA_PATH` **before** the path component of `RESOURCE_URL` (RFC
  9728/8414 convention), e.g. `RESOURCE_URL=https://host/mcp` →
  `https://host/.well-known/oauth-protected-resource/mcp`. rest-rego's own route
  registration always stays at the plain `RESOURCE_METADATA_PATH` — the path suffix is
  never used for routing, only for the strings returned to callers
- **REQ-007**: Deny-path status-code split in `internal/router/policy.go`:
  - Request had **no** `Authorization` header at all (`info.Request.Auth == nil`) →
    `401` + `WWW-Authenticate: Bearer resource_metadata="<well-known URL>"`
  - Otherwise (credentials were presented — valid, invalid-but-permissive-downgraded,
    or wrong-kind) → `403`, unchanged, no `WWW-Authenticate` header
- **REQ-008**: The `401 invalid credentials` path in `internal/router/auth.go` and the
  new `401` deny path in `policy.go` must share one challenge-string builder — no
  duplicated URL-construction logic
- **REQ-009**: `JWTSupport` exposes issuers via a new method so `internal/router` never
  reaches into `internal/jwtsupport` internals directly
- **SEC-001**: `RESOURCE_URL` is never inferred from the `Host` header or any
  `X-Forwarded-*` header (both attacker-controllable) — configuration only
- **SEC-002**: No secrets appear in the metadata document — issuer and resource URLs
  only
- **CON-001**: Feature applies only to JWT (OIDC) auth mode (`internal/jwtsupport`).
  Azure, Basic Auth, and No-Auth modes are unaffected — no metadata route, no `401`
  status-code split for them
- **CON-002**: `PERMISSIVE_AUTH` behavior is unaffected — a request whose invalid or
  wrong-kind token was downgraded to anonymous by permissive mode still returns `403`
  on policy deny, exactly as today. Only requests with **zero** `Authorization` header
  get the new `401` treatment (decision: exempt permissive-downgraded credentials)
- **CON-003**: `types.AuthProvider` interface stays backward-compatible — issuer
  exposure is an **optional** interface (mirrors the existing `types.AuthChallenger`
  pattern), not an addition to the base interface
- **CON-004**: If `RESOURCE_URL` is unset while JWT mode is active, do **not** fail
  startup — log `slog.Warn` and simply skip metadata-route registration (decision:
  warn-and-disable, not hard fail)
- **GUD-001**: Follow existing logging conventions (`log/slog`, structured key-value
  pairs, `component: message` style, e.g. `"router: ..."`, `"jwtsupport: ..."`)
- **GUD-002**: Follow the existing optional-interface pattern already used for
  `types.AuthChallenger` in `internal/router/auth.go`
- **PAT-001**: Compute the metadata JSON document and the challenge URL once at
  startup (in `router.New`) — never per-request

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD**: `/.specs/PRD.md`
- **Features**: [.specs/features/oauth-protected-resource-metadata.md](../features/oauth-protected-resource-metadata.md)
- **Technology Stack**: Go
- **Cross-Product Dependencies**: None

## 2. Implementation Steps

### Implementation Phase 1 — Configuration

- **GOAL-001**: Add and validate the two new config fields before any other phase
  depends on them.

- **TASK-001**: Add `ResourceURL` and `ResourceMetadataPath` fields to
  `internal/config/config.go` `Fields` struct `[📋 Planned]`
  - Files: `internal/config/config.go`
  - Action: insert after the `PermissiveAuth` field:
    ```go
    ResourceURL          string `arg:"--resource-url,env:RESOURCE_URL" help:"externally-reachable URL of this protected resource (RFC 9728); required for JWT mode to serve metadata" placeholder:"URL"`
    ResourceMetadataPath string `arg:"--resource-metadata-path,env:RESOURCE_METADATA_PATH" default:"/.well-known/oauth-protected-resource" help:"path this instance listens on (and advertises) for RFC 9728 metadata; empty keeps the RFC default" placeholder:"PATH"`
    ```

- **TASK-002**: Add `validateResourceMetadataConfig` validation to
  `internal/config/config.go`, called from `New()` `[📋 Planned]`
  - Files: `internal/config/config.go`
  - Action: add a new unexported method, called after the existing `authCount`
    block and before the final `return f`:
    ```go
    // validateResourceMetadata validates and normalizes RFC 9728 metadata config.
    func (f *Fields) validateResourceMetadata() {
        if f.ResourceMetadataPath == "" {
            f.ResourceMetadataPath = "/.well-known/oauth-protected-resource"
        }
        if f.ResourceURL == "" {
            if len(f.WellKnownURL) > 0 {
                slog.Warn("config: JWT (OIDC) mode active but resource-url is not set — RFC 9728 metadata endpoint disabled")
            }
            return
        }
        if len(f.WellKnownURL) == 0 {
            slog.Warn("config: resource-url is configured but JWT (OIDC) auth mode is not active — ignoring")
            return
        }
        u, err := url.Parse(f.ResourceURL)
        if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
            slog.Error("config: resource-url must be an absolute http(s) URL", "value", f.ResourceURL)
            os.Exit(1)
        }
    }
    ```
  - Dependencies: TASK-001. Requires adding `"net/url"` to the existing import block
    in `internal/config/config.go`
  - Call site: add `f.validateResourceMetadata()` in `New()` immediately after the
    existing `authCount > 1` / audiences validation block

- **TASK-003**: Add unit tests for `validateResourceMetadata` behavior `[📋 Planned]`
  - Files: `internal/config/config_test.go` (new or existing file — check for
    presence first)
  - Cases: empty `ResourceMetadataPath` → defaults; `ResourceURL` set without JWT mode
    → warns, does not exit; `ResourceURL` set with JWT mode and valid absolute URL →
    no exit; `ResourceURL` malformed/relative → `os.Exit(1)` path (test via extracted
    pure-validation helper returning `error` if `os.Exit` makes direct testing
    impractical — see TASK note below)
  - Note: if `os.Exit(1)` inside `validateResourceMetadata` is hard to unit-test
    directly (consistent with how `validateTimeouts`/`validateMetricLabels` are
    already structured in this file), follow the same existing pattern used for
    `validateMetricLabelsConfig` (pure function returning `error`) +
    `validateMetricLabels` (thin `os.Exit` wrapper) — split into
    `validateResourceMetadataConfig() error` and a thin wrapper, and test the pure
    function only

### Implementation Phase 2 — Issuer Extraction in `jwtsupport`

- **GOAL-002**: Capture the `issuer` field from each OIDC discovery document and
  expose the de-duplicated list without requiring an extra HTTP round-trip.

- **TASK-004**: Add `Issuer` field to `wellKnownData` struct `[📋 Planned]`
  - Files: `internal/jwtsupport/jwt.go`
  - Action: extend the struct (currently `JwksURI`, `SupportedAlgorithms`,
    `sourceURL`, `isLocalFile`):
    ```go
    type wellKnownData struct {
        JwksURI             string   `json:"jwks_uri"`
        Issuer              string   `json:"issuer"`
        SupportedAlgorithms []string `json:"id_token_signing_alg_values_supported"`
        sourceURL           string
        isLocalFile         bool
    }
    ```
  - No change needed to `LoadWellKnowns()` parsing logic itself — `json.Unmarshal`
    already populates any matching field automatically once it exists on the struct

- **TASK-005**: Add `Issuers() []string` method to `JWTSupport` `[📋 Planned]`
  - Files: `internal/jwtsupport/jwt.go`
  - Action: add after `New()` (or near `LoadWellKnowns`):
    ```go
    // Issuers returns the de-duplicated, non-empty issuer values collected from all
    // configured well-known (OIDC discovery) documents, in first-seen order.
    func (j *JWTSupport) Issuers() []string {
        seen := make(map[string]bool, len(j.wellknownList))
        var out []string
        for _, wc := range j.wellknownList {
            if wc.Issuer == "" || seen[wc.Issuer] {
                continue
            }
            seen[wc.Issuer] = true
            out = append(out, wc.Issuer)
        }
        return out
    }
    ```
  - Dependencies: TASK-004

- **TASK-006**: Add unit test for `Issuers()` `[📋 Planned]`
  - Files: `internal/jwtsupport/jwt_test.go` (existing test file — confirm location
    via `internal/jwtsupport` directory listing before editing)
  - Cases: no well-knowns loaded → empty slice; single issuer → one-element slice;
    duplicate issuers across two well-knowns → de-duplicated single entry; missing
    `issuer` field on one well-known → skipped, others still returned

### Implementation Phase 3 — Optional `IssuerProvider` Interface

- **GOAL-003**: Let `internal/router` detect issuer-capable auth providers without a
  direct dependency on `internal/jwtsupport`, mirroring the existing
  `types.AuthChallenger` pattern.

- **TASK-007**: Add `IssuerProvider` interface to `internal/types/types.go`
  `[📋 Planned]`
  - Files: `internal/types/types.go`
  - Action: add next to the existing `AuthChallenger` interface:
    ```go
    // IssuerProvider is optionally implemented by AuthProviders that can advertise
    // one or more OAuth 2.0 / OIDC issuer URLs for RFC 9728 protected-resource
    // metadata.
    type IssuerProvider interface {
        Issuers() []string
    }
    ```
  - Dependencies: TASK-005 (method signature must match)

### Implementation Phase 4 — Metadata Document, Handler, and Challenge Builder

- **GOAL-004**: Build the metadata document, the well-known URL helper, and the HTTP
  handler in a new file, computed once at startup.

- **TASK-008**: Create `internal/router/metadata.go` `[📋 Planned]`
  - Files: `internal/router/metadata.go` (new file)
  - Contents:
    - `protectedResourceMetadata` struct:
      ```go
      type protectedResourceMetadata struct {
          Resource                string   `json:"resource"`
          AuthorizationServers    []string `json:"authorization_servers"`
          BearerMethodsSupported  []string `json:"bearer_methods_supported"`
      }
      ```
    - `buildWellKnownURL(resourceURL, metadataPath string) (string, error)` — uses
      `net/url` to parse `resourceURL`, splits scheme+host from path, returns
      `scheme://host` + `metadataPath` + original path (path empty if root), per
      REQ-006. Returns an error for an unparseable `resourceURL` (should not happen
      post-validation, but keep the boundary explicit rather than panicking)
    - `newProtectedResourceMetadata(resourceURL string, issuers []string) *protectedResourceMetadata`
      — builds the struct with `BearerMethodsSupported: []string{"header"}`
    - `(proxy *Proxy) protectedResourceHandler(w http.ResponseWriter, r *http.Request)`
      — writes the pre-marshaled JSON body (see TASK-009) with
      `Content-Type: application/json`, `200 OK`
  - Dependencies: TASK-001 (config fields), TASK-007 (interface)

- **TASK-009**: Extend `Proxy` struct with precomputed metadata state
  `[📋 Planned]`
  - Files: `internal/router/types.go`
  - Action: add fields to `Proxy` struct:
    ```go
    resourceMetadataJSON []byte // pre-marshaled RFC 9728 document, nil if disabled
    resourceMetadataURL  string // well-known challenge URL, "" if disabled
    ```

- **TASK-010**: Add unit tests for `buildWellKnownURL` `[📋 Planned]`
  - Files: `internal/router/metadata_test.go` (new file)
  - Cases (from feature spec examples):
    - `RESOURCE_URL=https://host` + default path →
      `https://host/.well-known/oauth-protected-resource`
    - `RESOURCE_URL=https://host/mcp` + default path →
      `https://host/.well-known/oauth-protected-resource/mcp`
    - `RESOURCE_URL=https://gateway.example.com/mcp-server` + custom
      `RESOURCE_METADATA_PATH=/oauth/metadata` →
      `https://gateway.example.com/oauth/metadata/mcp-server`
    - `RESOURCE_URL` with trailing slash / metadataPath with/without leading slash —
      normalized consistently (no double slashes)

### Implementation Phase 5 — Route Registration in `router.New`

- **GOAL-005**: Wire the metadata endpoint into the proxy mux, conditionally, at
  construction time.

- **TASK-011**: Register the metadata route in `internal/router/router.go`
  `[📋 Planned]`
  - Files: `internal/router/router.go`
  - Action: in `New()`, after `proxy.auth = auth` is assigned and before `return
    proxy`, add:
    ```go
    if ip, ok := auth.(types.IssuerProvider); ok && cfg.ResourceURL != "" {
        issuers := ip.Issuers()
        if len(issuers) == 0 {
            slog.Warn("router: JWT mode active with resource-url set, but no issuers were discovered — RFC 9728 metadata endpoint disabled")
        } else {
            wellKnownURL, err := buildWellKnownURL(cfg.ResourceURL, cfg.ResourceMetadataPath)
            if err != nil {
                slog.Error("router: failed to build resource-metadata URL", "error", err)
            } else {
                meta := newProtectedResourceMetadata(cfg.ResourceURL, issuers)
                body, err := json.Marshal(meta)
                if err != nil {
                    slog.Error("router: failed to marshal protected-resource metadata", "error", err)
                } else {
                    proxy.resourceMetadataJSON = body
                    proxy.resourceMetadataURL = wellKnownURL
                    proxy.mux.Get(cfg.ResourceMetadataPath, proxy.protectedResourceHandler)
                    slog.Info("router: registered RFC 9728 protected-resource metadata endpoint",
                        "path", cfg.ResourceMetadataPath, "resource", cfg.ResourceURL, "authorization_servers", issuers)
                }
            }
        }
    }
    ```
  - Dependencies: TASK-007, TASK-008, TASK-009. Requires adding `"encoding/json"` to
    the existing import block in `router.go`
  - Note: this registration happens **before** `proxy.mux.Use(...)` middleware chain
    is attached to `/*` — confirm route registration order relative to the existing
    `proxy.mux.Handle("/*", proxy)` call so the well-known path is matched by chi
    ahead of the catch-all and does **not** pass through `authHandler`/
    `policyHandler` (the metadata document itself must be publicly readable,
    unauthenticated, per RFC 9728)

### Implementation Phase 6 — Deny-Path Status-Code Split and Shared Challenge Builder

- **GOAL-006**: Split `401`/`403` on policy deny, and share the challenge-string
  construction between `auth.go` and `policy.go`.

- **TASK-012**: Add a shared challenge-string helper to `internal/router/auth.go`
  `[📋 Planned]`
  - Files: `internal/router/auth.go`
  - Action: extract a small helper used by both the existing `401 invalid
    credentials` branch and the new deny-path branch:
    ```go
    // wwwAuthenticateChallenge returns the WWW-Authenticate header value for this
    // proxy's auth provider, appending resource_metadata when RFC 9728 metadata is
    // enabled.
    func (proxy *Proxy) wwwAuthenticateChallenge() string {
        challenge := "Bearer"
        if c, ok := proxy.auth.(types.AuthChallenger); ok {
            challenge = c.WWWAuthenticate()
        }
        if proxy.resourceMetadataURL != "" {
            challenge = fmt.Sprintf(`%s resource_metadata="%s"`, challenge, proxy.resourceMetadataURL)
        }
        return challenge
    }
    ```
  - Update the existing `errors.Is(err, types.ErrAuthenticationFailed)` branch in
    `authHandler` to call `proxy.wwwAuthenticateChallenge()` instead of its current
    inline `challenge :=` / type-assertion block
  - Requires adding `"fmt"` to the existing import block in `auth.go`

- **TASK-013**: Split `401`/`403` on policy deny in `internal/router/policy.go`
  `[📋 Planned]`
  - Files: `internal/router/policy.go`
  - Action: replace the existing:
    ```go
    if !allowBool {
        slog.Info("router: access denied by policy", ...)
        http.Error(w, "access denied", http.StatusForbidden)
        return
    }
    ```
    with:
    ```go
    if !allowBool {
        if info.Request.Auth == nil && proxy.resourceMetadataURL != "" {
            slog.Info("router: anonymous request denied by policy", "path", r.URL.Path, "method", r.Method, "id", info.Request.ID)
            w.Header().Set("WWW-Authenticate", proxy.wwwAuthenticateChallenge())
            http.Error(w, "authentication required", http.StatusUnauthorized)
            return
        }
        slog.Info("router: access denied by policy", "path", r.URL.Path, "method", r.Method, "id", info.Request.ID)
        http.Error(w, "access denied", http.StatusForbidden)
        return
    }
    ```
  - Dependencies: TASK-012. `info.Request.Auth == nil` is true only when the
    incoming request had no `Authorization` header at all (set in
    `types.NewInfo`) — this is what keeps `PERMISSIVE_AUTH`-downgraded requests
    (which had a header, just an invalid/wrong-kind one) on the `403` path per
    CON-002
  - Guard on `proxy.resourceMetadataURL != ""` ensures behavior is completely
    unchanged (still `403`) for Azure/Basic/No-Auth modes and for JWT mode without
    `RESOURCE_URL` configured

- **TASK-014**: Add/extend unit tests for the deny-path status-code split
  `[📋 Planned]`
  - Files: `internal/router/policy_test.go` (existing test file — confirm presence
    first)
  - Cases:
    - JWT mode + `RESOURCE_URL` set, no `Authorization` header, policy denies →
      `401` + `WWW-Authenticate` header present and contains `resource_metadata=`
    - JWT mode + `RESOURCE_URL` set, valid JWT, policy denies → `403`, no
      `WWW-Authenticate` header
    - JWT mode + `RESOURCE_URL` set, `PERMISSIVE_AUTH=true`, wrong-kind/invalid
      token downgraded to anonymous (`info.Request.Auth != nil`, `info.JWT ==
      nil`), policy denies → `403`, no `WWW-Authenticate` header
    - JWT mode, `RESOURCE_URL` **not** set, no `Authorization` header, policy
      denies → `403` (unchanged legacy behavior)
    - Azure / Basic Auth / No-Auth mode, no credentials, policy denies → `403`
      (unchanged legacy behavior)
    - A policy that explicitly allows the same anonymous request → `200`/proxied,
      confirming the allow path is untouched

- **TASK-015**: Add/extend unit test for `internal/router/auth.go`'s
  `401 invalid credentials` path with metadata enabled `[📋 Planned]`
  - Files: `internal/router/auth_test.go` (existing test file — confirm presence
    first)
  - Case: JWT mode + `RESOURCE_URL` set, malformed/expired token → `401`, existing
    `types.ErrAuthenticationFailed` behavior preserved, `WWW-Authenticate` header
    now additionally carries `resource_metadata=`

### Implementation Phase 7 — Documentation

- **GOAL-007**: Document the two new env vars, the new endpoint, the status-code
  split, and the ingress dual-path/rewrite requirement for `RESOURCE_URL` with a path
  component.

- **TASK-016**: Add `RESOURCE_URL` / `RESOURCE_METADATA_PATH` to
  `docs/CONFIGURATION.md` JWT section `[📋 Planned]`
  - Files: `docs/CONFIGURATION.md`
  - Action: add both new flags to the existing options table alongside
    `WELLKNOWN_OIDC` / `JWT_AUDIENCES` (around line 115), with defaults and a short
    description; add a short "RFC 9728 metadata" subsection explaining the new
    endpoint and the `401` vs `403` distinction

- **TASK-017**: Add a new `docs/MCP-AUTH.md` (or extend `docs/JWT.md`) explaining
  RFC 9728 support end-to-end `[📋 Planned]`
  - Files: `docs/JWT.md` or new `docs/MCP-AUTH.md` — decide based on existing doc
    length/scope conventions in `docs/`
  - Contents: example metadata document, example `WWW-Authenticate` header, the
    `RESOURCE_URL`-with-path ingress rewrite requirement (two distinct paths, one
    rewritten), and the Entra ID `identifierUris` / `AADSTS9010010` precedent from
    the feature spec

- **TASK-018**: Document the ingress dual-path/rewrite requirement in
  `docs/DEPLOYMENT.md` and add a Kubernetes example `[📋 Planned]`
  - Files: `docs/DEPLOYMENT.md`, `examples/kubernetes/ingress.yaml` (or a new
    `examples/kubernetes/mcp-auth/` folder mirroring the existing
    `examples/kubernetes/basic-auth/` and `examples/kubernetes/file-based-jwks/`
    pattern)
  - Content: an `Ingress` (or Gateway API `HTTPRoute`) example showing the original
    resource path routed unchanged, and the
    `/.well-known/oauth-protected-resource/<suffix>` path rewritten to the plain
    `RESOURCE_METADATA_PATH` before reaching rest-rego

### Implementation Phase 8 — Manual End-to-End Verification

- **GOAL-008**: Verify the feature against a real MCP client and a real IdP before
  considering the plan complete, per the feature spec's Open Questions.

- **TASK-019**: Verify VS Code MCP client discovery flow against a running rest-rego
  instance in JWT mode `[📋 Planned]`
  - Prerequisites: TASK-001 through TASK-018 complete; a reachable IdP (e.g. Entra
    ID) configured via `WELLKNOWN_OIDC`; `RESOURCE_URL` set to the externally-visible
    URL used by the MCP client
  - Steps: (1) confirm `GET <RESOURCE_METADATA_PATH>` returns `200` with expected
    JSON; (2) send an MCP client request with no `Authorization` header and confirm
    `401` + `WWW-Authenticate` with `resource_metadata`; (3) connect VS Code's MCP
    client and confirm it discovers the real IdP (no more "Dynamic Client
    Registration not supported" dead end); (4) confirm whether the client falls back
    to `/.well-known/openid-configuration` when RFC 8414 discovery 404s on Entra ID,
    per the feature spec's "Client discovery fallback behavior is unverified" open
    question
  - Testing: manual, not automatable in CI; record findings in a follow-up note
  - External dependency: reachable Entra ID (or other OIDC IdP) tenant, VS Code with
    MCP support enabled

- **TASK-020**: Verify multi-issuer (`WELLKNOWN_OIDC` comma-separated) metadata
  against a real MCP client `[📋 Planned]`
  - Prerequisites: TASK-019
  - Steps: configure two well-knowns, confirm `authorization_servers` contains both
    issuers, confirm the MCP client handles an array of more than one entry
    correctly (per the feature spec's "has this been tested" open question)
  - Testing: manual

**Status Tags:**
- `[✅ Completed: YYYY-MM-DD]` - Task finished
- `[⏳ In Progress]` - Currently being worked on
- `[📋 Planned]` - Not yet started
- `[⚠️ Blocked: reason]` - Cannot proceed due to dependency
- `[❌ Cancelled: reason]` - Task no longer needed

## 3. Alternatives

- **ALT-001**: Hard-fail startup (`os.Exit(1)`) when JWT mode is active without
  `RESOURCE_URL` — rejected per user decision (CON-004); would break existing JWT-mode
  deployments that upgrade rest-rego without immediately adopting this feature
- **ALT-002**: Apply the new `401` treatment to all anonymous-at-deny-time requests,
  including those downgraded by `PERMISSIVE_AUTH` — rejected per user decision
  (CON-002); would change response codes for existing permissive-mode deployments
  that already depend on today's `403` semantics
- **ALT-003**: Extend the base `types.AuthProvider` interface with `Issuers() []string`
  directly — rejected; would force Azure, Basic Auth, and No-Auth providers to
  implement a meaningless method, breaking the established optional-interface pattern
  (`types.AuthChallenger`)
- **ALT-004**: Infer `resource` from the incoming request's `Host` header instead of a
  dedicated config field — rejected per feature spec SEC-001; `Host` and
  `X-Forwarded-*` headers are attacker-controllable and unreliable behind
  proxies/tunnels

## 4. Dependencies

- **DEP-001**: No new external Go modules required — `encoding/json`, `net/url`,
  `fmt` are all standard library; `github.com/go-chi/chi/v5` (already a dependency)
  for route registration
- **DEP-002**: Reachable OIDC well-known endpoint(s) (`WELLKNOWN_OIDC`) that expose an
  `issuer` field — already a prerequisite of JWT mode today

## 5. Files

- **FILE-001**: `internal/config/config.go` — new `ResourceURL` /
  `ResourceMetadataPath` fields, `validateResourceMetadata` validation
- **FILE-002**: `internal/config/config_test.go` — validation unit tests
- **FILE-003**: `internal/jwtsupport/jwt.go` — `Issuer` field on `wellKnownData`,
  new `Issuers()` method
- **FILE-004**: `internal/jwtsupport/jwt_test.go` — `Issuers()` unit tests
- **FILE-005**: `internal/types/types.go` — new `IssuerProvider` interface
- **FILE-006**: `internal/router/metadata.go` (new) — metadata struct,
  `buildWellKnownURL`, `newProtectedResourceMetadata`, handler
- **FILE-007**: `internal/router/metadata_test.go` (new) — `buildWellKnownURL` unit
  tests
- **FILE-008**: `internal/router/types.go` — new `Proxy` fields
  (`resourceMetadataJSON`, `resourceMetadataURL`)
- **FILE-009**: `internal/router/router.go` — conditional route registration in
  `New()`
- **FILE-010**: `internal/router/auth.go` — shared `wwwAuthenticateChallenge()`
  helper, updated `401 invalid credentials` branch
- **FILE-011**: `internal/router/auth_test.go` — updated challenge-header test
  coverage
- **FILE-012**: `internal/router/policy.go` — `401`/`403` deny-path split
- **FILE-013**: `internal/router/policy_test.go` — deny-path status-code tests
- **FILE-014**: `docs/CONFIGURATION.md` — new env vars documented
- **FILE-015**: `docs/JWT.md` or new `docs/MCP-AUTH.md` — RFC 9728 flow
  documentation
- **FILE-016**: `docs/DEPLOYMENT.md` — ingress dual-path/rewrite requirement
- **FILE-017**: `examples/kubernetes/ingress.yaml` or new
  `examples/kubernetes/mcp-auth/` — example ingress rewrite rule

## 6. Testing

- **TEST-001**: `validateResourceMetadata`/`validateResourceMetadataConfig` — path
  normalization and validation-error cases (TASK-003)
- **TEST-002**: `JWTSupport.Issuers()` — de-duplication and missing-field cases
  (TASK-006)
- **TEST-003**: `buildWellKnownURL` — path-insertion correctness across
  bare-origin, path-suffixed, and custom-metadata-path inputs (TASK-010)
- **TEST-004**: Policy deny-path status-code split — full matrix across auth modes,
  credential states, and `RESOURCE_URL` configured/unconfigured (TASK-014)
- **TEST-005**: `401 invalid credentials` challenge header includes
  `resource_metadata` when enabled (TASK-015)
- **TEST-006**: Manual E2E — VS Code MCP client discovery against a real IdP
  (TASK-019)
- **TEST-007**: Manual E2E — multi-issuer `authorization_servers` array against a
  real MCP client (TASK-020)

## 7. Risks & Assumptions

- **RISK-001**: Entra ID does not serve RFC 8414 metadata at the conventional path
  (`/.well-known/oauth-authorization-server/...` 404s); whether MCP clients fall back
  to plain OIDC discovery is unverified until TASK-019 completes — the feature can be
  fully "correct" per RFC 9728 and still fail the end-to-end MCP flow against Entra ID
- **RISK-002**: An incorrect `authorization_servers` value causes token-acquisition
  failures at the IdP (e.g. Entra ID `AADSTS9010010`/`AADSTS500011` if `resource`
  doesn't match a registered `identifierUris` entry) — this is an operator
  misconfiguration risk, not a rest-rego defect, but should be called out prominently
  in docs (TASK-017)
- **RISK-003**: Operators deploying `RESOURCE_URL` with a path component must also
  update their ingress/`HTTPRoute` to add the well-known rewrite rule (REQ-006);
  forgetting this step means the advertised `resource_metadata` URL 404s at the
  ingress even though rest-rego itself is correctly configured
  — mitigated by TASK-018 documentation, but not enforceable by rest-rego itself
- **ASSUMPTION-001**: `go-arg`'s `default:` tag does not repopulate a field when the
  corresponding env var is explicitly set to an empty string — TASK-002's explicit
  `if f.ResourceMetadataPath == ""` fallback assumes this and is required for REQ-005
  to hold in that edge case; verify this behavior against the actual `go-arg` version
  in `go.mod` during TASK-002 implementation
- **ASSUMPTION-002**: Route registration via `proxy.mux.Get(cfg.ResourceMetadataPath,
  ...)` executed before `proxy.mux.Handle("/*", proxy)` causes `chi` to match the
  well-known path exactly and bypass the `proxy.mux.Use(...)` middleware chain
  (auth/policy) for that one route — this must be confirmed against `chi`'s actual
  routing/middleware-scoping behavior during TASK-011 implementation; if `chi`
  applies router-level `Use()` middleware regardless of match specificity, the
  metadata route will need to be mounted via a sub-router or `With()` to exclude
  `authHandler`/`policyHandler`

## 8. Related Specifications / Further Reading

- [.specs/features/oauth-protected-resource-metadata.md](../features/oauth-protected-resource-metadata.md)
- [.specs/features/basic-auth-support.md](../features/basic-auth-support.md) — precedent for the optional `types.AuthChallenger` interface pattern reused here
- [.specs/plan/feature-basic-auth-support-1.md](feature-basic-auth-support-1.md) — precedent plan structure for a new auth-adjacent feature in this codebase
- [RFC 9728 — OAuth 2.0 Protected Resource Metadata](https://www.rfc-editor.org/rfc/rfc9728)
- [RFC 8414 — OAuth 2.0 Authorization Server Metadata](https://www.rfc-editor.org/rfc/rfc8414)
- [MCP Authorization specification](https://modelcontextprotocol.io/specification/draft/basic/authorization)

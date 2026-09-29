---
goal: Add scopes_supported to the RFC 9728 Protected Resource Metadata document
version: "1.0"
date_created: 2026-09-29
last_updated: 2026-09-29
owner: rest-rego team
status: 'Completed'
tags: [feature, auth, mcp, rfc9728, jwt]
---

# Introduction

![Status: Completed](https://img.shields.io/badge/status-Completed-brightgreen)

Add an optional `scopes_supported` array to rest-rego's existing RFC 9728 OAuth 2.0
Protected Resource Metadata (PRM) document (served at `RESOURCE_METADATA_PATH` when
JWT (OIDC) auth mode is active and `RESOURCE_URL` is configured — see
[feature-oauth-protected-resource-metadata-1.md](feature-oauth-protected-resource-metadata-1.md)).
Without this field, MCP-aware clients such as VS Code fall back to the connecting
Entra ID tenant's generic OIDC `scopes_supported` (`openid profile email
offline_access`) when requesting a token, which — combined with an explicit
`resource` parameter — Entra ID rejects with `AADSTS9010010: The resource parameter
provided in the request doesn't match with the requested scopes`
(see [.specs/issues/missing-scopes-for-mcp.md](../issues/missing-scopes-for-mcp.md)).
This plan adds a new `RESOURCE_SCOPES` config field so operators can advertise the
resource's own app-specific scope(s), giving MCP clients a scope value that matches
the `resource` they're requesting a token for.

## 1. Requirements & Constraints

- **REQ-001**: The PRM document body gains an optional `scopes_supported` array
  field (RFC 9728 §2, optional), populated verbatim from a new config field
  (REQ-002), in the order given
- **REQ-002**: New config field `ResourceScopes []string` — env `RESOURCE_SCOPES`
  (comma-separated), flag `--resource-scopes` (repeatable) — following the existing
  `WellKnownURL`/`Audiences` `[]string` parsing convention in
  `internal/config/config.go`
- **REQ-003**: When `RESOURCE_SCOPES` is unset (empty), `scopes_supported` is
  omitted from the JSON document entirely (`omitempty`) — the wire format for
  existing deployments that don't set it is byte-for-byte unchanged
- **REQ-004**: `scopes_supported` is only ever populated when the PRM endpoint
  itself is active, i.e. the same gating already in place for the endpoint as a
  whole (JWT/OIDC auth mode **and** `RESOURCE_URL` set) — `RESOURCE_SCOPES` has no
  effect on its own
- **CON-001**: No breaking change to the existing `protectedResourceMetadata` JSON
  shape or the `newProtectedResourceMetadata` call sites' external behavior when
  `RESOURCE_SCOPES` is not set
- **CON-002**: `RESOURCE_SCOPES` values are passed through verbatim (no
  normalization, no derivation from `JWT_AUDIENCES`) — the exact scope string an
  IdP expects (e.g. Entra ID's `https://<resource>/access_as_user` or
  `api://<app-id>/access_as_user`) is operator/IdP-specific and cannot be reliably
  guessed by rest-rego
- **GUD-001**: Reuse the existing `[]string` config-field pattern (comma-separated
  env value or repeatable flag) already used by `WellKnownURL` and `Audiences` —
  do not introduce a new parsing convention
- **SEC-001**: `RESOURCE_SCOPES` is static operator-supplied startup configuration,
  never derived from request/user input — no injection surface is introduced

Update the status of each task below as the plan progresses.

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD**: `.specs/PRD.md`
- **Features**: `.specs/features/oauth-protected-resource-metadata.md` (base
  feature this plan extends)
- **Technology Stack**: Go
- **Cross-Product Dependencies**: None

## 2. Implementation Steps

### Implementation Phase 1 — Config Field

- **GOAL-001**: Add the `RESOURCE_SCOPES` config field and its validation warning.

- **TASK-001**: Add `ResourceScopes []string` field to `Fields` struct `[✅ Completed: 2026-09-29]`
  - Files: `internal/config/config.go`
  - Action: add immediately after the existing `ResourceMetadataPath` field:
    ```go
    ResourceScopes []string `arg:"--resource-scopes,env:RESOURCE_SCOPES" help:"OAuth scope(s) this resource exposes, advertised as scopes_supported in RFC 9728 metadata (requires resource-url)" placeholder:"SCOPE"`
    ```

- **TASK-002**: Warn when `RESOURCE_SCOPES` is set without `RESOURCE_URL`
  `[✅ Completed: 2026-09-29]`
  - Files: `internal/config/config.go`
  - Action: in `validateResourceMetadataConfig`, before the existing
    `if f.ResourceURL == ""` early-return block, add:
    ```go
    if f.ResourceURL == "" && len(f.ResourceScopes) > 0 {
        slog.Warn("config: resource-scopes configured but resource-url is not set — ignored, RFC 9728 metadata endpoint disabled")
    }
    ```
  - Dependencies: TASK-001

- **TASK-003**: Extend `TestValidateResourceMetadataConfig` `[✅ Completed: 2026-09-29]`
  - Files: `internal/config/config_test.go`
  - Cases to add:
    - `ResourceScopes` set, `ResourceURL` empty → no error (warning only, not
      asserted — matches existing test style for other warn-only branches)
    - JWT mode + valid `ResourceURL` + `ResourceScopes` set → no error
  - Dependencies: TASK-001, TASK-002

### Implementation Phase 2 — Metadata Document

- **GOAL-002**: Extend the PRM document struct and constructor with
  `scopes_supported`.

- **TASK-004**: Add `ScopesSupported` field to `protectedResourceMetadata`
  `[✅ Completed: 2026-09-29]`
  - Files: `internal/router/metadata.go`
  - Action: add to the existing struct:
    ```go
    type protectedResourceMetadata struct {
        Resource               string   `json:"resource"`
        AuthorizationServers   []string `json:"authorization_servers"`
        BearerMethodsSupported []string `json:"bearer_methods_supported"`
        ScopesSupported        []string `json:"scopes_supported,omitempty"`
    }
    ```

- **TASK-005**: Update `newProtectedResourceMetadata` signature `[✅ Completed: 2026-09-29]`
  - Files: `internal/router/metadata.go`
  - Action:
    ```go
    func newProtectedResourceMetadata(resourceURL string, issuers []string, scopes []string) *protectedResourceMetadata {
        return &protectedResourceMetadata{
            Resource:               resourceURL,
            AuthorizationServers:   issuers,
            BearerMethodsSupported: []string{"header"},
            ScopesSupported:        scopes,
        }
    }
    ```
  - Dependencies: TASK-004

- **TASK-006**: Add unit test for `scopes_supported` JSON behavior `[✅ Completed: 2026-09-29]`
  - Files: `internal/router/metadata_test.go`
  - Cases:
    - `newProtectedResourceMetadata(url, issuers, nil)` → marshaled JSON does
      **not** contain the `scopes_supported` key
    - `newProtectedResourceMetadata(url, issuers, []string{"a", "b"})` →
      marshaled JSON contains `"scopes_supported":["a","b"]`, order preserved
  - Dependencies: TASK-005

### Implementation Phase 3 — Wiring in `router.New`

- **GOAL-003**: Pass `cfg.ResourceScopes` through at PRM construction time and
  surface it in the startup log.

- **TASK-007**: Update the `newProtectedResourceMetadata` call site
  `[✅ Completed: 2026-09-29]`
  - Files: `internal/router/router.go`
  - Action: change
    ```go
    meta := newProtectedResourceMetadata(cfg.ResourceURL, issuers)
    ```
    to
    ```go
    meta := newProtectedResourceMetadata(cfg.ResourceURL, issuers, cfg.ResourceScopes)
    ```
    and extend the adjacent `slog.Info("router: registered RFC 9728 protected-resource metadata endpoint", ...)` call with an additional `"scopes_supported", cfg.ResourceScopes` key/value pair
  - Dependencies: TASK-005

- **TASK-008**: Add/extend a router-level test for the end-to-end response body
  `[✅ Completed: 2026-09-29]`
  - Files: `internal/router/router_test.go`
  - Cases:
    - `Fields{ResourceScopes: nil, ...}` (existing metadata-endpoint test setup) →
      response JSON has no `scopes_supported` key
    - `Fields{ResourceScopes: []string{"https://host/mcp/access_as_user"}, ...}` →
      response JSON's `scopes_supported` array contains that value
  - Dependencies: TASK-007

### Implementation Phase 4 — Documentation

- **GOAL-004**: Document the new field and its purpose (avoiding
  `AADSTS9010010`) in configuration reference, JWT guide, and the MCP example.

- **TASK-009**: Update `docs/CONFIGURATION.md` `[✅ Completed: 2026-09-29]`
  - Files: `docs/CONFIGURATION.md`
  - Action: add a row to the JWT Authentication options table:
    `| \`--resource-scopes\` | \`RESOURCE_SCOPES\` | - | OAuth scope(s) this resource exposes; advertised as \`scopes_supported\` in the RFC 9728 metadata document (requires \`RESOURCE_URL\`) |`
    and add a `scopes_supported` line to the RFC 9728 section's example JSON
    output, plus a one-paragraph explanation of why it matters for MCP clients
  - Dependencies: TASK-007

- **TASK-010**: Update `docs/JWT.md` `[✅ Completed: 2026-09-29]`
  - Files: `docs/JWT.md`
  - Action: in the "Enabling the endpoint" example JSON, add
    `"scopes_supported": ["https://api.example.com/mcp/access_as_user"]`; add a
    short subsection explaining that omitting `scopes_supported` causes MCP
    clients (e.g. VS Code) to fall back to the IdP's generic OIDC scopes, which
    Entra ID rejects when combined with an explicit `resource` parameter
    (`AADSTS9010010`)
  - Dependencies: TASK-007

- **TASK-011**: Update `examples/kubernetes/mcp-auth/deployment.yaml`
  `[✅ Completed: 2026-09-29]`
  - Files: `examples/kubernetes/mcp-auth/deployment.yaml`
  - Action: add an env entry after the existing `RESOURCE_URL` entry:
    ```yaml
    - name: RESOURCE_SCOPES
      value: https://api.example.com/mcp/access_as_user
    ```
  - Dependencies: TASK-007

- **TASK-012**: Update `examples/kubernetes/mcp-auth/README.md` `[✅ Completed: 2026-09-29]`
  - Files: `examples/kubernetes/mcp-auth/README.md`
  - Action: in the "Configure the identity provider and resource URL" step, add
    `RESOURCE_SCOPES` to the bullet list (the app registration's exposed scope,
    e.g. `access_as_user`, expressed as a full scope URI matching `RESOURCE_URL`)
    and add a note that omitting it can cause `AADSTS9010010` for MCP clients
    that fall back to generic tenant scopes
  - Dependencies: TASK-011

## 3. Alternatives

- **ALT-001**: Derive `scopes_supported` automatically from `JWT_AUDIENCES` plus a
  fixed suffix (e.g. `<audience>/access_as_user`) — rejected: the audience value
  and the app's actual exposed scope name are independent app-registration
  settings with no reliable naming relationship; guessing would silently produce
  wrong values.
- **ALT-002**: Query the Entra ID Graph API at startup to auto-discover the app
  registration's `oauth2PermissionScopes` — rejected: introduces an Azure Graph
  dependency into JWT-only auth mode (violates the project's mutually-exclusive
  Azure-vs-JWT auth mode convention) and adds a startup-time network dependency
  and failure mode for a purely cosmetic metadata field.
- **ALT-003**: Make `scopes_supported` mandatory whenever `RESOURCE_URL` is set
  — rejected: breaking change for existing deployments; there is no universally
  correct default scope value to fall back to, so this must remain opt-in.

## 4. Dependencies

- **DEP-001**: None — reuses `encoding/json` (already imported in
  `internal/router/metadata.go`) and `go-arg`'s existing `[]string` env/flag
  parsing (already used by `WellKnownURL`, `Audiences`).

## 5. Files

- **FILE-001**: `internal/config/config.go` — new `ResourceScopes` field and
  validation warning
- **FILE-002**: `internal/config/config_test.go` — new test cases
- **FILE-003**: `internal/router/metadata.go` — `ScopesSupported` field and
  constructor signature change
- **FILE-004**: `internal/router/metadata_test.go` — new JSON-shape test
- **FILE-005**: `internal/router/router.go` — updated call site and log line
- **FILE-006**: `internal/router/router_test.go` — end-to-end response body test
- **FILE-007**: `docs/CONFIGURATION.md` — new option row + example update
- **FILE-008**: `docs/JWT.md` — example update + explanatory subsection
- **FILE-009**: `examples/kubernetes/mcp-auth/deployment.yaml` — new env var
  example
- **FILE-010**: `examples/kubernetes/mcp-auth/README.md` — new config step

## 6. Testing

- **TEST-001**: `TestValidateResourceMetadataConfig` (extended) — `RESOURCE_SCOPES`
  without `RESOURCE_URL` produces no error; with `RESOURCE_URL` also produces no
  error
- **TEST-002**: `internal/router/metadata_test.go` (new test) — `scopes_supported`
  omitted from JSON when scopes are nil/empty; present with correct values and
  order when set
- **TEST-003**: `internal/router/router_test.go` (extended) — full PRM endpoint
  HTTP response reflects `RESOURCE_SCOPES` correctly, and is unchanged when unset

## 7. Risks & Assumptions

- **RISK-001**: Operators may enter the wrong scope string format for their IdP
  (e.g. Entra ID's `api://<app-id>/access_as_user` vs. a full HTTPS-URI form) —
  mitigated by documenting a concrete, worked Entra ID example in TASK-010 and
  TASK-012.
- **ASSUMPTION-001**: MCP clients (e.g. VS Code) implement the documented
  fallback chain (`authDetails.scopes ?? resourceMetadata.scopes_supported ??
  authorizationServerMetadata.scopes_supported`) and therefore pick up an
  app-specific `scopes_supported` value from the PRM document ahead of the IdP's
  generic OIDC scopes, resolving `AADSTS9010010` — this plan does not change
  client-side behavior, only what rest-rego advertises.

## 8. Related Specifications / Further Reading

- [.specs/issues/missing-scopes-for-mcp.md](../issues/missing-scopes-for-mcp.md) —
  originating issue and root-cause analysis
- [.specs/features/oauth-protected-resource-metadata.md](../features/oauth-protected-resource-metadata.md) —
  base RFC 9728 feature this plan extends
- [.specs/plan/feature-oauth-protected-resource-metadata-1.md](feature-oauth-protected-resource-metadata-1.md) —
  original implementation plan for the PRM endpoint
- [RFC 9728 — OAuth 2.0 Protected Resource Metadata](https://www.rfc-editor.org/rfc/rfc9728)

---
type: "feature"
feature: "rest-rego-v2"
repository_type: "single-product"
status: "proposed"
priority: "high"
complexity: "moderate"
technology_stack: ["go", "opa-rego", "lestrrat-go/jwx"]
azure_services: []
external_services: ["oidc-providers"]
on_premises_dependencies: []
multi_instance_support: "compatible"
observability: "basic"
related_prd: "PRD.md"
related_specs: ["issues/migrate-jwx-v2-to-v4.md", "issues/permissive-auth-unverified-identity.md"]
cross_product_dependencies: []
---

# Feature: rest-rego v2

## Problem Statement

rest-rego `v1.x` ships four authentication modes: JWT (OIDC), Azure Graph, Basic Auth and
No-Auth. The Azure Graph mode is the odd one out:

- It performs no local signature verification. The bearer token is decoded unverified
  (`jwt.Parse(..., jwt.WithVerify(false))` in `internal/azure/azure-app.go`) and the Graph API
  call made with that token acts as the verification.
- It adds a network round trip and an external availability dependency to the request path
  (cached for 5 minutes in `internal/azure/app-cache.go`).
- It requires consumers to request tokens for the Microsoft Graph audience
  (`scope=https://graph.microsoft.com/.default`), which is not how Entra ID is meant to protect
  an API.
- It duplicates what the JWT (OIDC) mode already does for Entra ID, which is documented in
  [docs/JWT.md](../../docs/JWT.md#azure-microsoft-entra-id), is faster, and is the recommended
  mode in the README.
- It gives `info.User` / `input.user` an object shape (the Graph service principal) that every
  policy author and every doc page has to explain. Azure is **not** the only writer of this
  field: Basic Auth sets `info.User` to the verified username string
  (`internal/basicauth/basicauth.go`). That use is undocumented, but it is today the only
  reliable "credentials were verified" signal for Basic Auth in permissive mode (see
  [issues/permissive-auth-unverified-identity.md](../issues/permissive-auth-unverified-identity.md)).

Removing Azure Graph mode is a breaking change, so it ships as `v2.0.0`. The same release
upgrades `github.com/lestrrat-go/jwx` from v2 to v4. jwx v2 is marked deprecated upstream
("no longer maintained" on pkg.go.dev). The upgrade is easier after the Azure removal,
because `internal/azure` holds two of the four non-test files that import jwx
(`azure-app.go`, `helpers.go`; the others are `internal/jwtsupport/jwt.go` and
`e2e-tests/jwt/helpers/main.go`).

## Goals

- Remove the Azure Graph authentication mode completely: code, configuration, policy input,
  samples, docs and dependencies
- Remove `input.user` from the Rego policy input. `input.request.auth` becomes the single
  verified-identity signal: it is `null` whenever credentials were missing or failed
  verification ([Q1](#open-questions), decided)
- Replace `docs/AZURE.md` with a migration guide from Azure Graph mode to JWT (OIDC) mode for
  Entra ID
- Upgrade `github.com/lestrrat-go/jwx` from `v2.1.7` to `v4` following
  [issues/migrate-jwx-v2-to-v4.md](../issues/migrate-jwx-v2-to-v4.md)
- Release as `v2.0.0` with release notes that list every breaking change

## Non-Goals

- Changing JWT, Basic Auth or No-Auth behavior beyond what the jwx upgrade, the
  permissive fix (Q1, Q11) and the optional issuer check (Q4) require
- Providing a compatibility shim that keeps `AZURE_TENANT` working
- Reimplementing Graph-based app metadata lookup as a policy-side or external data source
- Changing the Go module path. rest-rego is a deployable service, not an imported library, so
  the module stays `github.com/AB-Lindex/rest-rego` ([Q5](#open-questions), decided)
- Reusing the `-t` short flag ([Q6](#open-questions), decided)

---

## User Stories

### US-001: Azure Graph mode is gone

**As a** maintainer
**I want** the Azure Graph provider removed
**So that** there is one fewer auth path, one fewer policy-input shape, and no unverified token
decoding in the codebase

**Acceptance Criteria**:
- `internal/azure/` no longer exists
- `--azure-tenant` / `-t` / `AZURE_TENANT` are no longer defined in `internal/config/config.go`;
  passing `--azure-tenant` fails with the argument parser's unknown-flag error, and a set
  `AZURE_TENANT` environment variable makes rest-rego exit at startup with an error that
  names the migration guide ([Q3](#open-questions), decided)
- `internal/application/app.go` has no Azure case; with no provider configured the existing
  `application: no auth-provider configured` error is still produced
- `Info.User` is removed from `internal/types/request.go`; neither Azure nor Basic Auth
  writes it. The `isAuth` debug check in `internal/router/auth.go` is derived from
  `info.Request.Auth != nil` after the provider ran, and still reports
  `authenticated=true` for a verified Basic Auth request
  ([Q1](#open-questions), decided)
- `github.com/patrickmn/go-cache` is no longer in `go.mod` (verified: only
  `internal/azure/app-cache.go` imports it)
- No Go file, Rego file, Taskfile target, doc or example refers to Azure Graph mode or
  `input.user`

### US-002: Existing Azure Graph users can migrate

**As an** operator running `AZURE_TENANT`
**I want** a step-by-step guide to switch to JWT (OIDC) mode
**So that** I can move to v2 without losing tenant isolation or breaking my policies

**Acceptance Criteria**:
- The guide covers configuration, the token audience change for API consumers, the claim
  mapping from `input.user.*` to `input.jwt.*`, tenant enforcement in policy and a before/after
  policy sample (see [Migration guide content](#migration-guide-content))
- Every doc page that linked to `docs/AZURE.md` links to the migration guide or the Entra ID
  section of `docs/JWT.md` instead

### US-003: jwx is on v4

**As a** maintainer
**I want** `github.com/lestrrat-go/jwx` upgraded to v4
**So that** the project is on a supported major version of its JWT/JWK library

**Acceptance Criteria**:
- `go.mod` requires `github.com/lestrrat-go/jwx/v4` and no `jwx/v2`; `jwx/v3` remains only as the
  indirect dependency of OPA
- All phases and acceptance criteria in
  [issues/migrate-jwx-v2-to-v4.md](../issues/migrate-jwx-v2-to-v4.md) are met (that document
  assumes Azure is already removed and contains no Azure items)
- Behavior of JWT mode is unchanged: same accept/reject decisions, same `input.jwt` shape, same
  JWKS refresh and last-good-key retention

---

## Requirements

### Functional

1. Remove the Azure provider, its config field, its startup validation entry and its wiring
2. Remove `input.user` and `types.Info.User` (Azure and Basic Auth)
3. Update the mutual-exclusion error message in `config.New()` to list only
   `WELLKNOWN_OIDC`, `BASIC_AUTH_FILE` and `NO_AUTH`
4. Rewrite sample policies that use `input.user.appId` (`policies/request.rego`,
   `policies/roles.rego`) to use `input.jwt`
5. Replace `docs/AZURE.md` with the migration guide and update all references
6. Upgrade jwx per the linked migration document
7. Bump the released version to `v2.0.0` and add `bin/releasenotes-v2.0.0.md`
8. Every auth provider clears `info.Request.Auth` whenever it returns `nil` without having
   verified the credentials (permissive fallback, wrong auth kind, unknown user, empty user).
   `input.request.auth != null` therefore means "verified" in every mode, and
   `input.request.auth.user` is the documented Basic Auth identity. No-Auth mode always sets
   `info.Request.Auth = nil` ([Q11](#open-questions), decided)
10. New optional `--jwt-issuers` / `JWT_ISSUERS` (list, same format as `JWT_AUDIENCES`).
    Unset: `iss` is not checked (today's behavior). Set: the token's `iss` must equal one of
    the entries exactly, for every configured well-known; otherwise the token fails
    verification like any other invalid token ([Q4](#open-questions), decided)
11. After successful verification, `RequestInfo.ID` (logged as `id`) is set to the caller
    identifier: in JWT mode the first non-empty string claim of `azp`, `appid`,
    `client_id`, `sub`; in Basic Auth mode the username. It stays empty for anonymous
    requests and in No-Auth mode ([Q8](#open-questions), decided)
9. Fix the permissive-mode defect described in
   [issues/permissive-auth-unverified-identity.md](../issues/permissive-auth-unverified-identity.md)
   as part of `v2.0.0` ([Q2](#open-questions), decided)

### Non-Functional

- **Security**: The release must not weaken tenant isolation for migrated deployments. The
  Azure provider enforced `tid == AZURE_TENANT` in code; the JWT provider does not enforce a
  tenant, so the migration guide must make that a policy rule (see
  [Risks](#risks)). Clearing `input.request.auth` on failure must cover every permissive
  fallback path, so no unverified identity reaches the policy
- **Performance**: No regression in the JWT authenticate path (benchmarks from the jwx
  migration document apply)
- **Compatibility**: JWT, Basic Auth, No-Auth, permissive mode, RFC 9728 metadata and metrics
  behave as in `v1.6.x`
- **Multi-instance**: Stateless; unchanged

---

## Breaking Changes (v1 to v2)

| Area | v1 | v2 | Action for users |
| --- | --- | --- | --- |
| Auth mode | `AZURE_TENANT` / `--azure-tenant` / `-t` enables Azure Graph mode | Removed | Switch to `WELLKNOWN_OIDC` + `JWT_AUDIENCES` |
| Policy input (Azure) | `input.user` is the Graph service-principal object | Not populated | Use `input.jwt.*` |
| Policy input (Basic Auth) | `input.user` is the verified username (undocumented); `input.request.auth.user` holds the supplied name even when unverified in permissive mode | `input.user` removed; `input.request.auth` is `null` unless verified | Use `input.request.auth.user` (now always verified) |
| Policy input (permissive mode) | Failed credentials stay in `input.request.auth` | `input.request.auth` is `null` after any failed check | None if policies follow the docs; policies that inspected failed tokens or usernames lose that data |
| Policy input (No-Auth mode) | `input.request.auth` holds the unverified `Authorization` header (kind, token, Basic user) | Always `null` | Read `input.request.headers` instead |
| Token audience | Consumers request a Graph token | JWT mode checks `aud` against `JWT_AUDIENCES` | Consumers request a token for the API's own audience |
| Startup | `AZURE_TENANT` set together with another provider fails | Any non-empty `AZURE_TENANT` makes rest-rego exit with an error pointing to the migration guide ([Q3](#open-questions)) | Remove the variable from deployments |
| Request logs | `id` log field holds the caller's `appid` (only Azure mode sets `RequestInfo.ID`) | `id` = first of `azp`/`appid`/`client_id`/`sub` in JWT mode, username in Basic Auth mode ([Q8](#open-questions)) | None; log queries on `id` keep working and now cover more modes |
| Docs and metrics | `restrego_graph_api_calls_total` documented | Removed from docs (never emitted by code) | Remove dashboards or alerts that reference it |
| Issuer check | `iss` never checked (docs wrongly said "validated automatically") | Still not checked unless `JWT_ISSUERS` is set | None; optionally set `JWT_ISSUERS` |
| JWT parsing | jwx v2 defaults | jwx v4 defaults: strict base64url, exact time comparison with a new `JWT_CLOCK_SKEW` (default 30 s, so `exp`/`nbf` are slightly more lenient than v1). JWKS keys that cannot be parsed or are below `JWT_MIN_RSA_BITS` (new option, default 2048) are dropped individually and logged | No action for current providers (all publish RSA 2048, measured 2026-11-13); set `JWT_CLOCK_SKEW=0s` for v1-like strictness; lower `JWT_MIN_RSA_BITS` only for legacy issuers; see jwx migration document |

## Technical Design

### Removal inventory

| Item | Change |
| --- | --- |
| `internal/azure/` (`azure-app.go`, `app-cache.go`, `helpers.go`, `helpers_test.go`) | Delete the package |
| `internal/application/app.go` | Remove the `azure` import and the `AzureTenant` case in the provider switch |
| `internal/config/config.go` | Remove `AzureTenant` field; remove the `AzureTenant` branch of the auth counter; update the error message; add a startup check that exits with an error when `os.Getenv("AZURE_TENANT") != ""`; add `JWTIssuers` (`--jwt-issuers`, `JWT_ISSUERS`) |
| `internal/jwtsupport/jwt.go` (issuer) | When issuers are configured, add an issuer validator to the precomputed parse options (`jwt.WithIssuer` takes one value, so use a validator that accepts any listed value) |
| `internal/types/request.go` | Remove `Info.User`. `RequestInfo.ID` stays; filled by JWT and Basic Auth per requirement 11 ([Q8](#open-questions)) |
| `internal/router/auth.go` | `isAuth := info.Request.Auth != nil` (after the provider ran) |
| `internal/basicauth/basicauth.go` | Stop writing `info.User`; set `info.Request.ID` to the username on success; set `info.Request.Auth = nil` on every permissive fallback (`handleFailure`, non-Basic auth kind) |
| `internal/jwtsupport/jwt.go` | Set `info.Request.ID` from `azp`/`appid`/`client_id`/`sub` on success; set `info.Request.Auth = nil` on every permissive fallback (wrong auth kind, validation failure) |
| `internal/noauth/noauth.go` | Set `info.Request.Auth = nil` in `Authenticate`; update the comment mentioning `info.User` |
| `policies/request.rego`, `policies/roles.rego` | Replace `input.user.appId` with the equivalent `input.jwt` claim |
| `pkg/regocache/rego_test.go` | Review: uses a generic `input.user` key in a test policy, which is independent of the product field but should be renamed to avoid confusion |
| `go.mod` / `go.sum` | Remove `github.com/patrickmn/go-cache`; keep `go-resthelp` unless the jwx migration replaces the well-known fetch with plain `net/http` (shared HTTP client decision); run `go mod tidy` |
| `.github/copilot-instructions.md` | Remove Azure Graph from the overview, input format, provider list, critical configs and conventions |
| `tests/initial.http`, `tests/README.md` | Remove Graph API requests and the Azure tenant variable |
| `Taskfile.yml` | Rename `run-azure` to `run-entra`; replace the `common` well-known with `https://login.microsoftonline.com/{{.TENANT}}/v2.0/.well-known/openid-configuration`, reading `TENANT` from `.env` (Taskfile has no `dotenv` entry today; add `dotenv: ['.env']` to the task or file) ([Q7](#open-questions), decided) |

### Documentation changes

| File | Change |
| --- | --- |
| `docs/AZURE.md` | Replace with the migration guide; rename to `docs/MIGRATING-FROM-AZURE-GRAPH.md` |
| `README.md` | Remove Azure Graph from the feature list, the auth comparison table, the Docker example and the docs index; state "JWT (OIDC), Basic Auth, No-Auth" |
| `WHY.md` | Replace the Azure Authentication link |
| `docs/CONFIGURATION.md` | Remove the Azure Graph section, table of contents entry, `AZURE_TENANT` references, the mutual-exclusion message and the checklist item |
| `docs/DEPLOYMENT.md` | Replace the Secrets example (it stores `AZURE_TENANT`, which is not a secret) with a non-Azure example. Remove the Azure Workload Identity section: rest-rego never used its own Azure credentials (the Graph call reused the caller's bearer token, and OIDC well-known and JWKS endpoints are public), [Q9](#open-questions), decided |
| `docs/BASIC-AUTH.md` | Mutual-exclusion sentence lists `AZURE_TENANT`; state that `input.request.auth.user` is only present after successful verification |
| `docs/JWT.md`, `docs/WSO2.md` | Point `AZURE.md` links at the Entra ID section or the migration guide |
| `docs/PERMISSIVE.md` | Remove the Azure Graph rows, the `input.user` row and the Azure example; the anonymous-input table and `input.request.auth == null` advice become correct once the fix lands |
| `docs/POLICY.md` | Remove the `user.*` input row |
| `docs/NO-AUTH.md`, `examples/no-auth/*` | Replace "`input.user` is null" with "`input.jwt` and `input.request.auth` are always `null`; use `input.request.headers` for raw header values" |
| `docs/TROUBLESHOOTING.md` | Remove the conflicting-auth entry and `AZURE_TENANT` references. The entry quotes `cannot use both Azure Tenant and OIDC well-known endpoints`, which the code does not emit; quote the real v2 message if the entry is kept |
| `docs/OBSERVABILITY.md` | Remove `restrego_graph_api_calls_total` |
| `docs/ENV-VARS.md` | Remove `AZURE_TENANT`; add `JWT_ISSUERS` |
| `docs/JWT.md` | Correct "`iss` validated automatically"; document `JWT_ISSUERS`, including the Entra v1 (`https://sts.windows.net/<tid>/`) and v2 (`https://login.microsoftonline.com/<tid>/v2.0`) issuer forms |
| `docs/CONFIGURATION.md` (issuer) | Add `JWT_ISSUERS` to the JWT section |
| `docs/FILE-BASED-JWKS.md` | Update the Go snippet to the jwx v4 API (jwx upgrade) |
| `examples/README.md` | Remove the Azure Graph alternative and link |
| `.specs/PRD.md` | Remove Azure Graph requirements and the Graph metric; remove `go-cache` (listed as used for "Azure Graph, JWK", but only Azure uses it); update dependency versions (lists jwx `v2.1.6`, `go.mod` has `v2.1.7`) |
| `SECURITY.md` | Update the "JWT/Azure validation" scope line; remove the Workload Identity recommendation ([Q9](#open-questions)) |
| `.specs/features/basic-auth-support.md`, `.specs/plan/*` | Historical; leave unchanged |

### jwx upgrade

Implement exactly as described in
[issues/migrate-jwx-v2-to-v4.md](../issues/migrate-jwx-v2-to-v4.md). That document assumes
Phase 2 of this feature is done and contains no Azure items.

Order of work: remove Azure first, then migrate jwx. That leaves the jwx migration with one
production file (`internal/jwtsupport/jwt.go`), the new shutdown wiring in
`internal/application/app.go`, the tests and the e2e helper, and it keeps the two reviews
separate. Both ship together in `v2.0.0` ([Q10](#open-questions), decided).

### Migration guide content

The replacement document must contain the following.

**1. Configuration change**

```bash
# v1 (Azure Graph)
AZURE_TENANT=<tenant-id>

# v2 (JWT / OIDC)
WELLKNOWN_OIDC=https://login.microsoftonline.com/<tenant-id>/v2.0/.well-known/openid-configuration
JWT_AUDIENCES=<application-id-uri-or-client-id>
```

The `aud` value depends on the API app registration's `requestedAccessTokenVersion`: v1
tokens (the default when the manifest value is unset) carry the Application ID URI
(`api://...`), v2 tokens carry the client ID GUID. `JWT_AUDIENCES` accepts both, so list
what the consumers actually receive. Use a tenant-specific well-known URL, not `common` or
`organizations`.

**2. Claim mapping**

| v1 `input.user.*` | v2 equivalent | Notes |
| --- | --- | --- |
| `appId` | `input.jwt.azp` (v2 tokens) or `input.jwt.appid` (v1 tokens) | Graph tokens were always v1, so v1 policies only ever saw `appid`-based values |
| `appOwnerOrganizationId` | No direct equivalent | This is the app registration's home tenant. `input.jwt.tid` is the tenant that issued the token. They are equal for single-tenant apps called within their own tenant |
| `id` | `input.jwt.oid` | Object ID of the calling service principal in the issuing tenant |
| `displayName` | not in the token | Use `azp`/`appid` and keep a name map in the policy if names are needed |
| `servicePrincipalType` | not in the token | Optional claim `idtyp` (value `app` for app-only tokens) can be added to the app registration |

**3. Token audience.** Consumers must request a token for the API instead of Graph, for
example `scope=api://<application-id-uri>/.default`. The token's `aud` must match an entry in
`JWT_AUDIENCES`. This is a change on every API consumer, not only in rest-rego.

**4. Tenant isolation.** Azure mode rejected tokens whose `tid` differed from `AZURE_TENANT`.
JWT mode verifies the signature and audience but does not require a specific tenant unless
`JWT_ISSUERS` is set. Setting it to the tenant's issuer(s) restores the tenant check in
configuration, for example
`JWT_ISSUERS=https://sts.windows.net/<tid>/,https://login.microsoftonline.com/<tid>/v2.0`
(v1 and v2 tokens). Entra ID
signs tokens for all tenants with the same keys, so a valid signature does not prove the
tenant. For a single-tenant API, other tenants cannot normally obtain a token for its
audience; for a multi-tenant API they can. The guide recommends `JWT_ISSUERS` and/or the
policy rule:

```rego
allow if {
	input.jwt.tid == "<tenant-id>"
	client_id in valid_apps
}

# v2 tokens use azp, v1 tokens use appid
client_id := input.jwt.azp if input.jwt.azp
client_id := input.jwt.appid if not input.jwt.azp
```

**5. Before/after policy sample** based on `policies/request.rego`.

**6. Behavior differences**: JWT mode verifies the signature locally against the issuer's
JWKS (no Graph call, no per-request network dependency, faster) and checks `aud`. Both modes
reject expired and not-yet-valid tokens: jwx v2 `jwt.Parse` validates `exp`, `nbf` and `iat`
by default even with `jwt.WithVerify(false)`, which Azure mode used. The `id` log field is
filled from `azp`/`appid` instead of the Graph lookup ([Q8](#open-questions)). `PERMISSIVE_AUTH` semantics are the same as for any
other provider.

---

## Implementation Plan

Perform on one branch, in this order. Each phase ends with a green `go test ./...`.

### Phase 1: Baseline

- [ ] Record `go test ./...` and `go test -race ./...` results on `main`
- [ ] Capture the jwx baseline items from the migration document (benchmarks, golden
      `input.jwt` fixture, `govulncheck`)

### Phase 2: Remove Azure Graph mode

- [ ] Delete `internal/azure/`
- [ ] Remove the config field, validation branch and error-message entry
- [ ] Remove the application wiring
- [ ] Remove `Info.User`; update `internal/basicauth/basicauth.go` and
      `internal/router/auth.go`
- [ ] Clear `info.Request.Auth` on every permissive fallback in Basic Auth and JWT
      (permissive issue)
- [ ] Update the comment in `internal/noauth/noauth.go`
- [ ] Rewrite `policies/request.rego` and `policies/roles.rego`
- [ ] Rename the generic `input.user` key in the `pkg/regocache/rego_test.go` test policy
- [ ] `go mod tidy` and confirm `go-cache` is gone
- [ ] Update `.github/copilot-instructions.md` and `tests/`; rename the Taskfile target to
      `run-entra` per Q7

### Phase 3: Documentation

- [ ] Write the migration guide and rename `docs/AZURE.md`
- [ ] Update every file in the documentation table
- [ ] Search the repository for `azure`, `AZURE_TENANT`, `input.user`, `info.User`, `graph`
      and `AZURE.md` and resolve every remaining hit that is not historical

### Phase 4: jwx upgrade

- [ ] Resolve the open questions in the jwx migration document
- [ ] Execute its Phases 1 to 5

### Phase 5: Release

- [ ] Write `bin/releasenotes-v2.0.0.md` with the breaking-change table, a link to the
      migration guide, the jwx release-note items, and a section on the permissive-mode
      defect fixed in this release (the only place it is announced)
- [ ] Run the `prepare-release` checks (build, tests, `govulncheck`, Trivy filesystem and
      image scans)
- [ ] Tag `v2.0.0`

## Test Plan

| Area | Test |
| --- | --- |
| Config | `--azure-tenant` no longer recognized (unknown flag error); `AZURE_TENANT` set to any non-empty value exits with the migration-guide error, in every auth mode |
| Config | Mutual exclusion still rejects `WELLKNOWN_OIDC` + `BASIC_AUTH_FILE`, `NO_AUTH` + either; message lists the three remaining providers |
| Application | No provider configured exits with `application: no auth-provider configured` |
| Router | `authHandler` reports `authenticated=true` for valid JWT and for valid Basic Auth, `false` for anonymous; all proceed to policy as before |
| Policy input | Marshalled `types.Info` never contains a `user` key in any mode; after any failed check in permissive mode `request.auth` is absent/`null` |
| Basic Auth | Verified user is visible as `input.request.auth.user`; with `PERMISSIVE_AUTH=true` an unknown or empty username yields `input.request.auth == null`, and the `examples/kubernetes/basic-auth` policy denies it |
| JWT | With `PERMISSIVE_AUTH=true`, invalid token and wrong auth kind yield `input.request.auth == null` |
| No-Auth | A request with an `Authorization` header (Bearer and Basic) yields `input.request.auth == null`; the header is still in `input.request.headers` |
| Issuer | `JWT_ISSUERS` unset: any `iss` accepted. Set: listed `iss` accepted, other `iss` rejected (401 strict, anonymous permissive), missing `iss` rejected; multiple entries and multiple well-knowns work |
| Request ID | `id` log field: `azp` wins over `appid`/`client_id`/`sub`; falls back in that order; username for Basic Auth; empty for anonymous, failed permissive and No-Auth requests |
| Policies | `policies/*.rego` evaluate correctly with a JWT-mode input (add or adapt a test) and pass `regal` lint (`.regal.yaml`) |
| Migration | The before/after sample in the migration guide is executed as a test: v1-style policy fails against the v2 input; migrated policy allows the right `azp`/`appid` + `tid` (v1 and v2 token shapes) and denies a wrong `tid` |
| JWT | All tests from the jwx migration document |
| Build | `go build ./...`, `go vet ./...`, `go test -race ./...`, `govulncheck`, Docker build |
| Docs | Repository search for stale references returns only the historical `.specs` files and the release notes |

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Tenant isolation silently lost after migration | For a multi-tenant API, a token from another tenant with the right audience would pass | Migration guide and release notes recommend `JWT_ISSUERS` and/or the `tid` policy rule ([Q4](#open-questions)) |
| `AZURE_TENANT` left in a deployment | Startup fails after the upgrade | Intended (Q3); release notes and the error message point to the migration guide |
| API consumers keep sending Graph-audience tokens | `401` after the upgrade | Migration guide section on audience; release notes call it out as a consumer-side change |
| Basic Auth policies that use `input.user` break | Requests denied after upgrade | Release notes; switch to `input.request.auth.user`, which is now always verified |
| A permissive fallback path is missed when clearing `request.auth` | Unverified identity still reaches the policy | Tests for every fallback path in both providers, plus the end-to-end Basic Auth example test |
| Two breaking changes in one release | Harder to bisect a regression | Keep Azure removal, permissive fix and jwx upgrade as separate commits and review units (single release decided in [Q10](#open-questions)) |

## Other v2 candidates (not committed)

- Fail at startup, instead of warning, for configuration that is ignored today (for example
  `RESOURCE_URL` or `RESOURCE_SCOPES` without JWT mode, see `validateResourceMetadataConfig`)
- Revisit the JWKS refresh interval if issuers rotate keys faster than 15 min
  (decided for v2: cache headers, min 15 min, max 24 h; see jwx migration document)
- Align the metrics tables in `docs/OBSERVABILITY.md` and `.specs/PRD.md` with what the code
  emits (the Graph metric is documented but not emitted)

## Open questions

All questions were decided on 2026-11-13. The numbers are kept because other sections
reference them.

1. **Basic Auth and `input.user`.** *Decided 2026-11-13:* remove `input.user` and
   `Info.User`; every provider clears `input.request.auth` on any failed check, so
   `input.request.auth != null` means verified and `input.request.auth.user` is the
   Basic Auth identity.
2. **Permissive-mode defect timing.** *Decided 2026-11-13:* the fix ships in `v2.0.0`;
   no v1.6.x patch.
3. **`AZURE_TENANT` guard.** *Decided 2026-11-13:* exit at startup with an error that
   points to the migration guide when `AZURE_TENANT` is set.
4. **Issuer validation.** *Decided 2026-11-13:* add an optional, explicitly configured
   list `JWT_ISSUERS` (exact match, applies to all well-knowns; unset = no check) and fix
   `docs/JWT.md`. Not derived from the well-known `issuer` field, because Entra v1 tokens
   use a different issuer and `common` advertises a `{tenantid}` placeholder.
5. **Go module path.** *Decided 2026-11-13:* keep `github.com/AB-Lindex/rest-rego`
   (no `/v2`); the module is not imported by other code.
6. **Short flag `-t`.** *Decided 2026-11-13:* leave it unassigned in v2.
7. **Taskfile `run-azure`.** *Decided 2026-11-13:* rename to `run-entra` and use the
   tenant-specific endpoint with `TENANT` from `.env`.
8. **Request ID.** *Decided 2026-11-13:* JWT mode fills it from the first of `azp`,
   `appid`, `client_id`, `sub`; Basic Auth mode with the username.
9. **Workload Identity docs.** *Decided 2026-11-13:* remove the `docs/DEPLOYMENT.md`
   section and the `SECURITY.md` mention.
10. **Release split.** *Decided 2026-11-13:* one release. The permissive fix, the jwx v4
    upgrade (including the RSA key filter) and the Azure removal all ship in `v2.0.0`.
11. **No-Auth and `input.request.auth`.** *Decided 2026-11-13:* always `null` in No-Auth
    mode, so "`input.request.auth != null` means verified" holds in every mode.

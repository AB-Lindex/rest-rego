---
goal: rest-rego v2 phases 1-3 - baseline, Azure Graph removal with permissive fix, issuer check and request ID, and documentation
version: 1.0
date_created: 2026-11-13
last_updated: 2026-11-13
owner: rest-rego maintainers
status: 'Planned'
tags: [feature, breaking-change, security, refactor, documentation, v2]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

This plan implements Phases 1, 2 and 3 of [rest-rego-v2](../features/rest-rego-v2.md):

1. **Phase 1 (Baseline):** record test, race, benchmark, vulnerability and `input.jwt` golden baselines before any code changes.
2. **Phase 2 (Code):** remove Azure Graph mode, remove `input.user` / `Info.User`, fix the permissive-mode defect ([permissive-auth-unverified-identity](../issues/permissive-auth-unverified-identity.md)), clear `input.request.auth` in No-Auth mode, add the optional `JWT_ISSUERS` check and fill `RequestInfo.ID`. Requirements 8, 10 and 11 of the feature are included in this phase (decided with the owner on 2026-11-13).
3. **Phase 3 (Documentation):** replace `docs/AZURE.md` with a migration guide and update every affected document.

Phases 4 (jwx upgrade) and 5 (release) of the feature are out of scope. They are planned separately.

## 1. Requirements & Constraints

- **REQ-001**: `internal/azure/` is deleted. No Go code imports `github.com/AB-Lindex/rest-rego/internal/azure`.
- **REQ-002**: `Fields.AzureTenant` (`-t`, `--azure-tenant`, `AZURE_TENANT`) is removed from `internal/config/config.go`. `--azure-tenant` fails with the go-arg unknown-flag error. The `-t` short flag stays unassigned (feature Q6).
- **REQ-003**: When `os.Getenv("AZURE_TENANT")` is non-empty, rest-rego exits with code 1 at startup, in every auth mode, with an error that names `docs/MIGRATING-FROM-AZURE-GRAPH.md` (feature Q3).
- **REQ-004**: The mutual-exclusion error text is exactly `only one auth-provider may be configured (WELLKNOWN_OIDC, BASIC_AUTH_FILE or NO_AUTH)`, logged as the `error` attribute of `config: invalid auth configuration`.
- **REQ-005**: `types.Info.User` is removed. The JSON policy input never contains a top-level `user` key.
- **REQ-006**: Every auth provider sets `info.Request.Auth = nil` before returning `nil` without having verified credentials. No-Auth mode always sets `info.Request.Auth = nil`. `input.request.auth != null` means "verified" in every mode (feature requirement 8, Q1, Q11).
- **REQ-007**: The debug log `router: authentication complete` reports `authenticated = (info.Request.Auth != nil)` after the provider ran.
- **REQ-008**: New option `JWTIssuers []string` with tag `arg:"--jwt-issuers,env:JWT_ISSUERS"` (comma-separated in the environment, like `JWT_AUDIENCES`). Unset: `iss` is not checked. Set: the token `iss` must equal one entry exactly, for every configured well-known; a missing `iss` fails. Failure is handled like any other invalid token (401 strict, anonymous permissive) (feature requirement 10, Q4).
- **REQ-009**: After successful verification, `info.Request.ID` is set. JWT: first non-empty string claim of `azp`, `appid`, `client_id`, `sub`, in that order. Basic Auth: the username. Empty for anonymous, failed-permissive and No-Auth requests (feature requirement 11, Q8).
- **REQ-010**: `github.com/patrickmn/go-cache` is removed from `go.mod` and `go.sum`.
- **REQ-011**: `policies/request.rego` and `policies/roles.rego` use `input.jwt` instead of `input.user.appId` and pass `regal lint` with `.regal.yaml`.
- **REQ-012**: `docs/AZURE.md` is replaced by `docs/MIGRATING-FROM-AZURE-GRAPH.md` with the content defined in the feature section "Migration guide content".
- **REQ-013**: After Phase 3, a repository search for `azure`, `AZURE_TENANT`, `input.user`, `info.User`, `graph` and `AZURE.md` returns only allowed hits (see TASK-034).
- **SEC-001**: No unverified identity reaches the policy in any mode or fallback path (REQ-006).
- **SEC-002**: Wrong password for a known Basic Auth user remains `ErrAuthenticationFailed` in both modes (unchanged).
- **SEC-003**: The migration guide recommends `JWT_ISSUERS` and/or a `tid` policy rule to keep tenant isolation.
- **CON-001**: The Go module path stays `github.com/AB-Lindex/rest-rego` (feature Q5).
- **CON-002**: jwx stays on `github.com/lestrrat-go/jwx/v2 v2.1.7` in this plan. The issuer validator uses the v2 API; Phase 4 ports it.
- **CON-003**: Each phase ends with green `go build ./...`, `go vet ./...`, `go test ./...` and `go test -race ./...`.
- **CON-004**: Historical files stay unchanged: `.specs/features/*.md` other than `rest-rego-v2.md`, `.specs/plan/*.md` other than this file, `.specs/issues/*.md`, `bin/releasenotes-v1.*.md`, `heap-dumps/*`.
- **CON-005**: `bin/releasenotes-v2.0.0.md` is not created in this plan (feature Phase 5).
- **GUD-001**: Config validation follows the existing pattern: a pure `validateXConfig() error` method that is unit-tested, wrapped by a `validateX()` method that logs with `slog.Error` and calls `os.Exit(1)`.
- **GUD-002**: Structured logging with `log/slog`; log messages use the `<package>: <message>` prefix.
- **PAT-001**: Commit boundaries (one commit per item, in this order):
  - **C1** `test: capture v1 baseline` (Phase 1)
  - **C2** `feat!: remove Azure Graph auth mode` (TASK-006 to TASK-013)
  - **C3** `fix!: remove input.user and clear unverified request.auth` (TASK-014 to TASK-019)
  - **C4** `feat: optional JWT_ISSUERS check` (TASK-020 to TASK-022)
  - **C5** `feat: set request id from verified identity` (TASK-023 to TASK-025)
  - **C6** `docs: v2 documentation and Azure Graph migration guide` (Phase 3)

Update the status tag of each task when it starts and when it finishes. Update the front matter `status` and badge when the first task starts (`In progress`) and when all tasks are done (`Completed`).

## 1.1. Repository Context

- **Repository Type**: Single-Product
- **PRD**: `/.specs/PRD.md`
- **Features**: `/.specs/features/rest-rego-v2.md`
- **Related issues**: `/.specs/issues/permissive-auth-unverified-identity.md`, `/.specs/issues/migrate-jwx-v2-to-v4.md`
- **Technology Stack**: Go 1.27, OPA Rego (`github.com/open-policy-agent/opa/v1`), `github.com/lestrrat-go/jwx/v2`, `github.com/alexflint/go-arg`, Task (`Taskfile.yml`)
- **Cross-Product Dependencies**: None

## 2. Implementation Steps

### Implementation Phase 1

- **GOAL-001**: Record the v1 baseline on `main` so later phases and Phase 4 (jwx) can prove unchanged behavior. Commit C1.

- **TASK-001**: Record test results `[📋 Planned]`
  - Command: `go test ./... 2>&1 | tee temp/baseline-v1-test.txt` and `go test -race ./... 2>&1 | tee temp/baseline-v1-race.txt`
  - Validation: both exit with code 0. If not, stop and report the failing packages.
- **TASK-002**: Capture the benchmark snapshot `[📋 Planned]`
  - Command: `./e2e-tests/bench-snapshot.sh --label=before-jwx-v4`
  - Output: `heap-dumps/<timestamp>_before-jwx-v4.*` (commit `.bench.txt` and `.summary.txt`, as for earlier snapshots)
- **TASK-003**: Record the vulnerability baseline `[📋 Planned]`
  - Command: `task govulncheck 2>&1 | tee temp/baseline-v1-govulncheck.txt`
  - Validation: the finding set is copied into the C1 commit message body.
- **TASK-004**: Add the `input.jwt` golden fixture and test `[📋 Planned]`
  - Files: `internal/jwtsupport/testdata/golden-input-jwt.json` (new), `internal/jwtsupport/golden_test.go` (new)
  - Test `TestAuthenticate_GoldenClaims` with three subtests: `single-aud`, `multi-aud`, `custom-claims`. Each signs an RS256 token with a fixed test key (reuse the key helpers in `internal/jwtsupport/jwt_test.go`), using fixed `iat = 1700000000`, `nbf = 1700000000`, `exp = 4102444800` (2100-01-01), `iss = "https://issuer.example/"`, `sub = "golden-sub"`, `azp = "golden-azp"`. `multi-aud` uses `aud = ["test-audience","other"]`. `custom-claims` adds `roles = ["a","b"]`, `tid = "golden-tid"`, `nested = {"k": 1}`.
  - The test runs `Authenticate`, marshals `info.JWT` with `encoding/json`, and compares it to the matching key in the fixture file. With env `UPDATE_GOLDEN=1` it writes the fixture instead.
  - Validation: fixture generated once on `main`, test passes, fixture committed.
- **TASK-005**: Commit C1 `[📋 Planned]`
  - Files: TASK-002 snapshot files, TASK-004 files. `temp/` stays untracked.

### Implementation Phase 2

- **GOAL-002**: Remove Azure Graph mode and `input.user`, fix permissive fallbacks, clear No-Auth `request.auth`, add `JWT_ISSUERS` and fill `RequestInfo.ID`. Commits C2 to C5.

#### C2: Remove Azure Graph mode

- **TASK-006**: Delete the Azure package `[📋 Planned]`
  - Files: delete `internal/azure/azure-app.go`, `internal/azure/app-cache.go`, `internal/azure/helpers.go`, `internal/azure/helpers_test.go`, then the directory.
- **TASK-007**: Remove Azure wiring from the application `[📋 Planned]`
  - File: `internal/application/app.go`
  - Remove the import `github.com/AB-Lindex/rest-rego/internal/azure` and the `case len(app.config.AzureTenant) > 0:` branch (3 lines) in `New()`. Keep the `default:` branch with `application: no auth-provider configured`.
- **TASK-008**: Remove `AzureTenant` and add the auth-config validator `[📋 Planned]`
  - File: `internal/config/config.go`
  - Delete the `AzureTenant` field.
  - Add `const azureMigrationGuide = "https://github.com/AB-Lindex/rest-rego/blob/main/docs/MIGRATING-FROM-AZURE-GRAPH.md"`.
  - Add `func (f *Fields) validateAuthConfig(getenv func(string) string) error` that returns, in this order:
    1. if `getenv("AZURE_TENANT") != ""`: `fmt.Errorf("AZURE_TENANT is no longer supported: Azure Graph mode was removed in v2, see %s", azureMigrationGuide)`
    2. count of `len(f.WellKnownURL) > 0`, `f.BasicAuthFile != ""`, `f.NoAuth` > 1: `errors.New("only one auth-provider may be configured (WELLKNOWN_OIDC, BASIC_AUTH_FILE or NO_AUTH)")`
    3. if `len(f.WellKnownURL) > 0 && len(f.Audiences) == 0`: `errors.New("audiences must be provided when using well-known")`
    4. otherwise `nil`
  - Add `func (f *Fields) validateAuth()` that calls `f.validateAuthConfig(os.Getenv)` and on error logs `slog.Error("config: invalid auth configuration", "error", err)` and calls `os.Exit(1)`.
  - In `New()`, replace the `authCount` block and the audiences check with `f.validateAuth()`, called before `f.validateResourceMetadata()`.
- **TASK-009**: Unit-test the auth-config validator `[📋 Planned]`
  - File: `internal/config/config_test.go`
  - Add table test `TestValidateAuthConfig` with a fake `getenv`: AZURE_TENANT set + JWT, AZURE_TENANT set + Basic, AZURE_TENANT set + NoAuth, AZURE_TENANT set + nothing (all return the guide error containing `MIGRATING-FROM-AZURE-GRAPH.md`); JWT + Basic, JWT + NoAuth, Basic + NoAuth (multi-provider error); JWT without audiences; each single provider valid; no provider valid (`nil`, the application layer reports it).
  - Add `TestAzureTenantFlagUnknown`: `arg.NewParser(arg.Config{}, &Fields{})` then `p.Parse([]string{"--azure-tenant", "x"})` returns a non-nil error.
- **TASK-010**: Rewrite sample policies to JWT input `[📋 Planned]`
  - Files: `policies/request.rego`, `policies/roles.rego`
  - In both files add:
    ```rego
    # v2 tokens use azp, v1 tokens use appid
    client_id := input.jwt.azp if input.jwt.azp

    client_id := input.jwt.appid if {
    	not input.jwt.azp
    	input.jwt.appid
    }
    ```
  - `request.rego`: replace `print("appid-check", input.user.appId)` with `print("client-id-check", client_id)` and `input.user.appId in valid_apps` with `client_id in valid_apps`.
  - `roles.rego`: replace both `input.user.appId in ...` with `client_id in ...`; replace `app_id := input.user.appId` and its comment with nothing (the `client_id` rule replaces it).
  - Validation: `regal lint policies/` passes (if `regal` is not installed, record that in the PR and run `opa check --v1-compatible policies/`).
- **TASK-011**: Test the sample policies `[📋 Planned]`
  - File: `pkg/regocache/policies_test.go` (new)
  - `TestSamplePolicies`: copy `../../policies/request.rego` and `../../policies/roles.rego` into `t.TempDir()` and use `newTestCache` per file. Cases per file: v2 input `{"jwt":{"azp":"11112222-3333-4444-5555-666677778888"},"request":{"path":["x"]}}` allows; v1 input with `appid` allows; unknown id denies; no `jwt` denies; `request.rego` with `request.path[0] == "public"` allows without `jwt`; input with only `{"user":{"appId":"11112222-3333-4444-5555-666677778888"}}` denies.
- **TASK-012**: Rename the generic `input.user` key in the regocache test `[📋 Planned]`
  - File: `pkg/regocache/rego_test.go`, `TestEnvExpansion_multipleVars`
  - Rename policy key `input.user` to `input.name` and input map key `"user"` to `"name"` (both `Validate` calls). Env var names stay.
- **TASK-013**: Clean dependencies, tooling and assistant guide; commit C2 `[📋 Planned]`
  - `go mod tidy`; verify `grep -c patrickmn go.mod go.sum` returns `0` for both. `github.com/AB-Lindex/go-resthelp` stays (used by `internal/jwtsupport/jwt.go`).
  - `Taskfile.yml`: add top-level `dotenv: ['.env']`; rename task `run-azure` to `run-entra`, `desc: Run restrego against Microsoft Entra ID (OIDC) for the tenant in .env`, `WELLKNOWN_OIDC: https://login.microsoftonline.com/{{.TENANT}}/v2.0/.well-known/openid-configuration`. Keep `JWT_AUDIENCES`, `EXPOSE_BLOCKED_HEADERS` and `cmds`.
  - `tests/initial.http`: delete the Graph-scope `login` request and both `# Azure Info` Graph requests. Keep `login2` and the `GET {{base}}/user/someone` request.
  - `tests/README.md`: change `TENANT=azure-tenant-id` to `TENANT=<entra-tenant-id>` and remove any Graph wording.
  - `.github/copilot-instructions.md`: overview says "supporting JWT (OIDC), Basic Auth and No-Auth modes"; policy input format becomes `{"request": RequestInfo, "jwt": JWTClaims}`; "Validates mutually exclusive auth modes (JWT, Basic Auth, No-Auth)"; remove the `internal/azure` provider bullet; remove `user` from the policy-development input structure; remove `AZURE_TENANT` from critical configs and add `JWT_ISSUERS`; replace "Dual auth modes: Never allow both Azure and JWT simultaneously" with "Exactly one auth mode: JWT, Basic Auth or No-Auth"; remove Azure Graph API from external dependencies; change the `.env` line to `TENANT`, `CLIENT_ID`, `CLIENT_SECRET` for Entra ID token requests.
  - Validation: CON-003, then commit C2.

#### C3: Remove `input.user`, clear unverified `request.auth`

- **TASK-014**: Remove `Info.User` `[📋 Planned]`
  - File: `internal/types/request.go`: delete `User interface{} \`json:"user,omitempty"\`` from `Info`.
- **TASK-015**: Derive `isAuth` from `request.auth` `[📋 Planned]`
  - File: `internal/router/auth.go`, `authHandler`, `case err == nil:` replace `isAuth := info.JWT != nil || info.User != nil` with `isAuth := info.Request.Auth != nil`.
- **TASK-016**: Fix Basic Auth fallbacks `[📋 Planned]`
  - File: `internal/basicauth/basicauth.go`
  - In `Authenticate`, split the first guard: `if auth == nil { return nil }`; then `if !strings.EqualFold(auth.Kind, "basic") { info.Request.Auth = nil; return nil }` (applies in both modes, since this branch already returns `nil` in both).
  - Change `handleFailure(permissive bool) error` to `handleFailure(info *types.Info, permissive bool) error`; in the permissive branch set `info.Request.Auth = nil` before `return nil`. Update both call sites.
  - Delete `info.User = auth.User` and its comment.
  - The deferred `auth.Password = ""` stays (operates on the captured pointer).
- **TASK-017**: Fix JWT fallbacks `[📋 Planned]`
  - File: `internal/jwtsupport/jwt.go`, `Authenticate`
  - Case 2 permissive branch: set `info.Request.Auth = nil` before `return nil`.
  - Case 3 permissive branch: set `info.Request.Auth = nil` before `return nil`.
- **TASK-018**: No-Auth always clears `request.auth` `[📋 Planned]`
  - File: `internal/noauth/noauth.go`, `Authenticate`: body becomes `info.Request.Auth = nil` and `return nil`; replace the comment with `// No authentication performed; input.jwt and input.request.auth are always null.`
- **TASK-019**: Tests for C3; commit C3 `[📋 Planned]`
  - `internal/basicauth/basicauth_test.go`: `TestAuthenticate_UnknownUser_PermissiveMode` additionally asserts `info.Request.Auth == nil`; new `TestAuthenticate_EmptyUser_PermissiveMode` (same assertions); `TestAuthenticate_NonBasicAuth_Anonymous` asserts `info.Request.Auth == nil`; `TestAuthenticate_ValidCredentials_SuccessAndPasswordCleared` asserts `info.Request.Auth != nil`, `info.Request.Auth.User == <user>`, password empty; strict-mode unknown user still returns `ErrAuthenticationFailed`.
  - `internal/jwtsupport/jwt_test.go`: new `TestAuthenticate_Permissive_ClearsRequestAuth` with subtests `wrong-kind` and `invalid-token`: returns `nil`, `info.Request.Auth == nil`, `info.JWT == nil`. Existing success test asserts `info.Request.Auth != nil`.
  - `internal/noauth/noauth_test.go` (new): `Authorization: Bearer x` and `Authorization: Basic dXNlcjpwYXNz` requests built with `types.NewInfo`: after `Authenticate`, `info.Request.Auth == nil` and `info.Request.Headers["Authorization"]` still holds the header value.
  - `internal/types/request_test.go`: new `TestInfoJSON_NoUserKey`: marshal an `Info` and assert no top-level `"user"` key.
  - `internal/router/auth_test.go`: new `TestAuthHandler_AuthenticatedFlag` with a provider mock that sets or clears `info.Request.Auth`; capture `slog` output with a `slog.NewTextHandler` on a buffer at debug level (restore the default logger with `t.Cleanup`) and assert `authenticated=true` / `authenticated=false`; `next` is called in both cases.
  - `internal/router/basicauth_example_test.go` (new): end-to-end through `router.New()` and `proxy.Handler()` following the setup in `TestMetadataEndpoint_BypassesAuthAndPolicy` (`internal/router/router_test.go`), with `basicauth.New(<temp htpasswd with one bcrypt user>, true)` and policy `examples/kubernetes/basic-auth/request.rego` copied into a temp policy dir. Assert: unknown user with any password returns `403`; known user with correct password reaches the backend (`200` from an `httptest.Server`); known user with wrong password returns `401`.
  - Validation: CON-003, then commit C3.

#### C4: Optional `JWT_ISSUERS`

- **TASK-020**: Add the config field `[📋 Planned]`
  - File: `internal/config/config.go`
  - Add after `AudienceKey`: `JWTIssuers []string \`arg:"--jwt-issuers,env:JWT_ISSUERS" help:"accepted token issuers (exact match on iss); unset = issuer not checked" placeholder:"ISSUER"\``.
  - In `validateAuthConfig`, after step 3: any empty entry in `f.JWTIssuers` returns `errors.New("jwt-issuers: empty entry")`.
  - In `New()`, after `f.validateAuth()`: if `len(f.JWTIssuers) > 0 && len(f.WellKnownURL) == 0` log `slog.Warn("config: jwt-issuers is configured but JWT (OIDC) auth mode is not active — ignoring")`.
- **TASK-021**: Enforce issuers in jwtsupport `[📋 Planned]`
  - File: `internal/jwtsupport/jwt.go`
  - Add field `issuers []string` to `JWTSupport`.
  - Change `New(wellKnowns []string, audKey string, audList []string, kind string, permissive bool)` to `New(wellKnowns []string, audKey string, audList []string, issuers []string, kind string, permissive bool)` and store `issuers`.
  - Add method `func (j *JWTSupport) issuerValidator() jwt.Validator` returning `jwt.ValidatorFunc(func(_ context.Context, t jwt.Token) jwt.ValidationError { iss := t.Issuer(); if iss != "" && slices.Contains(j.issuers, iss) { return nil }; return jwt.NewValidationError(jwt.ErrInvalidIssuer()) })`.
  - In `buildParseOptions`, when `len(j.issuers) > 0`, append `jwt.WithValidator(j.issuerValidator())` after the audience option (index 0 stays the key option, as `Authenticate` overwrites `options[0]`).
  - The error text must not contain `failed to fetch`, `connection` or `timeout`, so the strict-mode classification in `Authenticate` yields `ErrAuthenticationFailed`.
  - Update callers: `internal/application/app.go` passes `app.config.JWTIssuers`; `internal/jwtsupport/jwt_test.go` calls `New(..., nil, "bearer", false)` at both call sites.
- **TASK-022**: Issuer tests; commit C4 `[📋 Planned]`
  - File: `internal/jwtsupport/issuer_test.go` (new), `TestAuthenticate_Issuers` (build `JWTSupport` like existing tests, set `issuers`, call `buildParseOptions`): unset + any `iss` accepted; set + listed `iss` accepted; set + other `iss` returns `ErrAuthenticationFailed` (strict) and `nil` with `info.Request.Auth == nil` (permissive); set + missing `iss` rejected; two entries both accepted; two well-knowns (file-based) with issuers set apply to both.
  - File: `internal/config/config_test.go`: extend `TestValidateAuthConfig` with an empty `JWTIssuers` entry case.
  - Validation: CON-003, then commit C4.

#### C5: Request ID from verified identity

- **TASK-023**: Set ID in JWT mode `[📋 Planned]`
  - File: `internal/jwtsupport/jwt.go`
  - Add `var callerIDClaims = [...]string{"azp", "appid", "client_id", "sub"}` and `func callerID(claims map[string]interface{}) string` returning the first value that is a non-empty `string`, else `""`.
  - In `Authenticate` success path, after `info.JWT = fields`: `info.Request.ID = callerID(fields)`.
- **TASK-024**: Set ID in Basic Auth mode `[📋 Planned]`
  - File: `internal/basicauth/basicauth.go`: on success (where `info.User` was set before C3) add `info.Request.ID = auth.User`.
- **TASK-025**: Request ID tests; commit C5 `[📋 Planned]`
  - `internal/jwtsupport/jwt_test.go`: table test `TestCallerID`: all four present → `azp`; no `azp` → `appid`; only `client_id` → `client_id`; only `sub` → `sub`; `azp` empty string → `appid`; `azp` non-string → next claim; none → `""`. Plus an `Authenticate` success case asserting `info.Request.ID`, and a permissive failure case asserting `info.Request.ID == ""`.
  - `internal/basicauth/basicauth_test.go`: success sets `info.Request.ID` to the username; permissive unknown user leaves it `""`.
  - `internal/noauth/noauth_test.go`: `info.Request.ID == ""`.
  - Validation: CON-003, then commit C5.

### Implementation Phase 3

- **GOAL-003**: Replace `docs/AZURE.md` with the migration guide and remove every stale Azure Graph, `input.user` and `AZURE_TENANT` reference. Document `JWT_ISSUERS`, the No-Auth and permissive input changes and the `id` log field. Commit C6.

- **TASK-026**: Write the migration guide `[📋 Planned]`
  - `git mv docs/AZURE.md docs/MIGRATING-FROM-AZURE-GRAPH.md`, then replace its content. Title: `# Migrating from Azure Graph mode to JWT (OIDC)`.
  - Sections, in order, with the content from the feature section "Migration guide content": 1. Configuration change (v1/v2 env block; `requestedAccessTokenVersion` and `aud` forms; tenant-specific well-known, not `common`/`organizations`). 2. Claim mapping table (`appId`, `appOwnerOrganizationId`, `id`, `displayName`, `servicePrincipalType`). 3. Token audience (consumer-side change, `scope=api://<application-id-uri>/.default`). 4. Tenant isolation (`JWT_ISSUERS` example with v1 and v2 issuer, `tid` policy rule, shared signing keys across tenants). 5. Before/after policy sample. 6. Behavior differences (local signature verification, `aud` check, both modes reject expired and not-yet-valid tokens, `id` log field from `azp`/`appid`, `PERMISSIVE_AUTH` unchanged). 7. Checklist (remove `AZURE_TENANT`; set `WELLKNOWN_OIDC`, `JWT_AUDIENCES`, optionally `JWT_ISSUERS`; update consumers; rewrite `input.user.*`; test in staging). The checklist is a bullet list, not a table.
  - Before/after sample: store as `examples/migrating-from-azure-graph/before.rego` (copy of the v1 `policies/request.rego` allow rule using `input.user.appId`) and `examples/migrating-from-azure-graph/after.rego` (the `tid` + `client_id` rule from the feature), and include both verbatim in the guide.
- **TASK-027**: Test the migration sample `[📋 Planned]`
  - File: `pkg/regocache/migration_guide_test.go` (new), `TestMigrationGuideSample`: load both files from `examples/migrating-from-azure-graph/` via `newTestCache`. Inputs use the v2 shape (no `user`). Assert: `before.rego` denies a v2 input; `after.rego` allows a v2-token shape (`azp` + correct `tid`), allows a v1-token shape (`appid` + correct `tid`), denies wrong `tid`, denies unknown client id, denies missing `jwt`.
  - Add a test that reads `docs/MIGRATING-FROM-AZURE-GRAPH.md` and asserts it contains the exact content of both `.rego` files, so the guide and the tested files cannot drift.
- **TASK-028**: Update top-level documents `[📋 Planned]`
  - `README.md`: intro and feature list read "JWT (OIDC), Basic Auth and No-Auth"; remove the Azure Graph row from the auth comparison table; remove the `AZURE_TENANT` Docker example (replace with the `WELLKNOWN_OIDC` + `JWT_AUDIENCES` example if none exists in that section); replace the `docs/AZURE.md` index entry with `docs/MIGRATING-FROM-AZURE-GRAPH.md` ("Migrating from v1 Azure Graph mode").
  - `WHY.md`: replace the Azure Authentication link with a link to `docs/JWT.md#azure-microsoft-entra-id`.
  - `SECURITY.md`: scope line becomes "Request authentication (JWT, Basic Auth)"; remove the Workload Identity recommendation (feature Q9).
  - `examples/README.md`: remove the "Alternative: Azure Graph Authentication" block and its link.
- **TASK-029**: Update configuration references `[📋 Planned]`
  - `docs/CONFIGURATION.md`: remove the "Azure Graph Authentication" TOC entry, list item and section (`-t, --azure-tenant` table); remove all `AZURE_TENANT` mentions and the related checklist item; update the mutual-exclusion message to REQ-004; in the "affects only JWT mode" note drop "Azure"; add `--jwt-issuers` / `JWT_ISSUERS` to the JWT section table with default "unset (issuer not checked)".
  - `docs/ENV-VARS.md`: remove `AZURE_TENANT`; add `JWT_ISSUERS` with the same description.
  - `docs/DEPLOYMENT.md`: replace the Secrets example that stores `AZURE_TENANT` with a `BASIC_AUTH_FILE` htpasswd Secret mounted as a file (or `JWT_AUDIENCES` in a ConfigMap if a Secret is not needed there); remove the Azure Workload Identity section.
  - `docs/TROUBLESHOOTING.md`: replace the conflicting-auth entry: quote the real v2 message (REQ-004) and the `AZURE_TENANT` startup error (TASK-008), with fixes "configure exactly one provider" and "remove `AZURE_TENANT`, see the migration guide".
- **TASK-030**: Update JWT documentation `[📋 Planned]`
  - `docs/JWT.md`: change item 5 under the validation list (line ~140) and the `iss` row in "Key Azure Claims" (line ~239) from "validated automatically" to "checked only when `JWT_ISSUERS` is set"; change the warning at line ~19 to "signature and audience (and issuer when `JWT_ISSUERS` is set)"; add a `### Issuer validation (JWT_ISSUERS)` subsection with the Entra v1 (`https://sts.windows.net/<tid>/`) and v2 (`https://login.microsoftonline.com/<tid>/v2.0`) forms and the multi-tenant warning; fix the troubleshooting row "Wrong tenant in WELLKNOWN_OIDC → issuer validation failed" to apply only with `JWT_ISSUERS`; add a short `### Request ID` note (`id` log field = first of `azp`, `appid`, `client_id`, `sub`); replace any `AZURE.md` link with the migration guide.
  - `docs/WSO2.md`: replace any `AZURE.md` link with `JWT.md#azure-microsoft-entra-id`; keep the WSO2 vs Entra ID comparison table.
- **TASK-031**: Update policy-input documentation `[📋 Planned]`
  - `docs/POLICY.md`: remove the `user.*` input row; state that `request.auth` is `null` unless credentials were verified, in every mode.
  - `docs/PERMISSIVE.md`: remove the Azure Graph rows, the `input.user` row and the Azure example; remove `AZURE_TENANT` from the provider list; replace "check `input.request.auth` / `input.jwt` / `input.user`" with "check `input.request.auth` / `input.jwt`"; keep `is_anonymous if { input.request.auth == null }` (now correct); add a note that v1 left failed credentials in `input.request.auth` and v2 clears them.
  - `docs/BASIC-AUTH.md`: mutual-exclusion sentence lists `WELLKNOWN_OIDC` and `NO_AUTH`; state that `input.request.auth.user` is only present after successful verification and is the Basic Auth identity; mention the `id` log field holds the username; permissive table "Unknown username: `null` auth" stays (now correct).
  - `docs/NO-AUTH.md`, `examples/no-auth/README.md`, `examples/no-auth/request.rego` (comment): replace "`input.user` is null" with "`input.jwt` and `input.request.auth` are always `null`; use `input.request.headers` for raw header values"; replace "JWT, Azure Graph token, or htpasswd" with "JWT or htpasswd".
- **TASK-032**: Update observability and file-based JWKS docs `[📋 Planned]`
  - `docs/OBSERVABILITY.md`: remove the `restrego_graph_api_calls_total` row and any dashboard panel or alert that uses it; review the "Request Rate (Graph)" item at line ~605 (it refers to a graph panel, not Microsoft Graph; keep it if so).
  - `docs/FILE-BASED-JWKS.md`: no change in this plan (jwx v4 snippet is feature Phase 4). The `subgraph` Mermaid hits are allowed.
- **TASK-033**: Update specification documents `[📋 Planned]`
  - `.specs/PRD.md`: remove REQ-003 (Azure Graph) and its acceptance criteria; renumber nothing (mark it `Removed in v2.0.0` with a one-line pointer to the migration guide, so other references keep working); update the overview, feature list, docs index (`AZURE.md` entry), policy input AC (remove `user`), conflicting-modes AC, observability (remove Graph API rates and `restrego_graph_api_calls_total`), external dependencies (remove Microsoft Graph API), `azure_services` front matter (remove `microsoft-graph`); remove `go-cache`; correct jwx version to `v2.1.7`.
  - `.specs/features/rest-rego-v2.md`: tick the Phase 1, 2 and 3 checkboxes as tasks complete; set front matter `status` to `in-progress` when Phase 1 starts.
- **TASK-034**: Stale-reference sweep; commit C6 `[📋 Planned]`
  - Command: `grep -rniE 'azure|AZURE_TENANT|input\.user|info\.User|graph|AZURE\.md' --exclude-dir={.git,bin,heap-dumps,temp,node_modules} .`
  - Allowed hits: CON-004 historical files; `.specs/features/rest-rego-v2.md`; `.specs/issues/*`; `docs/MIGRATING-FROM-AZURE-GRAPH.md`; `examples/migrating-from-azure-graph/*` and its test; Entra ID / "Azure AD" / "Azure (Microsoft Entra ID)" wording in `docs/JWT.md`, `docs/WSO2.md`, `docs/CONFIGURATION.md`, `README.md`, `WHY.md`, `.specs/PRD.md` (OIDC provider names); `go.sum` / `go.mod` `dgraph-io`; Mermaid `subgraph`; `pkg/regocache/policies_test.go` negative `user` case; `.github/prompts/*` generic framework lists; `examples/kubernetes/file-based-jwks/README.md` `cryptography.hazmat` line; `Taskfile.yml` `login.microsoftonline.com` URL.
  - Every other hit is fixed. Then run all links in changed Markdown files through a link check (`grep -o '([^)]*\.md[^)]*)'` and verify each target exists).
  - Validation: CON-003, then commit C6.

## 3. Alternatives

- **ALT-001**: Keep `Info.User` for Basic Auth only. Rejected by feature Q1: two identity signals, and `input.request.auth` must be the single verified signal.
- **ALT-002**: Add an explicit `input.request.auth.verified` flag instead of clearing `request.auth`. Rejected in the permissive issue: clearing matches the existing documentation.
- **ALT-003**: Derive accepted issuers from the well-known `issuer` field. Rejected by feature Q4: Entra v1 tokens use a different issuer and `common` advertises a `{tenantid}` placeholder.
- **ALT-004**: Use `jwt.WithIssuer` once per issuer, multiplying the precomputed option matrix. Rejected: `jwt.WithIssuer` accepts one value, and the matrix would grow per issuer; one validator accepting a list keeps a single option.
- **ALT-005**: Keep `AZURE_TENANT` as a silently ignored variable. Rejected by feature Q3: a silent ignore removes tenant enforcement without notice.
- **ALT-006**: Put the migration sample only inline in the guide. Rejected: an untested sample can drift; TASK-027 tests the files and the guide content.

## 4. Dependencies

- **DEP-001**: `github.com/lestrrat-go/jwx/v2 v2.1.7` (`jwt.ValidatorFunc`, `jwt.NewValidationError`, `jwt.ErrInvalidIssuer`, `jwt.WithValidator`).
- **DEP-002**: `github.com/alexflint/go-arg` (unknown-flag error for `--azure-tenant`, `[]string` env parsing for `JWT_ISSUERS`).
- **DEP-003**: Task with `dotenv` support (Taskfile v3) for `run-entra`.
- **DEP-004**: `regal` CLI for policy lint (fallback: `opa check`).
- **DEP-005**: `govulncheck` via `go run golang.org/x/vuln/cmd/govulncheck@latest`.
- **DEP-006**: Removed: `github.com/patrickmn/go-cache`.

## 5. Files

- **FILE-001**: `internal/azure/*` - deleted.
- **FILE-002**: `internal/application/app.go` - Azure case removed; `JWTIssuers` passed to `jwtsupport.New`.
- **FILE-003**: `internal/config/config.go`, `internal/config/config_test.go` - `AzureTenant` removed, `validateAuthConfig`, `JWTIssuers`, tests.
- **FILE-004**: `internal/types/request.go`, `internal/types/request_test.go` - `Info.User` removed, JSON test.
- **FILE-005**: `internal/router/auth.go`, `internal/router/auth_test.go`, `internal/router/basicauth_example_test.go` - `isAuth`, tests.
- **FILE-006**: `internal/basicauth/basicauth.go`, `internal/basicauth/basicauth_test.go` - fallbacks clear `request.auth`, ID, tests.
- **FILE-007**: `internal/jwtsupport/jwt.go`, `jwt_test.go`, `golden_test.go`, `issuer_test.go`, `testdata/golden-input-jwt.json` - fallbacks, issuers, ID, golden fixture.
- **FILE-008**: `internal/noauth/noauth.go`, `internal/noauth/noauth_test.go` - clears `request.auth`.
- **FILE-009**: `policies/request.rego`, `policies/roles.rego` - `input.jwt` based.
- **FILE-010**: `pkg/regocache/rego_test.go`, `policies_test.go`, `migration_guide_test.go` - tests.
- **FILE-011**: `examples/migrating-from-azure-graph/before.rego`, `after.rego` - tested migration sample.
- **FILE-012**: `go.mod`, `go.sum` - `go-cache` removed.
- **FILE-013**: `Taskfile.yml`, `tests/initial.http`, `tests/README.md`, `.github/copilot-instructions.md` - tooling.
- **FILE-014**: `docs/MIGRATING-FROM-AZURE-GRAPH.md` (renamed from `docs/AZURE.md`), `README.md`, `WHY.md`, `SECURITY.md`, `docs/CONFIGURATION.md`, `docs/ENV-VARS.md`, `docs/DEPLOYMENT.md`, `docs/TROUBLESHOOTING.md`, `docs/JWT.md`, `docs/WSO2.md`, `docs/POLICY.md`, `docs/PERMISSIVE.md`, `docs/BASIC-AUTH.md`, `docs/NO-AUTH.md`, `docs/OBSERVABILITY.md`, `examples/README.md`, `examples/no-auth/README.md`, `examples/no-auth/request.rego` - documentation.
- **FILE-015**: `.specs/PRD.md`, `.specs/features/rest-rego-v2.md` - specification updates.
- **FILE-016**: `heap-dumps/<timestamp>_before-jwx-v4.bench.txt`, `.summary.txt` - baseline.

## 6. Testing

- **TEST-001**: Baseline `go test ./...` and `go test -race ./...` green on `main` (TASK-001).
- **TEST-002**: `TestAuthenticate_GoldenClaims` passes before and after Phase 2 unchanged (TASK-004).
- **TEST-003**: `TestValidateAuthConfig`, `TestAzureTenantFlagUnknown` (TASK-009, TASK-022).
- **TEST-004**: `TestSamplePolicies` (TASK-011); `regal lint policies/`.
- **TEST-005**: Basic Auth, JWT and No-Auth permissive/fallback tests asserting `info.Request.Auth == nil` (TASK-019).
- **TEST-006**: `TestInfoJSON_NoUserKey` (TASK-019).
- **TEST-007**: `TestAuthHandler_AuthenticatedFlag` (TASK-019).
- **TEST-008**: End-to-end Basic Auth example policy test in permissive mode (TASK-019).
- **TEST-009**: `TestAuthenticate_Issuers` (TASK-022).
- **TEST-010**: `TestCallerID` and request-ID assertions in all providers (TASK-025).
- **TEST-011**: `TestMigrationGuideSample` and guide/file sync test (TASK-027).
- **TEST-012**: End of each phase: `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`, `task test`, `task build`.
- **TEST-013**: Manual smoke: `AZURE_TENANT=x bin/restrego --no-auth` exits 1 with the guide URL; `bin/restrego --azure-tenant x` exits with the unknown-flag error.
- **TEST-014**: Stale-reference sweep (TASK-034) returns only allowed hits.

## 7. Risks & Assumptions

- **RISK-001**: A permissive fallback path is missed. Mitigation: TASK-019 covers every `return nil` that is not a success in `basicauth`, `jwtsupport` and `noauth`; reviewers check each `return nil` in the three `Authenticate` functions.
- **RISK-002**: The issuer validator error is misclassified as `ErrAuthenticationUnavailable` by the string matching in `Authenticate`. Mitigation: TASK-022 asserts `ErrAuthenticationFailed` in strict mode.
- **RISK-003**: `isAuth` semantics change for Bearer requests with a wrong kind in strict mode. Not reachable: strict mode returns an error before the debug log.
- **RISK-004**: Tenant isolation is lost for migrated deployments. Mitigation: migration guide sections 4 and 7, SEC-003.
- **RISK-005**: `regal` is not installed in the environment. Mitigation: `opa check` fallback, recorded in the PR.
- **RISK-006**: The golden fixture depends on `token.AsMap` output for time claims. It is captured on `main` with jwx v2 and must not be regenerated in Phases 2 and 3.
- **ASSUMPTION-001**: `JWT_ISSUERS` without JWT mode is warned and ignored, matching the existing `RESOURCE_URL` pattern. The feature does not specify this case.
- **ASSUMPTION-002**: `JWT_ISSUERS` entries are matched without trimming; empty entries are a configuration error.
- **ASSUMPTION-003**: The non-Basic auth-kind branch in Basic Auth clears `request.auth` in both modes, because it returns `nil` (anonymous) in both modes today.
- **ASSUMPTION-004**: Each file in `policies/` is compiled as its own module by `pkg/regocache` (`rego.Module(name, ...)` per file), so `client_id` may be defined in both sample files.
- **ASSUMPTION-005**: The migration guide is linked by absolute GitHub URL on `main` from the startup error; the URL resolves once C6 is merged.

## 8. Related Specifications / Further Reading

- [rest-rego v2 feature](../features/rest-rego-v2.md)
- [Permissive mode exposes unverified credentials](../issues/permissive-auth-unverified-identity.md)
- [Migrate jwx v2 to v4](../issues/migrate-jwx-v2-to-v4.md) (feature Phase 4, out of scope)
- [PRD](../PRD.md)
- [jwx v2 jwt package](https://pkg.go.dev/github.com/lestrrat-go/jwx/v2/jwt)
- [go-arg](https://github.com/alexflint/go-arg)
- [Taskfile dotenv](https://taskfile.dev/usage/#env-files)

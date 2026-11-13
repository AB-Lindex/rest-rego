---
name: update-go-packages
description: 'Update rest-rego Go dependencies: bump direct modules in go.mod to their latest minor/patch release, report newer major versions without applying them, then run tidy, vet, race tests and govulncheck. Use when the user asks to update, bump, or upgrade Go packages, modules, or dependencies, or to check for outdated dependencies.'
---

# Update Go Packages

Updates the direct dependencies in `go.mod` to their latest release and verifies the result. Indirect dependencies only move when a direct dependency requires it. Do not commit, tag, or push.

## Procedure

Run only the scripts in [./scripts/](./scripts/), one at a time, from the repo root, as `bash .github/skills/update-go-packages/scripts/<name>.sh`. Do not run ad-hoc `go` or `git` commands to do the work; if a script lacks something, tell the user and propose a script change. Each script prints full output, stops at the first failure and also writes `release/update-pkgs-<name>.log`.

Stop at the first failing phase, report the cause, and ask how to proceed.

| Phase | Script | Covers |
|---|---|---|
| 1. Check | `check.sh` | Warns about existing `go.mod`/`go.sum` changes; table of direct modules with available updates; newer major versions (different import path); current `go` directive. Changes nothing |
| 2. Apply | `apply.sh` | `go get <each direct module>@latest` with `GOTOOLCHAIN=local`, `go mod tidy`, go directive before/after, markdown table of dependency changes |
| 3. Verify | `verify.sh` | `go mod tidy`/`verify` (warns if `go.mod`/`go.sum` changed), `go vet ./...`, `go test -race -count=1 ./...`, `govulncheck` |

### 1. Check

Run `check.sh`. Present the update table and any newer major versions to the user. If nothing is outdated, say so and stop. If `go.mod`/`go.sum` already had uncommitted changes, mention that the final diff will include them.

### 2. Apply

Run `apply.sh`. Newer major versions are reported only; they need import path changes and code migration, so never apply them without the user asking for that as a separate task.

If `go get` fails because a dependency requires a newer Go than the installed toolchain, report the module and required version. Do not install Go or change `go.mod` by hand; point to the `update-go-version` skill.

If the script warns that the `go` directive was raised, tell the user and recommend running `update-go-version` so the `Dockerfile` stays aligned.

### 3. Verify

Run `verify.sh`.

- Test failure: report the failing packages, the likely dependency responsible (compare against the table from phase 2), and leave the changes in place. Ask how to proceed, for example pinning that module to its previous version. Do not revert or edit code without asking.
- govulncheck: only vulnerabilities in called code fail the script. Vulnerabilities in required but uncalled modules are printed; list them with their IDs for the user.

### 4. Finish

Summarise: the dependency change table (direct modules first, old to new version), go directive change if any, newer major versions not applied, test result, and govulncheck result. Mention that `/prepare-release` also runs the Trivy scans. Do not commit.

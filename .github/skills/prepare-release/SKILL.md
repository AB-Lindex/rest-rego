---
name: prepare-release
description: 'Prepare a rest-rego release: run chores, build, all tests, govulncheck, Trivy fs and Trivy image scans, then ask for the next version and write bin/releasenotes-vX.Y.Z.md. Use when the user asks to prepare, verify, or cut a release, run release checks, or write release notes.'
argument-hint: 'Optional: target version, e.g. v1.7.0'
---

# Prepare Release

Verifies the repository is release-ready, then writes release notes in the style of the previous ones.
Do not tag, push, or create a GitHub release. The user does that.

## Procedure

Track each phase with the todo list. Run only the scripts in [./scripts/](./scripts/), one at a time, from the repo root, as `bash .github/skills/prepare-release/scripts/<name>.sh`. Do not run ad-hoc `git`, `go`, `podman` or `trivy` commands; if a script lacks something, tell the user and propose a script change. Each script prints full output, stops at the first failure and also writes `release/<name>.log`. `release/` is emptied by `preflight.sh` and everything in it except its `.gitignore` is ignored by git.

Stop at the first failing phase, report the cause, and ask how to proceed. Do not continue to release notes with failures or unreviewed findings.

| Phase | Script | Covers |
|---|---|---|
| 1. Preflight | `preflight.sh` | Empties `release/`; branch and tree state; latest tag and commits since; tools (`go`, `task`, `podman`, `regal`, `git`; `k6` only warns); `.trivyignore`; `go.mod` vs installed toolchain; `Dockerfile` `golang:` vs `go.mod` major.minor |
| 2. Chores | `chores.sh` | `go mod tidy`/`verify` (warns if `go.mod`/`go.sum` changed), `gofmt -l`, `go vet`, `regal lint policies`, `task build` |
| 3. Tests | `tests.sh` | `go test -race -count=1 ./...` |
| 4. Scans | `scans.sh [govulncheck\|fs\|image\|all]` | `task govulncheck`, `task scan`, `task scan-image` |
| 6. Version | `version.sh [vX.Y.Z]` | No argument: latest tag and patch/minor/major suggestions. With argument: format, tag unused, greater than latest |
| 7. Notes input | `notes-context.sh [tag]` | Commits, changed files, `go` directive change, and a markdown table of `go.mod` dependency bumps since the tag |

### 1. Preflight

Run `preflight.sh`. A dirty tree or non-`main` branch is a warning; uncommitted `go.mod`/`go.sum` changes are expected after dependency updates. A mismatch between the `go.mod` `go` directive and the installed toolchain, or between `Dockerfile` and `go.mod`, fails the script. Ask the user whether to update `go.mod` or the `Dockerfile`, since patch releases often carry security fixes. Do not change either without asking.

### 2. Chores

Run `chores.sh`.

### 3. Tests

Run `tests.sh`. E2E (k6, see `e2e-tests/README.md`) needs several terminals and takes a long time. Ask the user whether to run it. Skip by default.

### 4. Scans

Run `scans.sh govulncheck`, then `scans.sh fs`, then `scans.sh image`, one at a time (`scans.sh` without argument runs all three). A zero exit code does not mean zero findings; read the full output of each.

- govulncheck: only vulnerabilities in called code block the release. Vulnerabilities in required but uncalled modules are reported, not blocking.
- Trivy: summarise findings by severity, package, installed version, and fixed version. Any HIGH or CRITICAL finding with an available fix blocks the release. For anything else, list it and ask the user whether to accept it. Do not edit `.trivyignore` without asking.

### 5. Gate

Present a pass/fail summary table of all phases (chores, tests, govulncheck, trivy fs, trivy image). If anything failed or is unaccepted, stop here.

### 6. Version

Run `version.sh` for the latest tag and suggestions, then ask the user for the next version (use the ask-questions tool). Suggest a semver bump based on the commits and dependency changes from `notes-context.sh`: patch for fixes and dependency or toolchain updates, minor for features, major for breaking changes. If an argument was given, confirm it instead of asking from scratch. Validate the chosen version with `version.sh <version>`; it rejects a bad format, an existing tag, or a version not greater than the latest tag.

### 7. Release notes

Write `bin/releasenotes-<version>.md`. Note that `bin/` is gitignored, so the notes are not tracked. Read the examples in [./examples/](./examples/) first and use the newest one as the format reference: [v1.6.5](./examples/releasenotes-v1.6.5.md) (policy and metrics fixes) and [v1.6.6](./examples/releasenotes-v1.6.6.md) (toolchain security fix). After writing the new notes, offer to copy them into `./examples/` so the examples stay current.

Structure, include only sections that apply:

```markdown
# Changes since <previous version>

## Security / Fixes
- **<Area>**
  - <What changed and why it matters to users.>

## Features
## Tests
## Upgrade notes
```

Rules:

- Run `notes-context.sh` and base content on its output, not guesswork. Read the diff for anything unclear.
- Group by area in bold (Policy, Metrics, Toolchain, Dependencies, and so on). Write from the user's perspective.
- Dependencies: list notable bumps from the dependency table printed by `notes-context.sh` (direct modules first, old to new version; mention indirect bumps only if relevant).
- Scan results: state the full Trivy and govulncheck outcome, not only the blocking severities. Name every finding of any severity (LOW, MEDIUM, UNKNOWN included) that was reported, with its ID and whether it was fixed, accepted via `.trivyignore`, or not called. Never write "no HIGH or CRITICAL findings" when lower-severity findings existed.
- Security fixes: link the advisory by its ID, for example `[GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809)` with its CVE alias. Open the link and confirm the affected package and fixed version before citing.
- Tests: say what was added. If nothing was added, say that the full suite passes on the Go version used.
- Upgrade notes: list behaviour changes that affect operators (status codes, metric labels, config, minimum Go version). If there are none, say so.
- Plain sentences, no emojis, no marketing wording.

### 8. Finish

Show the user the path of the notes file and the suggested next steps (commit, tag `<version>`, create the GitHub release, which triggers `.github/workflows/release.yml`). Do not run them.

---
name: update-go-version
description: 'Update rest-rego to the latest installed Go version: verify the installed Go is the latest stable release, set the go.mod go directive and Dockerfile golang image, then run tidy, vet and tests. Use when the user asks to update, bump, or upgrade Go, the Go toolchain, or the Go version in go.mod or the Dockerfile.'
---

# Update Go Version

Aligns `go.mod` and `Dockerfile` with the installed Go toolchain, but only if that toolchain is the latest stable release. Do not commit, tag, or push.

## Procedure

Run only the scripts in [./scripts/](./scripts/), one at a time, from the repo root, as `bash .github/skills/update-go-version/scripts/<name>.sh`. Do not run ad-hoc `go`, `sed` or `git` commands to do the work; if a script lacks something, tell the user and propose a script change. Each script prints full output, stops at the first failure and also writes `release/update-go-<name>.log`.

Stop at the first failing phase, report the cause, and ask how to proceed.

| Phase | Script | Covers |
|---|---|---|
| 1. Check | `check.sh` | Installed Go (ignoring auto-download) vs latest stable from `https://go.dev/dl/?mode=json`; current `go.mod` and `Dockerfile` values; planned changes. Fails if offline or if installed is not the latest. Changes nothing |
| 2. Apply | `apply.sh` | Re-verifies latest, then `go mod edit -go=<x.y.z>` and `Dockerfile` `FROM golang:<x.y>`; prints the diff |
| 3. Verify | `verify.sh` | Alignment check, `go mod tidy`/`verify` (warns if `go.mod`/`go.sum` changed), `go vet ./...`, `go test -race -count=1 ./...` |

### 1. Check

Run `check.sh`. If it fails because the installed Go is not the latest stable release, tell the user the two versions and that Go must be upgraded first (https://go.dev/dl/). Do not install Go, edit files, or continue. If it fails because the feed is unreachable, report that and stop.

If everything already matches, say so and still offer to run phase 3.

### 2. Apply

Run `apply.sh` and show the user the resulting diff of `go.mod` and `Dockerfile`.

### 3. Verify

Run `verify.sh`. On a test failure, report the failing packages and the likely cause (for example a behaviour change in the new Go release). Do not modify code, or revert the version change, without asking.

### 4. Finish

Summarise: old and new `go.mod` version, `Dockerfile` image, test result, and whether `go mod tidy` changed `go.mod`/`go.sum`. Mention that `/prepare-release` also runs the vulnerability and image scans. Do not commit.

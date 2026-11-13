#!/usr/bin/env bash
# Phase 3: tidy, vet and run the unit tests with the race detector after the Go version update.
# Usage: verify.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
start_log verify

step "Alignment"
installed="$(installed_go_version)"
gm="$(gomod_go_version)"
df="$(sed -n 's/^FROM golang:\([0-9.]*\).*/\1/p' Dockerfile | head -1)"
[ "$gm" = "$installed" ] || die "go.mod ($gm) differs from installed Go ($installed); run apply.sh"
[ "$df" = "$(major_minor "$installed")" ] || die "Dockerfile golang:$df does not match $(major_minor "$installed"); run apply.sh"
ok "go.mod $gm, Dockerfile golang:$df"

step "go mod tidy / verify"
before="$(git diff -- go.mod go.sum | sha256sum)"
run go mod tidy
run go mod verify
[ "$before" = "$(git diff -- go.mod go.sum | sha256sum)" ] || warn "go mod tidy changed go.mod/go.sum"

step "go vet"
run go vet ./...

step "go test -race"
run go test -race -count=1 ./...

step "Verify passed"

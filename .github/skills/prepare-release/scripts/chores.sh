#!/usr/bin/env bash
# Phase 2: chores. Stops at the first failure.
# Usage: chores.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
start_log chores

before="$(git diff -- go.mod go.sum | sha256sum)"

step "go mod tidy / verify"
run go mod tidy
run go mod verify
[ "$before" = "$(git diff -- go.mod go.sum | sha256sum)" ] || warn "go mod tidy changed go.mod/go.sum"

step "gofmt"
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
  echo "$unformatted"
  die "files need gofmt"
fi
ok "gofmt clean"

step "go vet"
run go vet ./...

step "regal lint"
run regal lint policies

step "build"
run task build

step "Chores passed"

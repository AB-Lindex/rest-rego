#!/usr/bin/env bash
# Phase 3: tidy, vet, race tests and govulncheck after the dependency update.
# Usage: verify.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
start_log verify

step "go mod tidy / verify"
before="$(git diff -- go.mod go.sum | sha256sum)"
run go mod tidy
run go mod verify
[ "$before" = "$(git diff -- go.mod go.sum | sha256sum)" ] || warn "go mod tidy changed go.mod/go.sum"

step "go vet"
run go vet ./...

step "go test -race"
run go test -race -count=1 ./...

step "govulncheck (only vulnerabilities in called code fail)"
run go run golang.org/x/vuln/cmd/govulncheck@latest ./...

step "Verify passed; read the govulncheck output above for reported but uncalled vulnerabilities"

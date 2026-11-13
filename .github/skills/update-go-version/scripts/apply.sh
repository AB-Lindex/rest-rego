#!/usr/bin/env bash
# Phase 2: set go.mod 'go' directive and Dockerfile golang image to the installed (verified latest) Go.
# Usage: apply.sh
# Re-verifies that the installed Go is the latest stable release before editing anything.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
start_log apply

step "Verify installed Go is latest"
installed="$(installed_go_version)"
latest="$(latest_go_version)" || die "could not read https://go.dev/dl/?mode=json (offline?)"
echo "installed: $installed"
echo "latest:    $latest"
[ "$installed" = "$latest" ] || die "installed Go $installed is not the latest stable $latest; nothing changed"
mm="$(major_minor "$installed")"

step "Update go.mod"
run go mod edit -go="$installed"
echo "go directive: $(gomod_go_version)"

step "Update Dockerfile"
run sed -i -E "s/^(FROM golang:)[0-9][0-9.]*/\1$mm/" Dockerfile
grep -n '^FROM golang:' Dockerfile

step "Diff"
git --no-pager diff --stat -- go.mod go.sum Dockerfile
git --no-pager diff -- go.mod Dockerfile

step "Apply done"

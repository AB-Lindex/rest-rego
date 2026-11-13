#!/usr/bin/env bash
# Phase 1: verify the installed Go is the latest stable release and show what would change.
# Usage: check.sh
# Exit 0: installed == latest. Exit 1: not latest, offline, or unreadable. Changes nothing.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
start_log check

step "Versions"
installed="$(installed_go_version)"
gomod="$(gomod_go_version)"
dockerfile="$(sed -n 's/^FROM golang:\([0-9.]*\).*/\1/p' Dockerfile | head -1)"
echo "installed toolchain: $installed"
echo "go.mod go directive: $gomod"
echo "Dockerfile golang:   ${dockerfile:-<not found>}"
[ -n "$dockerfile" ] || die "no 'FROM golang:' line found in Dockerfile"

step "Latest stable release (go.dev)"
latest="$(latest_go_version)" || die "could not read https://go.dev/dl/?mode=json (offline?)"
echo "latest stable: $latest"

if [ "$installed" != "$latest" ]; then
  die "installed Go $installed is not the latest stable $latest; upgrade Go (https://go.dev/dl/) and rerun. Nothing was changed."
fi
ok "installed Go is the latest stable release"

step "Planned changes"
target_mm="$(major_minor "$installed")"
[ "$gomod" = "$installed" ] && ok "go.mod already $installed" || echo "go.mod:     $gomod -> $installed"
[ "$dockerfile" = "$target_mm" ] && ok "Dockerfile already golang:$target_mm" || echo "Dockerfile: golang:$dockerfile -> golang:$target_mm"

step "Check passed"

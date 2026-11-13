#!/usr/bin/env bash
# Phase 2: update direct dependencies to their latest minor/patch release and tidy.
# Usage: apply.sh
# Indirect dependencies only move when a direct dependency requires it. Major versions are never applied.
# GOTOOLCHAIN=local makes 'go get' fail instead of silently downloading a newer Go toolchain.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
start_log apply

export GOTOOLCHAIN=local

step "Direct dependencies"
mapfile -t mods < <(direct_modules)
[ "${#mods[@]}" -gt 0 ] || die "no direct dependencies found"
printf '%s\n' "${mods[@]}"
go_before="$(gomod_go_version)"

step "go get <direct modules>@latest"
args=()
for m in "${mods[@]}"; do args+=("$m@latest"); done
run go get "${args[@]}"

step "go mod tidy"
run go mod tidy

step "Go directive"
go_after="$(gomod_go_version)"
echo "go.mod go directive: $go_before -> $go_after"
if [ "$go_before" != "$go_after" ]; then
  warn "a dependency update raised the go directive; use the update-go-version skill to align Dockerfile and verify the toolchain"
fi

step "Dependency changes (HEAD -> working tree)"
dep_changes_table

step "Apply done"

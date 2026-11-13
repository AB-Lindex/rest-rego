#!/usr/bin/env bash
# Phase 1: list direct dependencies with available updates and newer major versions. Changes nothing.
# Usage: check.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
start_log check

step "Working tree"
if git diff --quiet HEAD -- go.mod go.sum; then
  ok "go.mod/go.sum have no uncommitted changes"
else
  warn "go.mod/go.sum already have uncommitted changes (for example from a Go version update); the final diff will include them"
fi

step "Direct dependencies with available updates"
go list -m -u -f '{{if and (not .Main) (not .Indirect)}}{{.Path}} {{.Version}} {{if .Update}}{{.Update.Version}}{{end}}{{end}}' all |
  awk 'NF { if (NF >= 3) { printf "| %s | %s | %s |\n", $1, $2, $3; n++ } else { c++ } }
       BEGIN { print "| Module | Current | Latest |"; print "|---|---|---|" }
       END { printf "\n%d with updates, %d up to date\n", n, c }'

step "Newer major versions (different import path, reported only, never applied)"
found=0
while read -r path; do
  case "$path" in
    gopkg.in/*) continue ;;
    *) ;;
  esac
  if [[ "$path" =~ ^(.*)/v([0-9]+)$ ]]; then
    base="${BASH_REMATCH[1]}"; n=$((BASH_REMATCH[2] + 1))
  else
    base="$path"; n=2
  fi
  # No prompts and a timeout: private or unreachable modules must not hang the probe.
  while out="$(GIT_TERMINAL_PROMPT=0 timeout 20 go list -m "$base/v$n@latest" 2>/dev/null)"; do
    echo "$path -> $out"
    found=1
    n=$((n + 1))
  done
done < <(direct_modules)
[ "$found" -eq 1 ] || ok "no newer major versions found"

step "Go directive"
echo "go.mod go directive: $(gomod_go_version)"

step "Check done (nothing changed)"

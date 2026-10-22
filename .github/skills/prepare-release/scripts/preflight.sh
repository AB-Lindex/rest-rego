#!/usr/bin/env bash
# Phase 1: preflight. Empties ./release, checks git state, tools and version alignment.
# Usage: preflight.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# Empty the log dir first, keeping only its .gitignore.
mkdir -p "$RELEASE_DIR"
find "$RELEASE_DIR" -mindepth 1 -maxdepth 1 ! -name .gitignore -exec rm -rf {} +
start_log preflight

step "Git state"
branch="$(git branch --show-current)"
echo "branch: $branch"
[ "$branch" = "main" ] || warn "branch is not main"
git status --short
if [ -n "$(git status --porcelain | grep -v -E ' (go\.mod|go\.sum)$' || true)" ]; then
  warn "working tree has changes besides go.mod/go.sum"
fi

step "Latest tag and commits since"
tag="$(latest_tag)"
echo "latest tag: ${tag:-<none>}"
if [ -n "$tag" ]; then
  git --no-pager log --oneline "$tag..HEAD"
  echo "commits since tag: $(git rev-list --count "$tag..HEAD")"
fi

step "Tools"
missing=0
for t in go task podman regal git; do
  if command -v "$t" >/dev/null; then ok "$t"; else echo "MISSING: $t"; missing=1; fi
done
# k6 is only needed for the optional E2E run.
command -v k6 >/dev/null && ok "k6" || warn "k6 missing (only needed for E2E)"
[ "$missing" -eq 0 ] || die "required tools missing"

step "Trivy prerequisites"
[ -f .trivyignore ] || die ".trivyignore missing in repo root (mounted by Trivy tasks)"
ok ".trivyignore present"

step "Go version alignment"
tc="$(toolchain_version)"
gm="$(gomod_go_version)"
echo "installed toolchain: $tc"
echo "go.mod go directive: $gm"
[ "$tc" = "$gm" ] || die "go.mod ($gm) differs from installed toolchain ($tc); ask the user whether to update go.mod"
ok "go.mod matches toolchain"

df="$(sed -n 's/^FROM golang:\([0-9.]*\).*/\1/p' Dockerfile | head -1)"
gm_mm="$(echo "$gm" | cut -d. -f1,2)"
echo "Dockerfile golang image: ${df:-<not found>}"
[ -n "$df" ] || die "no 'FROM golang:' line found in Dockerfile"
[ "$df" = "$gm_mm" ] || die "Dockerfile golang:$df does not match go.mod major.minor $gm_mm"
ok "Dockerfile matches go.mod"

step "Preflight passed"

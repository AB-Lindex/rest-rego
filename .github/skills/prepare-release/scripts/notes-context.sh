#!/usr/bin/env bash
# Phase 7 input: everything needed to write release notes, relative to the latest tag.
# Includes uncommitted go.mod changes (working tree vs tag).
# Usage: notes-context.sh [since-tag]
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

tag="${1:-$(latest_tag)}"
[ -n "$tag" ] || die "no tag to compare against"

step "Compared against $tag"

step "Commits since $tag"
git --no-pager log --oneline "$tag..HEAD"
echo "(count: $(git rev-list --count "$tag..HEAD"))"

step "Changed files since $tag (including uncommitted)"
git --no-pager diff --stat "$tag" -- . ':(exclude)go.sum'

step "Go directive and toolchain"
old_go="$(git show "$tag:go.mod" | awk '$1 == "go" { print $2; exit }')"
echo "go.mod go directive: $old_go -> $(gomod_go_version)"
echo "installed toolchain: $(toolchain_version)"

step "Dependency changes (go.mod, $tag -> working tree)"
git diff -U0 "$tag" -- go.mod | awk '
  /^-[[:space:]]+[^[:space:]]+[[:space:]]+v/ { sub(/^-[[:space:]]+/, ""); old[$1] = $2; seen[$1] = 1; next }
  /^\+[[:space:]]+[^[:space:]]+[[:space:]]+v/ {
    line = $0; sub(/^\+[[:space:]]+/, "", line)
    split(line, f, " "); new[f[1]] = f[2]; seen[f[1]] = 1
    kind[f[1]] = (line ~ /\/\/ indirect/) ? "indirect" : "direct"
    next
  }
  END {
    print "| Module | Old | New | Type |"
    print "|---|---|---|---|"
    for (m in seen) {
      o = (m in old) ? old[m] : "(new)"
      n = (m in new) ? new[m] : "(removed)"
      k = (m in kind) ? kind[m] : "-"
      print "| " m " | " o " | " n " | " k " |"
    }
  }' | { read -r h1; read -r h2; echo "$h1"; echo "$h2"; sort; }

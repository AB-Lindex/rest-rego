#!/usr/bin/env bash
# Shared helpers for the update-go-packages scripts. Source this file, do not run it.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
RELEASE_DIR="$ROOT/release"
cd "$ROOT"

# Tee all output to release/update-pkgs-<name>.log (full output also goes to stdout).
start_log() {
  mkdir -p "$RELEASE_DIR"
  exec > >(tee "$RELEASE_DIR/update-pkgs-$1.log") 2>&1
}

step() { printf '\n==> %s\n' "$*"; }
ok()   { printf 'OK:   %s\n' "$*"; }
warn() { printf 'WARN: %s\n' "$*"; }
die()  { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

# Run a command, stop the script on failure.
run() {
  printf '+ %s\n' "$*"
  "$@" || die "command failed: $*"
}

# Version of the 'go' directive in go.mod, e.g. 1.27.4
gomod_go_version() { awk '$1 == "go" { print $2; exit }' "$ROOT/go.mod"; }

# Direct (non-indirect) required modules, one path per line.
direct_modules() {
  go list -m -f '{{if and (not .Main) (not .Indirect)}}{{.Path}}{{end}}' all
}

# Markdown table of go.mod requirement changes, HEAD -> working tree.
dep_changes_table() {
  git diff -U0 HEAD -- go.mod | awk '
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
    }' | { read -r h1 || true; read -r h2 || true; echo "$h1"; echo "$h2"; sort; }
}

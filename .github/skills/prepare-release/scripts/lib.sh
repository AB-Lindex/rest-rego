#!/usr/bin/env bash
# Shared helpers for the prepare-release scripts. Source this file, do not run it.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
RELEASE_DIR="$ROOT/release"
cd "$ROOT"

# Tee all output to release/<name>.log (full output also goes to stdout).
start_log() {
  mkdir -p "$RELEASE_DIR"
  exec > >(tee "$RELEASE_DIR/$1.log") 2>&1
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

# Version of the 'go' directive in go.mod, e.g. 1.27.3
gomod_go_version() { awk '$1 == "go" { print $2; exit }' "$ROOT/go.mod"; }

# Installed toolchain version without the 'go' prefix, e.g. 1.27.3
toolchain_version() { go env GOVERSION | sed 's/^go//'; }

# Latest tag, empty if none.
latest_tag() { git describe --tags --abbrev=0 2>/dev/null || true; }

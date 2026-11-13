#!/usr/bin/env bash
# Shared helpers for the update-go-version scripts. Source this file, do not run it.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
RELEASE_DIR="$ROOT/release"
cd "$ROOT"

# Tee all output to release/update-go-<name>.log (full output also goes to stdout).
start_log() {
  mkdir -p "$RELEASE_DIR"
  exec > >(tee "$RELEASE_DIR/update-go-$1.log") 2>&1
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

# Installed toolchain, ignoring GOTOOLCHAIN auto-download, without the 'go' prefix, e.g. 1.27.4
installed_go_version() { GOTOOLCHAIN=local go env GOVERSION | sed 's/^go//'; }

# Latest stable release according to go.dev (first entry of the feed). Fails if unreachable.
latest_go_version() {
  local feed v
  feed="$(curl -fsS --max-time 20 'https://go.dev/dl/?mode=json')" || return 1
  v="$(printf '%s\n' "$feed" | sed -n 's/^ *"version": "go\([0-9][0-9.]*\)".*/\1/p' | head -1)"
  [ -n "$v" ] || return 1
  printf '%s\n' "$v"
}

# Major.minor of a version, e.g. 1.27.4 -> 1.27
major_minor() { echo "$1" | cut -d. -f1,2; }

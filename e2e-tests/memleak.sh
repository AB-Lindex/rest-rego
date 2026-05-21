#!/usr/bin/env bash
set -euo pipefail

USAGE="Usage: memleak.sh <mgmt-url> <proxy-url> <k6-script> [--token-url=URL] [--threshold-mb=N] [--size=BYTES] [--pprof=LABEL-PREFIX] [--pprof-url=URL]"

if [[ $# -lt 3 ]]; then
  echo "$USAGE" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MGMT="$1"
PROXY="$2"
K6_SCRIPT="$3"

TOKEN_URL=""
THRESHOLD_MB=50
RESPONSE_SIZE=""
PPROF_LABEL=""
PPROF_URL="http://localhost:6060"

for arg in "${@:4}"; do
  case "$arg" in
    --token-url=*)    TOKEN_URL="${arg#--token-url=}" ;;
    --threshold-mb=*) THRESHOLD_MB="${arg#--threshold-mb=}" ;;
    --size=*)         RESPONSE_SIZE="${arg#--size=}" ;;
    --pprof=*)        PPROF_LABEL="${arg#--pprof=}" ;;
    --pprof-url=*)    PPROF_URL="${arg#--pprof-url=}" ;;
    *)
      echo "Unknown argument: $arg" >&2
      echo "$USAGE" >&2
      exit 1
      ;;
  esac
done

# Prerequisite checks
for cmd in curl awk k6; do
  if ! command -v "$cmd" > /dev/null 2>&1; then
    echo "Required tool not found in PATH: $cmd" >&2
    exit 1
  fi
done

K6_VER=$(k6 version 2>&1 | head -1)
if ! echo "$K6_VER" | grep -qE 'v2\.'; then
  echo "k6 v2.x required, got: $K6_VER" >&2
  exit 1
fi

if [[ ! -f "$K6_SCRIPT" ]]; then
  echo "k6 script not found: $K6_SCRIPT" >&2
  exit 1
fi

heap() {
  curl -sf "${MGMT}/metrics" | awk '/^go_memstats_heap_inuse_bytes / {print $2}'
}

heap_mb() {
  awk -v bytes="$1" 'BEGIN { printf "%.1f MB", bytes / 1048576 }'
}

heap_dump() {
  local label="$1"
  bash "${SCRIPT_DIR}/heap-dump.sh" "$PPROF_URL" --label="${PPROF_LABEL}-${label}"
}

# 1. Capture baseline
BASELINE=$(heap)
echo "baseline heap: $(heap_mb "$BASELINE")"
[[ -n "$PPROF_LABEL" ]] && heap_dump "before"

# 2. Build k6 args array
k6_args=(run --env "PROXY=$PROXY")
if [[ -n "$TOKEN_URL" ]]; then
  k6_args+=(--env "TOKEN_URL=$TOKEN_URL")
fi
if [[ -n "$RESPONSE_SIZE" ]]; then
  k6_args+=(--env "RESPONSE_SIZE=$RESPONSE_SIZE")
  echo "response size: ${RESPONSE_SIZE} bytes"
fi

# 3. Run k6; capture exit code without letting set -e abort the script
set +e
k6 "${k6_args[@]}" "$K6_SCRIPT"
K6_EXIT=$?
set -e
[[ -n "$PPROF_LABEL" ]] && heap_dump "after-load"

# 4. GC settle period
echo "waiting 30s for GC to settle..."
sleep 30

# 5. Capture final heap
FINAL=$(heap)
[[ -n "$PPROF_LABEL" ]] && heap_dump "settled"
echo "final heap: $(heap_mb "$FINAL")"

# 6. Compare and print delta; set AWK_EXIT based on threshold
set +e
awk -v baseline="$BASELINE" -v final="$FINAL" -v threshold="$THRESHOLD_MB" 'BEGIN {
  delta = (final - baseline) / 1048576
  printf "heap delta: %.1f MB\n", delta
  exit (delta > threshold) ? 1 : 0
}'
AWK_EXIT=$?
set -e

exit $(( AWK_EXIT || K6_EXIT ))

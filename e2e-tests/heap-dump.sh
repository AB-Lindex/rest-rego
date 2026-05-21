#!/usr/bin/env bash
set -euo pipefail

USAGE="Usage: heap-dump.sh <pprof-url> [--label=NAME] [--out-dir=DIR] [--interval=SECONDS] [--count=N]

Dumps one or more pprof heap profiles to timestamped files for later analysis.

Arguments:
  pprof-url           Base URL of the pprof server (e.g. http://localhost:6060)

Options:
  --label=NAME        Optional label embedded in the filename (e.g. version, scenario)
  --out-dir=DIR       Output directory (default: heap-dumps)
  --interval=SECONDS  Seconds between dumps when --count > 1 (default: 10)
  --count=N           Number of dumps to take (default: 1)

Examples:
  # Single snapshot
  ./e2e-tests/heap-dump.sh http://localhost:6060 --label=before-load

  # Five snapshots, 30s apart, during a load test
  ./e2e-tests/heap-dump.sh http://localhost:6060 --label=during-load --count=5 --interval=30

Compare two heap profiles:
  go tool pprof -diff_base heap-dumps/<first>.pb.gz heap-dumps/<second>.pb.gz"

if [[ $# -lt 1 ]]; then
  echo "$USAGE" >&2
  exit 1
fi

PPROF_URL="$1"
LABEL=""
OUT_DIR="heap-dumps"
INTERVAL=10
COUNT=1

for arg in "${@:2}"; do
  case "$arg" in
    --label=*)    LABEL="${arg#--label=}" ;;
    --out-dir=*)  OUT_DIR="${arg#--out-dir=}" ;;
    --interval=*) INTERVAL="${arg#--interval=}" ;;
    --count=*)    COUNT="${arg#--count=}" ;;
    -h|--help)    echo "$USAGE"; exit 0 ;;
    *)
      echo "Unknown argument: $arg" >&2
      echo "$USAGE" >&2
      exit 1
      ;;
  esac
done

# Prerequisite checks
for cmd in curl; do
  if ! command -v "$cmd" > /dev/null 2>&1; then
    echo "Required tool not found in PATH: $cmd" >&2
    exit 1
  fi
done

mkdir -p "$OUT_DIR"

dump_heap() {
  local ts
  ts=$(date +"%Y%m%d_%H%M%S")

  local filename
  if [[ -n "$LABEL" ]]; then
    filename="${ts}_${LABEL}.pb.gz"
  else
    filename="${ts}.pb.gz"
  fi

  local outfile="${OUT_DIR}/${filename}"

  curl -sf --output "$outfile" "${PPROF_URL}/debug/pprof/heap"
  echo "heap dump saved: $outfile"
}

for (( i=1; i<=COUNT; i++ )); do
  dump_heap
  if (( i < COUNT )); then
    echo "waiting ${INTERVAL}s before next dump..."
    sleep "$INTERVAL"
  fi
done

echo ""
echo "To inspect: go tool pprof ${OUT_DIR}/<file>.pb.gz"
if (( COUNT > 1 )); then
  echo "To diff:    go tool pprof -diff_base ${OUT_DIR}/<first>.pb.gz ${OUT_DIR}/<last>.pb.gz"
fi

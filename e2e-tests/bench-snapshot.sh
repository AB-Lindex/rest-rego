#!/usr/bin/env bash
# bench-snapshot.sh — run the JWT + Rego allocation benchmarks and record
# pprof memory profiles so improvements can be verified later.
#
# Usage:
#   ./e2e-tests/bench-snapshot.sh [--label=NAME] [--out-dir=DIR]
#
# Options:
#   --label=NAME   Short label embedded in every output filename (e.g. "before", "after-opt1")
#   --out-dir=DIR  Directory for profiles and the summary file (default: heap-dumps)
#
# Outputs written to <out-dir>/<timestamp>[_<label>].*:
#   .jwt-auth.out      pprof alloc_space profile for BenchmarkAuthenticate
#   .jwt-multi.out     pprof alloc_space profile for BenchmarkAuthenticate_MultipleAudiences
#   .rego.out          pprof alloc_space profile for BenchmarkValidate
#   .rego-large.out    pprof alloc_space profile for BenchmarkValidate_LargeInput
#   .bench.txt         raw -benchmem numbers for all four benchmarks
#   .summary.txt       human-readable top-10 allocation sites from each profile
#
# Compare two snapshots:
#   go tool pprof -diff_base heap-dumps/<before>.jwt-auth.out heap-dumps/<after>.jwt-auth.out
set -euo pipefail

LABEL=""
OUT_DIR="heap-dumps"

for arg in "$@"; do
  case "$arg" in
    --label=*)   LABEL="${arg#--label=}" ;;
    --out-dir=*) OUT_DIR="${arg#--out-dir=}" ;;
    -h|--help)
      sed -n '/^# bench-snapshot/,/^set -/p' "$0" | grep '^#' | sed 's/^# *//'
      exit 0
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

# Prerequisite check
for cmd in go; do
  if ! command -v "$cmd" > /dev/null 2>&1; then
    echo "Required tool not found in PATH: $cmd" >&2
    exit 1
  fi
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

mkdir -p "$OUT_DIR"

TS=$(date +"%Y%m%d_%H%M%S")
if [[ -n "$LABEL" ]]; then
  PREFIX="${OUT_DIR}/${TS}_${LABEL}"
else
  PREFIX="${OUT_DIR}/${TS}"
fi

echo "bench-snapshot: label=${LABEL:-<none>}"
echo "bench-snapshot: output prefix=${PREFIX}"
echo

BENCH_OUT="${PREFIX}.bench.txt"
SUMMARY_OUT="${PREFIX}.summary.txt"

# ----- Helper: run one benchmark and capture profile + numbers -----
run_bench() {
  local pkg="$1"         # e.g. ./internal/jwtsupport/
  local bench="$2"       # regex, e.g. ^BenchmarkAuthenticate$
  local profile="$3"     # output .out path
  local desc="$4"        # human label for progress output

  echo "  running ${desc}..."

  local tmp_bin
  tmp_bin=$(mktemp /tmp/bench-XXXXXX)
  trap "rm -f ${tmp_bin}" RETURN

  # Build the test binary once so go doesn't recompile between profile and bench
  go test -c -o "${tmp_bin}" "${pkg}" 2>/dev/null

  # Run: capture numbers to BENCH_OUT, profile to file.
  # Suppress slog output by redirecting stderr; benchmark lines go to stdout.
  # The test binary prints benchmark results to stdout — extract the data line
  # (starts with the benchmark name or whitespace+digits).
  local raw
  raw=$("${tmp_bin}" \
    -test.run='^$' \
    -test.bench="${bench}" \
    -test.benchmem \
    -test.benchtime=3s \
    -test.memprofile="${profile}" \
    2>/dev/null) || true

  # Extract the result line: "BenchmarkFoo-N   <iters>  <ns/op>  <B/op>  <allocs/op>"
  echo "${raw}" | grep -E '^Benchmark' >> "${BENCH_OUT}" || true
}

# ----- Run all four benchmarks -----
echo "==> Benchmarks" | tee -a "${BENCH_OUT}"
echo "    $(date)" | tee -a "${BENCH_OUT}"
echo >> "${BENCH_OUT}"

run_bench \
  ./internal/jwtsupport/ \
  '^BenchmarkAuthenticate$' \
  "${PREFIX}.jwt-auth.out" \
  "BenchmarkAuthenticate (1 audience)"

run_bench \
  ./internal/jwtsupport/ \
  '^BenchmarkAuthenticate_MultipleAudiences$' \
  "${PREFIX}.jwt-multi.out" \
  "BenchmarkAuthenticate_MultipleAudiences (3 audiences)"

run_bench \
  ./pkg/regocache/ \
  '^BenchmarkValidate$' \
  "${PREFIX}.rego.out" \
  "BenchmarkValidate (simple policy)"

run_bench \
  ./pkg/regocache/ \
  '^BenchmarkValidate_LargeInput$' \
  "${PREFIX}.rego-large.out" \
  "BenchmarkValidate_LargeInput (large input)"

echo
echo "==> Benchmark numbers (ns/op  B/op  allocs/op)"
cat "${BENCH_OUT}"
echo

# ----- Generate summary: top-10 alloc_space per profile -----
echo "==> Generating allocation summaries..."

{
  echo "bench-snapshot summary"
  echo "Generated: $(date)"
  echo "Label:     ${LABEL:-<none>}"
  echo "Profiles:  ${PREFIX}.*"
  echo
} > "${SUMMARY_OUT}"

summarise_profile() {
  local profile="$1"
  local title="$2"

  if [[ ! -f "$profile" ]]; then
    echo "  [missing: ${profile}]"
    return
  fi

  {
    echo "--- ${title} ---"
    go tool pprof -alloc_space -top -nodecount=10 "${profile}" 2>/dev/null \
      | grep -v '^File:\|^Build\|^Type:\|^Time:\|^Showing\|^Dropped'
    echo
  } | tee -a "${SUMMARY_OUT}"
}

summarise_profile "${PREFIX}.jwt-auth.out"   "BenchmarkAuthenticate (1 aud)"
summarise_profile "${PREFIX}.jwt-multi.out"  "BenchmarkAuthenticate_MultipleAudiences (3 aud)"
summarise_profile "${PREFIX}.rego.out"       "BenchmarkValidate (simple)"
summarise_profile "${PREFIX}.rego-large.out" "BenchmarkValidate_LargeInput"

# ----- Print file listing -----
echo "==> Files written"
ls -lh "${PREFIX}".* 2>/dev/null
echo
echo "To compare two snapshots:"
echo "  go tool pprof -diff_base ${PREFIX}.jwt-auth.out <newer>.jwt-auth.out"

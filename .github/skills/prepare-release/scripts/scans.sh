#!/usr/bin/env bash
# Phase 4: vulnerability scans. Findings must be reviewed by the caller from the output/log.
# Usage: scans.sh [govulncheck|fs|image|all]   (default: all, in that order)
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

which="${1:-all}"
case "$which" in
  govulncheck|fs|image|all) ;;
  *) echo "usage: $0 [govulncheck|fs|image|all]" >&2; exit 2 ;;
esac
start_log "scans-$which"

if [ "$which" = all ] || [ "$which" = govulncheck ]; then
  step "govulncheck"
  run task govulncheck
fi

if [ "$which" = all ] || [ "$which" = fs ]; then
  step "Trivy filesystem scan"
  run task scan
fi

if [ "$which" = all ] || [ "$which" = image ]; then
  step "Trivy image scan"
  run task scan-image
fi

step "Scans finished; review findings above (a zero exit code does not mean zero findings)"

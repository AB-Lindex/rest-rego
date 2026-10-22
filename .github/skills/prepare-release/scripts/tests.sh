#!/usr/bin/env bash
# Phase 3: unit tests with the race detector.
# Usage: tests.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
start_log tests

step "go test -race"
run go test -race -count=1 ./...

step "Tests passed"

#!/usr/bin/env bash
# Phase 6: validate a proposed version and suggest a semver bump.
# Usage: version.sh                 print latest tag and suggested bumps
#        version.sh vX.Y.Z          validate format, unused tag, and greater than latest
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

tag="$(latest_tag)"
[ -n "$tag" ] || die "no tags found"

if [[ "$tag" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  ma="${BASH_REMATCH[1]}"; mi="${BASH_REMATCH[2]}"; pa="${BASH_REMATCH[3]}"
else
  die "latest tag '$tag' is not vMAJOR.MINOR.PATCH"
fi

if [ $# -eq 0 ]; then
  echo "latest tag: $tag"
  echo "patch: v$ma.$mi.$((pa + 1))"
  echo "minor: v$ma.$((mi + 1)).0"
  echo "major: v$((ma + 1)).0.0"
  exit 0
fi

new="$1"
[[ "$new" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || die "'$new' is not vMAJOR.MINOR.PATCH"
[ -z "$(git tag -l "$new")" ] || die "tag $new already exists"
highest="$(printf '%s\n%s\n' "$tag" "$new" | sort -V | tail -1)"
[ "$highest" = "$new" ] || die "$new is not greater than latest tag $tag"
ok "$new is valid and unused (latest tag: $tag)"

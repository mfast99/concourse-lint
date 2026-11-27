#!/usr/bin/env bash
# Prints the version tag of the newest release in CHANGELOG.md: the first
# heading "## [X.Y.Z]" (or "## [X.Y.Z-rc.1]") below "## [Unreleased]", as
# vX.Y.Z. The release workflow releases this version when its tag does not
# exist yet, so a release is a merge to main whose changelog has a new
# version section.
#
#   scripts/release-version.sh        # v1.2.0
#
# Without a version section the output is empty (nothing to release). A
# heading that is not a version X.Y.Z exits 2. CHANGELOG (default:
# CHANGELOG.md of the repository) names another file, for the tests in
# scripts/test-release-version.sh.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
changelog="${CHANGELOG:-$root/CHANGELOG.md}"

while IFS= read -r line; do
  [[ "$line" =~ ^"## ["([^]]*)"]" ]] || continue
  name="${BASH_REMATCH[1]}"
  [ "$name" = Unreleased ] && continue
  if ! [[ "$name" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    echo "release version: the heading \"$line\" in $changelog is not a version X.Y.Z" >&2
    exit 2
  fi
  echo "v$name"
  exit 0
done <"$changelog"

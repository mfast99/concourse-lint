#!/usr/bin/env bash
# Tests scripts/release-notes.sh against a small changelog.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
notes_script="$root/scripts/release-notes.sh"

workspace="$(mktemp -d)"
trap 'rm -rf "$workspace"' EXIT
changelog="$workspace/CHANGELOG.md"
cat >"$changelog" <<'EOF'
# Changelog

Intro text.

## [Unreleased]

### Added

- Something new.

## [1.2.0-rc.2] - 2027-02-10

### Changed

- A release candidate of its own.

## [1.1.0] - 2027-02-01

### Fixed

- A false positive in CS001.

## [1.0.0] - 2027-01-15

### Added

- First release.

## [0.9.0] - 2026-12-01

[Unreleased]: https://example.com/compare/v1.1.0...HEAD
[1.1.0]: https://example.com/compare/v1.0.0...v1.1.0
EOF

failures=0
fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

# expect_notes <tag> <expected notes>
expect_notes() {
  local tag="$1" expected="$2" actual
  if ! actual="$(CHANGELOG="$changelog" "$notes_script" "$tag" 2>&1)"; then
    fail "$tag: exited with an error: $actual"
    return
  fi
  [ "$actual" = "$expected" ] || fail "$tag: got
$actual
expected
$expected"
}

# expect_error <tag> <text the message contains>
expect_error() {
  local tag="$1" message="$2" actual
  if actual="$(CHANGELOG="$changelog" "$notes_script" "$tag" 2>&1)"; then
    fail "$tag: expected an error, got
$actual"
    return
  fi
  [[ "$actual" == *"$message"* ]] || fail "$tag: the error \"$actual\" does not contain \"$message\""
}

expect_notes v1.1.0 "### Fixed

- A false positive in CS001."

expect_notes v1.0.0 "### Added

- First release."

# A pre-release of a version without a section gets the Unreleased section.
expect_notes v1.2.0-rc.1 "### Added

- Something new."

# A pre-release of a version with a section gets that section.
expect_notes v1.1.0-rc.1 "### Fixed

- A false positive in CS001."

# A pre-release with a section of its own gets that section.
expect_notes v1.2.0-rc.2 "### Changed

- A release candidate of its own."

expect_error v2.0.0 "no section [2.0.0] in"
expect_error v0.9.0 "section [0.9.0] in"
expect_error 1.0.0 "not a version tag vX.Y.Z"
expect_error v1.0 "not a version tag vX.Y.Z"

if [ "$failures" -gt 0 ]; then
  echo "$failures release notes test(s) failed" >&2
  exit 1
fi
echo "release notes tests passed"

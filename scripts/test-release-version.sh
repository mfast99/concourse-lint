#!/usr/bin/env bash
# Tests scripts/release-version.sh against small changelogs.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
version_script="$root/scripts/release-version.sh"

workspace="$(mktemp -d)"
trap 'rm -rf "$workspace"' EXIT

failures=0
fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

# expect_version <name> <expected output> <changelog>
expect_version() {
  local name="$1" expected="$2" actual
  printf '%s' "$3" >"$workspace/CHANGELOG.md"
  if ! actual="$(CHANGELOG="$workspace/CHANGELOG.md" "$version_script" 2>&1)"; then
    fail "$name: exited with an error: $actual"
    return
  fi
  [ "$actual" = "$expected" ] || fail "$name: got \"$actual\", expected \"$expected\""
}

# expect_error <name> <text the message contains> <changelog>
expect_error() {
  local name="$1" message="$2" actual
  printf '%s' "$3" >"$workspace/CHANGELOG.md"
  if actual="$(CHANGELOG="$workspace/CHANGELOG.md" "$version_script" 2>&1)"; then
    fail "$name: expected an error, got \"$actual\""
    return
  fi
  [[ "$actual" == *"$message"* ]] || fail "$name: the error \"$actual\" does not contain \"$message\""
}

expect_version "newest version below Unreleased" v1.1.0 '# Changelog

## [Unreleased]

- Not released yet.

## [1.1.0] - 2027-02-01

- Fixed.

## [1.0.0] - 2027-01-15

- First release.

[1.1.0]: https://example.com/compare/v1.0.0...v1.1.0
'

expect_version "without Unreleased" v1.0.0 '# Changelog

## [1.0.0] - 2027-01-15

- First release.
'

expect_version "pre-release" v1.0.0-rc.1 '# Changelog

## [Unreleased]

## [1.0.0-rc.1] - 2027-01-10

- Release candidate.
'

# No released version yet: no output, nothing to release.
expect_version "only Unreleased" "" '# Changelog

## [Unreleased]

- Not released yet.
'

expect_error "heading without a patch version" "not a version X.Y.Z" '# Changelog

## [1.0] - 2027-01-15
'

expect_error "heading with a leading v" "not a version X.Y.Z" '# Changelog

## [v1.0.0] - 2027-01-15
'

if [ "$failures" -gt 0 ]; then
  echo "$failures release version test(s) failed" >&2
  exit 1
fi
echo "release version tests passed"

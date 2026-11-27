#!/usr/bin/env bash
# Installs the Homebrew formula that GoReleaser wrote to dist/release/ (make
# release-snapshot, or the release workflow) the way users do, but from the
# local archives instead of the GitHub release: a local tap, the download
# URLs rewritten to file://, brew install, the version and brew test.
#
# Needs brew (macOS, or Linuxbrew as on the GitHub-hosted Ubuntu runners).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
release="$root/dist/release"
formula="$release/homebrew/Formula/concourse-lint.rb"
tap=concourse-lint-test/local

fail() {
  echo "homebrew test failed: $*" >&2
  exit 1
}

command -v brew >/dev/null || fail "brew is not installed"
[ -f "$formula" ] || fail "no $formula; run make release-snapshot first"

export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ANALYTICS=1

brew untap --force "$tap" >/dev/null 2>&1 || true
brew tap-new --no-git "$tap" >/dev/null
trap 'brew uninstall --force concourse-lint >/dev/null 2>&1 || true; brew untap --force "$tap" >/dev/null 2>&1 || true' EXIT
tap_formula="$(brew --repository "$tap")/Formula/concourse-lint.rb"

# https://github.com/mfast99/concourse-lint/releases/download/<tag>/<archive>
# -> file://<dist/release>/<archive>
sed -E "s#https://github.com/mfast99/concourse-lint/releases/download/[^/\"]+/#file://$release/#" \
  "$formula" >"$tap_formula"
grep -q "file://$release/" "$tap_formula" || fail "no download URL of a GitHub release in $formula"

brew install --formula "$tap/concourse-lint" || fail "brew install failed"
version_output="$(concourse-lint version)"
[[ "$version_output" == "concourse-lint "* ]] || fail "unexpected version output: $version_output"
brew test "$tap/concourse-lint" || fail "brew test failed"

echo "homebrew test passed: $version_output"

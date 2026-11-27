#!/usr/bin/env bash
# Installs the npm package from dist/npm/ (scripts/package-npm.sh) into an
# empty project and runs it the way users do: npx concourse-lint. Checks the
# version, that exit codes and stdin pass through the launcher, and that the
# binaries are still executable after npm pack.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tarballs=("$root"/dist/npm/concourse-lint-*.tgz)
if [ ! -f "${tarballs[0]}" ]; then
  echo "no package in dist/npm; run scripts/package-npm.sh first" >&2
  exit 2
fi
tarball="${tarballs[0]}"

project="$(mktemp -d)"
trap 'rm -rf "$project"' EXIT
cd "$project"
npm init --yes >/dev/null
npm install --no-audit --no-fund --save-dev "$tarball" >/dev/null

fail() {
  echo "smoke test failed: $*" >&2
  exit 1
}

version_output="$(npx --no-install concourse-lint version)"
[[ "$version_output" == "concourse-lint "* ]] || fail "unexpected version output: $version_output"

for binary in node_modules/concourse-lint/bin/{linux,darwin}-*/concourse-lint; do
  [ -x "$binary" ] || fail "$binary is not executable"
done
for binary in node_modules/concourse-lint/bin/win32-*/concourse-lint.exe; do
  [ -f "$binary" ] || fail "$binary is missing"
done

# The license texts of the third-party software in the binaries.
cmp -s node_modules/concourse-lint/THIRD_PARTY_LICENSES.txt "$root/cmd/concourse-lint/third_party_licenses.txt" \
  || fail "THIRD_PARTY_LICENSES.txt is missing or differs from cmd/concourse-lint/third_party_licenses.txt"

clean_pipeline='jobs:
- name: test
  plan:
  - task: run
    config:
      platform: linux
      image_resource:
        type: registry-image
        source: {repository: busybox, tag: "1.37"}
      run: {path: "true"}
'
# With resources:, so that it is a pipeline and not a fragment (CL023).
broken_pipeline='resources:
- name: repo
  type: git
  source: {uri: "https://example.com/repo.git"}
jobs:
- name: test
  plan:
  - get: rep
'

printf '%s' "$clean_pipeline" | npx --no-install concourse-lint check - >/dev/null \
  || fail "a clean pipeline did not exit with 0"

set +e
printf '%s' "$broken_pipeline" | npx --no-install concourse-lint check - >/dev/null
broken_exit_code=$?
set -e
[ "$broken_exit_code" -eq 1 ] || fail "a pipeline with an error exited with $broken_exit_code, not 1"

echo "smoke test passed: $version_output"

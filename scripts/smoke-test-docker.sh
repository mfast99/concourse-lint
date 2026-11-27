#!/usr/bin/env bash
# Runs the container image the way users do: the version, exit codes and
# stdin through `docker run`, a shell for CI systems like GitLab CI, and a
# user that is not root.
#
#   scripts/smoke-test-docker.sh ghcr.io/mfast99/concourse-lint:1.2.3
set -euo pipefail
image="${1:-}"
if [ -z "$image" ]; then
  echo "usage: scripts/smoke-test-docker.sh <image>" >&2
  exit 2
fi

fail() {
  echo "docker smoke test failed: $*" >&2
  exit 1
}

version_output="$(docker run --rm "$image" version)" || fail "$image does not run"
[[ "$version_output" == "concourse-lint "* ]] || fail "unexpected version output: $version_output"

# GitLab CI and container jobs run a script with sh in the image.
shell_output="$(docker run --rm --entrypoint sh "$image" -c 'concourse-lint version')" \
  || fail "the image has no sh or concourse-lint is not on the PATH"
[ "$shell_output" = "$version_output" ] || fail "sh -c found another concourse-lint: $shell_output"

user_id="$(docker run --rm --entrypoint id "$image" -u)"
[ "$user_id" != 0 ] || fail "the image runs as root"

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

printf '%s' "$clean_pipeline" | docker run --rm -i "$image" check - >/dev/null \
  || fail "a clean pipeline did not exit with 0"

set +e
printf '%s' "$broken_pipeline" | docker run --rm -i "$image" check - >/dev/null
broken_exit_code=$?
set -e
[ "$broken_exit_code" -eq 1 ] || fail "a pipeline with an error exited with $broken_exit_code, not 1"

echo "docker smoke test passed: $image ($version_output)"

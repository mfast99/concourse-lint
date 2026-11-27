#!/usr/bin/env bash
# Tests the hooks in .pre-commit-hooks.yaml the way users run them: a Git
# repository with a clean pipeline and a YAML file that is no pipeline
# passes, a broken pipeline fails. "pre-commit try-repo" takes the hooks
# from this checkout, uncommitted changes included.
#
#   scripts/test-pre-commit.sh             the hook concourse-lint (golang)
#   scripts/test-pre-commit.sh system      the hook concourse-lint-system;
#                                          concourse-lint must be on the PATH
#
# Needs pre-commit (or uvx) and Go.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
hook=concourse-lint
[ "${1:-}" = system ] && hook=concourse-lint-system

if command -v pre-commit >/dev/null; then
  pre_commit=(pre-commit)
elif command -v uvx >/dev/null; then
  pre_commit=(uvx pre-commit)
else
  echo "pre-commit test: neither pre-commit nor uvx is installed" >&2
  exit 2
fi

fail() {
  echo "pre-commit test failed: $*" >&2
  exit 1
}

project="$(mktemp -d)"
trap 'rm -rf "$project"' EXIT
cd "$project"
git init --quiet
git config user.email test@example.com
git config user.name test

mkdir ci
cat >ci/pipeline.yml <<'EOF'
jobs:
- name: test
  plan:
  - task: run
    config:
      platform: linux
      image_resource:
        type: registry-image
        source: {repository: busybox, tag: "1.37"}
      run: {path: "true"}
EOF
# Not a pipeline: the hook must not report it.
cat >ci/configmap.yml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata: {name: example}
EOF
git add . && git commit --quiet -m init

"${pre_commit[@]}" try-repo "$root" "$hook" --all-files || fail "$hook fails on a clean project"

cat >ci/broken.yml <<'EOF'
resources:
- name: repo
  type: git
  source: {uri: "https://example.com/repo.git"}
jobs:
- name: test
  plan:
  - get: rep
EOF
git add ci/broken.yml
if "${pre_commit[@]}" try-repo "$root" "$hook" --all-files; then
  fail "$hook passes a pipeline with an error"
fi

echo "pre-commit test passed: $hook"

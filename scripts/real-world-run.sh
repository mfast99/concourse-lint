#!/usr/bin/env bash
# Checks a collection of public Concourse pipelines (the real-world run).
#
# Usage:
#   scripts/real-world-run.sh [--repos FILE] [--jobs N]
#   scripts/real-world-run.sh --pin [--repos FILE]
#
# The repositories come from scripts/real-world-repos.txt: one
# "owner/repo commit" per line, "#" starts a comment. A line without a
# commit takes the default branch.
#
# Every repository is cloned once into the cache folder
# (${CONCOURSE_LINT_REAL_WORLD:-$XDG_CACHE_HOME/concourse-lint/real-world}),
# with only its *.yml and *.yaml files, at the given commit. Then the
# freshly built binary checks each repository as a folder, the way a user
# would: JSON per repository in <cache>/results/, a summary per rule on
# stdout. A crash or an exit code 2 (input could not be read) is listed at
# the end and gives exit code 1.
#
# --pin clones the repositories and prints the list with the commit of each
# clone, for pinning new lines in the list.
#
# Needs git, go and jq. Public repositories only; nothing is written into
# this repository.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
repos_file="$root/scripts/real-world-repos.txt"
jobs=8
pin=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --repos) repos_file="$2"; shift 2 ;;
    --jobs) jobs="$2"; shift 2 ;;
    --pin) pin=true; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

cache="${CONCOURSE_LINT_REAL_WORLD:-${XDG_CACHE_HOME:-$HOME/.cache}/concourse-lint/real-world}"
mkdir -p "$cache/repos" "$cache/results"

# clone_repo fetches one repository at one commit (or the default branch)
# with only its YAML files. An existing clone at the right commit is kept.
clone_repo() {
  local name="$1" commit="$2" cache="$3"
  local target="$cache/repos/${name/\//__}"
  if [[ -d "$target/.git" ]]; then
    local current
    current="$(git -C "$target" rev-parse HEAD 2>/dev/null || true)"
    if [[ -z "$commit" || "$current" == "$commit" ]]; then
      return 0
    fi
  fi
  rm -rf "${target:?}"
  mkdir -p "$target"
  git -C "$target" init -q
  git -C "$target" remote add origin "https://github.com/$name.git"
  git -C "$target" config core.sparseCheckout true
  git -C "$target" sparse-checkout set --no-cone '*.yml' '*.yaml' 2>/dev/null
  if ! GIT_TERMINAL_PROMPT=0 git -C "$target" fetch -q --depth 1 --filter=blob:none origin "${commit:-HEAD}" 2>/dev/null \
    || ! git -C "$target" checkout -q FETCH_HEAD 2>/dev/null; then
    echo "clone failed: $name ${commit:-HEAD}" >&2
    rm -rf "${target:?}"
    return 0
  fi
  # A YAML symlink whose target is no YAML file (BOSH "spec.yml -> spec")
  # dangles in the sparse checkout: add the targets inside the repository.
  local link resolved
  local targets=()
  while IFS= read -r -d '' link; do
    resolved="$(realpath -m --relative-to="$target" "$(dirname "$link")/$(readlink "$link")")"
    if [[ "$resolved" != ../* ]]; then
      targets+=("/$resolved")
    fi
  done < <(find "$target" -xtype l -print0)
  if [[ ${#targets[@]} -gt 0 ]]; then
    GIT_TERMINAL_PROMPT=0 git -C "$target" sparse-checkout add "${targets[@]}" 2>/dev/null || true
  fi
}
export -f clone_repo

repo_lines() {
  sed -e 's/#.*//' -e '/^[[:space:]]*$/d' "$repos_file"
}

repo_lines | while read -r name commit; do
  printf '%s\0%s\0%s\0' "$name" "${commit:-}" "$cache"
done | xargs -0 -n 3 -P "$jobs" bash -c 'clone_repo "$1" "$2" "$3"' _

if $pin; then
  repo_lines | while read -r name _; do
    target="$cache/repos/${name/\//__}"
    if [[ -d "$target/.git" ]]; then
      echo "$name $(git -C "$target" rev-parse HEAD)"
    fi
  done
  exit 0
fi

binary="$cache/concourse-lint"
(cd "$root" && go build -o "$binary" ./cmd/concourse-lint)

status=0
problems=()
repos=0
pipelines=0
rm -f "$cache/results/"*.json
while read -r name _; do
  target="$cache/repos/${name/\//__}"
  [[ -d "$target/.git" ]] || continue
  repos=$((repos + 1))
  result="$cache/results/${name/\//__}.json"
  # Roughly lint.LooksLikePipeline: a top-level jobs: and a plan:.
  count="$( (grep -rlZE --include='*.yml' --include='*.yaml' '^jobs[[:space:]]*:' "$target" 2>/dev/null \
    | xargs -0 -r grep -lE '(^|[[:space:]{,-])plan[[:space:]]*:' 2>/dev/null || true) | wc -l)"
  pipelines=$((pipelines + count))
  set +e
  (cd "$target" && "$binary" check --format json . <"/dev/null" >"$result" 2>"$result.stderr")
  code=$?
  set -e
  if [[ $code -ge 2 ]]; then
    problems+=("$name: exit code $code: $(head -c 300 "$result.stderr")")
    status=1
  fi
done < <(repo_lines)

echo "repositories: $repos, pipelines (about): $pipelines"
echo
echo "findings per rule:"
cat "$cache/results/"*.json | jq -r '.findings[] | "\(.rule) \(.severity)"' \
  | sort | uniq -c | awk '{ printf "  %-6s %-8s %6d\n", $2, $3, $1 }'
echo
echo "results: $cache/results"
if [[ ${#problems[@]} -gt 0 ]]; then
  echo
  echo "problems:"
  printf '  %s\n' "${problems[@]}"
fi
exit $status

#!/usr/bin/env bash
# Prints the release notes of a tag: its section in CHANGELOG.md, without the
# heading. The release workflow passes them to GoReleaser.
#
#   scripts/release-notes.sh v1.2.0 > notes.md
#
# A release vX.Y.Z needs a section "## [X.Y.Z]" with content, so that no
# release goes out without its changelog entry. A pre-release (v1.2.0-rc.1)
# takes its own section "## [1.2.0-rc.1]" if there is one, else the section
# of its version, else "## [Unreleased]".
# CHANGELOG (default: CHANGELOG.md of the repository) names another file,
# for the tests in scripts/test-release-notes.sh.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
changelog="${CHANGELOG:-$root/CHANGELOG.md}"
tag="${1:-}"

if ! [[ "$tag" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "release notes: \"$tag\" is not a version tag vX.Y.Z" >&2
  exit 2
fi
version="${BASH_REMATCH[1]}"
prerelease="${BASH_REMATCH[2]}"

# section <name>: the lines after "## [<name>]" up to the next "## " heading
# or the link definitions at the end ("[1.0.0]: https://…"), without the
# blank lines around them. Exits 1 if there is no such heading.
section() {
  awk -v heading="## [$1]" '
    index($0, heading) == 1 { found = 1; inside = 1; next }
    inside && (/^## / || /^\[[^]]+\]: /) { inside = 0 }
    inside { lines[++count] = $0 }
    END {
      if (!found) exit 1
      first = 1
      while (first <= count && lines[first] ~ /^[[:space:]]*$/) first++
      last = count
      while (last >= first && lines[last] ~ /^[[:space:]]*$/) last--
      for (line = first; line <= last; line++) print lines[line]
    }
  ' "$changelog"
}

name="$version"
# A pre-release with a section of its own ("## [1.2.0-rc.1]") gets that
# section.
if [ -n "$prerelease" ] && notes="$(section "$version$prerelease")"; then
  name="$version$prerelease"
elif ! notes="$(section "$version")"; then
  if [ -z "$prerelease" ]; then
    echo "release notes: no section [$version] in $changelog; move the items from [Unreleased] there" >&2
    exit 1
  fi
  name=Unreleased
  notes="$(section Unreleased)" || {
    echo "release notes: no section [$version] or [Unreleased] in $changelog" >&2
    exit 1
  }
fi

if [ -z "$notes" ]; then
  echo "release notes: the section [$name] in $changelog is empty" >&2
  exit 1
fi
printf '%s\n' "$notes"

#!/usr/bin/env bash
# Checks the release archives that GoReleaser writes to dist/release/
# (make release-snapshot, or the release workflow): one archive per target,
# each with the binary, LICENSE, NOTICE, THIRD_PARTY_LICENSES.txt and
# README.md, all of them listed in checksums.txt, and the binary of this
# machine prints its version.
#
#   scripts/check-release-archives.sh            any version
#   scripts/check-release-archives.sh v1.2.3     the archives of v1.2.3
#
# Needs tar, python3 (for the zip archives) and sha256sum.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
release="$root/dist/release"
expected_version="${1:-}"

fail() {
  echo "release check failed: $*" >&2
  exit 1
}

[ -d "$release" ] || fail "no dist/release; run make release-snapshot first"
[ -f "$release/checksums.txt" ] || fail "no checksums.txt in dist/release"

archive_version=""
if [ -n "$expected_version" ]; then
  archive_version="${expected_version#v}"
fi

workspace="$(mktemp -d)"
trap 'rm -rf "$workspace"' EXIT

targets=(linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 windows_amd64 windows_arm64)
host_os="$(go env GOOS)"
host_arch="$(go env GOARCH)"

for target in "${targets[@]}"; do
  binary=concourse-lint
  extension=tar.gz
  if [[ "$target" == windows_* ]]; then
    binary=concourse-lint.exe
    extension=zip
  fi
  if [ -n "$archive_version" ]; then
    archives=("$release/concourse-lint_${archive_version}_$target.$extension")
  else
    archives=("$release"/concourse-lint_*_"$target.$extension")
  fi
  [ -f "${archives[0]}" ] || fail "no archive for $target"
  [ "${#archives[@]}" -eq 1 ] || fail "more than one archive for $target: ${archives[*]}"
  archive="${archives[0]}"
  archive_name="$(basename "$archive")"

  grep -q "  $archive_name\$" "$release/checksums.txt" || fail "$archive_name is not in checksums.txt"

  unpacked="$workspace/$target"
  mkdir -p "$unpacked"
  if [ "$extension" = zip ]; then
    python3 -I -m zipfile -e "$archive" "$unpacked"
  else
    tar -xzf "$archive" -C "$unpacked"
  fi

  for file in "$binary" LICENSE NOTICE THIRD_PARTY_LICENSES.txt README.md; do
    [ -f "$unpacked/$file" ] || fail "$archive_name has no $file at its top level"
  done
  cmp -s "$unpacked/THIRD_PARTY_LICENSES.txt" "$root/cmd/concourse-lint/third_party_licenses.txt" ||
    fail "THIRD_PARTY_LICENSES.txt in $archive_name differs from cmd/concourse-lint/third_party_licenses.txt"
  cmp -s "$unpacked/LICENSE" "$root/LICENSE" || fail "LICENSE in $archive_name differs from LICENSE"
  cmp -s "$unpacked/NOTICE" "$root/NOTICE" || fail "NOTICE in $archive_name differs from NOTICE"

  if [ "$target" = "${host_os}_${host_arch}" ]; then
    # zipfile does not keep the executable bit; only tar archives are run.
    [ -x "$unpacked/$binary" ] || fail "$binary in $archive_name is not executable"
    version_output="$("$unpacked/$binary" version)"
    if [ -n "$expected_version" ]; then
      [ "$version_output" = "concourse-lint $expected_version" ] ||
        fail "$binary in $archive_name prints \"$version_output\", expected \"concourse-lint $expected_version\""
    else
      [[ "$version_output" == "concourse-lint v"* ]] ||
        fail "$binary in $archive_name prints \"$version_output\", expected a version vX.Y.Z…"
    fi
  fi
done

(cd "$release" && sha256sum --check --quiet checksums.txt) || fail "checksums.txt does not match the archives"
echo "release archives in dist/release are complete"

#!/usr/bin/env bash
# Installs concourse-lint for the GitHub Action (action.yml): the release
# archive of this runner, checked against checksums.txt and, for a download
# from GitHub, against its build provenance attestation; then the binary
# goes on the PATH of the following steps.
#
#   install.sh <version>        e.g. 0.1.0
#
# CONCOURSE_LINT_DOWNLOAD_URL replaces the release URL (a folder with the
# archives and checksums.txt, also file://), for the tests in CI; the
# attestation is then not checked.
set -euo pipefail
version="${1:?usage: install.sh <version>}"
repository=mfast99/concourse-lint

case "${RUNNER_OS:-}" in
  Linux) system=linux ;;
  macOS) system=darwin ;;
  Windows) system=windows ;;
  *) echo "::error::concourse-lint: unsupported runner OS ${RUNNER_OS:-unknown}"; exit 1 ;;
esac
case "${RUNNER_ARCH:-}" in
  X64) machine=amd64 ;;
  ARM64) machine=arm64 ;;
  *) echo "::error::concourse-lint: unsupported runner architecture ${RUNNER_ARCH:-unknown}"; exit 1 ;;
esac
extension=tar.gz
binary=concourse-lint
if [ "$system" = windows ]; then
  extension=zip
  binary=concourse-lint.exe
fi
archive="concourse-lint_${version}_${system}_${machine}.${extension}"
base_url="${CONCOURSE_LINT_DOWNLOAD_URL:-https://github.com/$repository/releases/download/v$version}"

work="$(mktemp -d "${RUNNER_TEMP:-/tmp}/concourse-lint.XXXXXX")"
cd "$work"
curl --fail --silent --show-error --location --retry 3 --output checksums.txt "$base_url/checksums.txt"
curl --fail --silent --show-error --location --retry 3 --output "$archive" "$base_url/$archive"

expected="$(awk -v name="$archive" '$2 == name {print $1}' checksums.txt)"
if [ -z "$expected" ]; then
  echo "::error::concourse-lint: $archive is not in checksums.txt"
  exit 1
fi
if command -v sha256sum >/dev/null; then
  actual="$(sha256sum "$archive" | cut -d' ' -f1)"
else
  actual="$(shasum -a 256 "$archive" | cut -d' ' -f1)"
fi
if [ "$actual" != "$expected" ]; then
  echo "::error::concourse-lint: the checksum of $archive does not match checksums.txt"
  exit 1
fi

# The attestation proves that the release workflow of the repository built
# the archive. Skipped for a test download.
if [ -z "${CONCOURSE_LINT_DOWNLOAD_URL:-}" ]; then
  gh attestation verify "$archive" --repo "$repository" >/dev/null
fi

mkdir bin
# The tar of Git Bash on Windows is GNU tar, which cannot read zip; the tar
# of Windows itself is bsdtar, which can.
if [ "$extension" = zip ]; then
  "$SYSTEMROOT/System32/tar.exe" -xf "$archive" -C bin "$binary"
else
  tar -xf "$archive" -C bin "$binary"
fi
chmod +x "bin/$binary"
echo "$work/bin" >>"$GITHUB_PATH"
echo "concourse-lint $version installed"

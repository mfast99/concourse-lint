#!/usr/bin/env bash
# Builds the npm package concourse-lint with the binaries of all targets
# inside (bin/<platform>-<arch>/, named like process.platform and
# process.arch). One package for every platform, so that it also works with
# --omit=optional, lockfiles from another system and node_modules copied
# into a container. Needs Go and Node.
#
# The package lands in dist/npm/concourse-lint-<version>.tgz. VERSION
# (default: git describe) goes into the binaries; the package version is
# VERSION without the leading "v" when it is a release version, otherwise
# 0.0.0-dev.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
output="$root/dist/npm"
staging="$output/package"

targets=(linux-x64 linux-arm64 darwin-x64 darwin-arm64 win32-x64 win32-arm64)
version="${VERSION:-$(git -C "$root" describe --tags --always --dirty 2>/dev/null || echo dev)}"
package_version="${version#v}"
if ! [[ "$package_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  package_version=0.0.0-dev
fi

(cd "$root/npm" && npm test)

rm -rf "$output"
mkdir -p "$staging"
cp -R "$root/npm/bin" "$root/npm/lib" "$root/npm/package.json" "$root/npm/README.md" "$staging/"
cp "$root/LICENSE" "$root/NOTICE" "$staging/"
# The license texts of the third-party software in the binaries. The package
# has no npm dependencies, so the text of the binary is the whole list; CI
# checks that it is up to date (scripts/third-party-licenses.sh --check).
cp "$root/cmd/concourse-lint/third_party_licenses.txt" "$staging/THIRD_PARTY_LICENSES.txt"

for target in "${targets[@]}"; do
  case "$target" in
    linux-x64) goos=linux goarch=amd64 ;;
    linux-arm64) goos=linux goarch=arm64 ;;
    darwin-x64) goos=darwin goarch=amd64 ;;
    darwin-arm64) goos=darwin goarch=arm64 ;;
    win32-x64) goos=windows goarch=amd64 ;;
    win32-arm64) goos=windows goarch=arm64 ;;
  esac
  binary=concourse-lint
  [ "$goos" = windows ] && binary=concourse-lint.exe
  (cd "$root" && CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$staging/bin/$target/$binary" ./cmd/concourse-lint)
done

cd "$staging"
npm pkg set "version=$package_version"
npm pack --pack-destination "$output"
cd "$root"
rm -rf "$staging"
echo "package in $output"

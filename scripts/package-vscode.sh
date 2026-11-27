#!/usr/bin/env bash
# Builds the VS Code extension with the concourse-lint binary inside, one
# package per platform. Needs Go, Node and pnpm.
#
#   scripts/package-vscode.sh                 all platforms
#   scripts/package-vscode.sh linux-x64       only the ones named
#
# The packages land in dist/vscode/concourse-lint-<target>.vsix. VERSION
# (default: git describe) goes into the binary; the extension version is the
# one in editors/vscode/package.json.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
extension="$root/editors/vscode"
output="$root/dist/vscode"

all_targets=(linux-x64 linux-arm64 darwin-x64 darwin-arm64 win32-x64 win32-arm64)
targets=("${@:-${all_targets[@]}}")
version="${VERSION:-$(git -C "$root" describe --tags --always --dirty 2>/dev/null || echo dev)}"

cd "$extension"
pnpm install --frozen-lockfile
pnpm run check
pnpm test
pnpm run build
cp "$root/LICENSE" "$root/NOTICE" .
# THIRD_PARTY_LICENSES.txt goes into every package: it must match the
# dependencies that are built in.
"$root/scripts/third-party-licenses.sh" --check
mkdir -p "$output"

for target in "${targets[@]}"; do
  case "$target" in
    linux-x64) goos=linux goarch=amd64 ;;
    linux-arm64) goos=linux goarch=arm64 ;;
    darwin-x64) goos=darwin goarch=amd64 ;;
    darwin-arm64) goos=darwin goarch=arm64 ;;
    win32-x64) goos=windows goarch=amd64 ;;
    win32-arm64) goos=windows goarch=arm64 ;;
    *) echo "unknown target $target (known: ${all_targets[*]})" >&2; exit 2 ;;
  esac
  binary=concourse-lint
  [ "$goos" = windows ] && binary=concourse-lint.exe
  rm -rf bin
  mkdir bin
  (cd "$root" && CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$extension/bin/$binary" ./cmd/concourse-lint)
  pnpm exec vsce package --no-dependencies --target "$target" --out "$output/concourse-lint-$target.vsix"
done
rm -rf bin
echo "packages in $output"

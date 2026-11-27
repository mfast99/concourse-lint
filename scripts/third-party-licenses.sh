#!/usr/bin/env bash
# Collects the license texts of the third-party software that concourse-lint
# ships, from the Go module cache and the node_modules of the extension.
# Needs Go, Node, pnpm and jq.
#
#   scripts/third-party-licenses.sh           write the files
#   scripts/third-party-licenses.sh --check   fail if a file is out of date
#
# It writes two files:
#
#   cmd/concourse-lint/third_party_licenses.txt
#       the Go modules linked into the binary and the Go standard library;
#       embedded in the binary (concourse-lint licenses); the npm package
#       ships it as THIRD_PARTY_LICENSES.txt (scripts/package-npm.sh)
#   editors/vscode/THIRD_PARTY_LICENSES.txt
#       the npm packages bundled into extension.js, then the text above for
#       the binary the extension contains
#
# Run it after every dependency update (Dependabot); CI runs --check.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
extension="$root/editors/vscode"
binary_file="$root/cmd/concourse-lint/third_party_licenses.txt"
extension_file="$extension/THIRD_PARTY_LICENSES.txt"

mode=write
case "${1:-}" in
  "") ;;
  --check) mode=check ;;
  *) echo "usage: $0 [--check]" >&2; exit 2 ;;
esac

separator="================================================================================"

# license_files prints the license and notice files of a package folder,
# sorted, one per line.
license_files() {
  find "$1" -maxdepth 1 -type f \
    \( -iname 'licen[cs]e*' -o -iname 'copying*' -o -iname 'notice*' -o -iname 'thirdpartynotices*' \) \
    | LC_ALL=C sort
}

# section prints one package: a header, then the texts of its license files.
# It fails when the package has no license file.
section() {
  local title="$1" folder="$2" files
  files="$(license_files "$folder")"
  if [ -z "$files" ]; then
    echo "no license file in $folder ($title)" >&2
    return 1
  fi
  printf '%s\n%s\n%s\n' "$separator" "$title" "$separator"
  while IFS= read -r file; do
    printf '\n'
    cat "$file"
    # A text without a final newline would run into the next header.
    [ -n "$(tail -c 1 "$file")" ] && printf '\n'
  done <<< "$files"
  printf '\n'
}

binary_text() {
  cat <<'EOF'
Third-party software in the concourse-lint binary

concourse-lint itself is licensed under the Apache License, Version 2.0 (see
LICENSE and NOTICE in https://github.com/mfast99/concourse-lint). The binary
contains the following third-party software, each under its own license.

EOF
  # Some Linux packages of Go move its LICENSE out of GOROOT (Arch Linux:
  # /usr/share/licenses/go).
  local go_license_folder
  go_license_folder="$(go env GOROOT)"
  if [ ! -f "$go_license_folder/LICENSE" ] && [ -f /usr/share/licenses/go/LICENSE ]; then
    go_license_folder=/usr/share/licenses/go
  fi
  section "Go standard library and runtime (https://go.dev)" "$go_license_folder"
  # Every module of a package linked into the binary, except our own. A
  # replaced module is listed with its replacement, whose folder holds the
  # code that is actually built.
  (cd "$root" && go mod download && go list -deps \
    -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{with .Replace}} (replaced by {{.Path}} {{.Version}}){{end}}|{{.Dir}}{{end}}{{end}}' \
    ./cmd/concourse-lint) | LC_ALL=C sort -u | while IFS='|' read -r title folder; do
    section "$title" "$folder"
  done
}

extension_text() {
  cat <<'EOF'
Third-party software in the concourse-lint extension

The extension is licensed under the Apache License, Version 2.0 (see LICENSE
and NOTICE). Its extension.js contains the following npm packages, each under
its own license. After them follow the licenses for the concourse-lint
binary that the extension contains.

EOF
  (cd "$extension" && pnpm install --frozen-lockfile --silent >&2 && pnpm licenses list --prod --json) \
    | jq -r '[.[][]] | sort_by(.name) | .[] | .versions as $versions | .paths | to_entries[] | "\(.value)|\($versions[.key])|\(.value | sub(".*/node_modules/"; ""))"' \
    | while IFS='|' read -r folder version name; do
      license="$(jq -r '.license // "see the license text"' "$folder/package.json")"
      section "$name $version (npm, $license)" "$folder"
    done
  printf '\n'
  cat "$binary_file.new"
}

binary_text > "$binary_file.new"
extension_text > "$extension_file.new"

status=0
for file in "$binary_file" "$extension_file"; do
  if [ "$mode" = check ]; then
    if ! cmp -s "$file.new" "$file"; then
      echo "${file#"$root/"} is out of date: run scripts/third-party-licenses.sh" >&2
      diff -u "$file" "$file.new" >&2 || true
      status=1
    fi
    rm -f "$file.new"
  else
    mv "$file.new" "$file"
    echo "wrote ${file#"$root/"}"
  fi
done
exit "$status"

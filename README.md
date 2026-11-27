# concourse-lint

[![ci](https://github.com/mfast99/concourse-lint/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/mfast99/concourse-lint/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Linter and language server for [Concourse](https://concourse-ci.org) pipelines.
Finds errors, security issues and bad practice **before** `fly set-pipeline`
runs: in your editor, in CI and for AI agents. One static Go binary, no
Concourse installation needed.

## What it checks

37 rules in three areas. `concourse-lint rules` lists them with their
severity in your project, `concourse-lint explain <ID>` shows what a rule
checks, an example and how to fix it. The full list is also in
[spec/README.md](spec/README.md#rules).

| Area | IDs | Examples |
|------|-----|----------|
| Correctness | `CL000`–`CL024` | YAML syntax, unknown fields, missing resources or jobs, `passed:` cycles, artifacts that don't exist at that point, invalid suppression comments, jobs missing from groups, numbers that silently change, stray tabs in strings, templates and fragments that need rendering first |
| Security | `CS001`–`CS009` | plaintext secrets, known token formats (also in comments), disabled TLS, unpinned images, privileged tasks, public build logs |
| Best practice | `CB001`–`CB003` | unused resource types, too frequent `check_every`, jobs without steps |

Findings can be suppressed per line or per file with a justification:

```yaml
privileged: true  # concourse-lint:ignore CS006 -- oci-build-task needs this
```

## Install

* **Release archives:** download the archive for your platform from the
  [releases](https://github.com/mfast99/concourse-lint/releases) (see
  "Release archives" below) and put `concourse-lint` on your `PATH`.
* **Homebrew** (macOS and Linux): `brew install mfast99/tap/concourse-lint`
* **Container image:** `ghcr.io/mfast99/concourse-lint` for linux/amd64
  and linux/arm64 (see "Container image" below).
* **Go:** `go install github.com/mfast99/concourse-lint/cmd/concourse-lint@latest`
* **npm** (Node 18 or newer, binary included, also as a `devDependency`):
  `npx concourse-lint check pipeline.yml`

## Usage

```sh
concourse-lint check pipeline.yml
concourse-lint check ci/                            # every pipeline in a folder
concourse-lint check --format json pipeline.yml     # for AI agents and scripts
concourse-lint check --format sarif pipeline.yml    # for GitHub Code Scanning
concourse-lint check --format github ci/            # annotations in GitHub Actions
concourse-lint check --fail-on warning -            # read from stdin
concourse-lint rules                                # all rules, severities from the config
concourse-lint explain CS001                        # what a rule checks and how to fix it
concourse-lint lsp                                  # language server (stdio)
concourse-lint mcp                                  # MCP server for AI agents (stdio)
```

Exit codes: `0` nothing at or above `--fail-on` (default `error`), `1` findings,
`2` usage error, a broken config file or an input that could not be read.

A `.concourse-lint.yml` in the folder of a pipeline or above it sets rule
severities, options and excluded paths (see
[spec/README.md](spec/README.md#config-file)).

### Release archives

Every [release](https://github.com/mfast99/concourse-lint/releases) has an
archive for Linux, macOS and Windows (amd64 and arm64) with the binary,
`LICENSE`, `NOTICE` and the licenses of the third-party software, plus
`checksums.txt`. Each archive has a build provenance attestation: it proves
that the release workflow of this repository built it from the tagged
commit. Check it with the [GitHub CLI](https://cli.github.com/) before you
unpack it:

```sh
gh attestation verify concourse-lint_0.2.0_linux_amd64.tar.gz --repo mfast99/concourse-lint
# or at least the checksum
sha256sum --check --ignore-missing checksums.txt
```

## Container image

`ghcr.io/mfast99/concourse-lint` has the tags `1.2.3`, `1.2`, `1` and
`latest`. It is based on Alpine, so CI systems that run scripts with `sh`
can use it; it runs as an unprivileged user with `/work` as working
directory and `concourse-lint` as entrypoint.

```sh
docker run --rm -v "$PWD:/work" ghcr.io/mfast99/concourse-lint:1 check ci/
```

GitLab CI:

```yaml
lint-pipelines:
  image:
    name: ghcr.io/mfast99/concourse-lint:1
    entrypoint: [""]
  script:
    - concourse-lint check ci/
```

A Concourse task:

```yaml
- task: lint-pipelines
  config:
    platform: linux
    image_resource:
      type: registry-image
      source: {repository: ghcr.io/mfast99/concourse-lint, tag: "1"}
    inputs: [{name: repo}]
    run: {path: concourse-lint, args: [check, repo/ci]}
```

Every image has a build provenance attestation:
`gh attestation verify oci://ghcr.io/mfast99/concourse-lint:1 --repo mfast99/concourse-lint`.

## GitHub Actions

`--format github` shows the findings as annotations on the changed lines of
a pull request, without code scanning, and adds a table of all findings to
the summary of the run (GitHub shows only 10 annotations per level and step
and 50 per job; the summary lists all of them):

```yaml
- uses: mfast99/concourse-lint-action@v0.2.0
  with:
    args: check --format github ci/   # default: check --format github .
```

The [action](https://github.com/mfast99/concourse-lint-action) installs the
release archive for the runner after checking its checksum and attestation;
its version is the concourse-lint version. With concourse-lint already
installed, run `concourse-lint check --format github ci/` directly.

Paths are reported relative to the repository root, also when the step
runs in a subfolder.

## pre-commit

```yaml
repos:
  - repo: https://github.com/mfast99/concourse-lint
    rev: v0.2.0
    hooks:
      - id: concourse-lint          # builds concourse-lint with Go
      # - id: concourse-lint-system # or uses the concourse-lint on the PATH
```

The hook runs when a YAML file changed and checks the whole folder, so that
concourse-lint can tell pipelines from other YAML files; `args: [check, ci/]`
checks only `ci/`.

## Editor

The VS Code extension in [`editors/vscode/`](editors/vscode/) brings the
binary along and shows findings while you type (also in VSCodium), with
go to definition, references, hover, completion and quick fixes. Install
`mfast99.concourse-lint` from the
[Visual Studio Marketplace](https://marketplace.visualstudio.com/items?itemName=mfast99.concourse-lint)
or [Open VSX](https://open-vsx.org/extension/mfast99/concourse-lint), or
the `.vsix` for your platform from the [releases](https://github.com/mfast99/concourse-lint/releases) with
`code --install-extension <file>.vsix`. `make vscode` builds it yourself
(needs Go, Node and pnpm); the packages land in `dist/vscode/`.
Sublime Text has the package [`editors/sublime/`](editors/sublime/); Helix,
Emacs and other editors start `concourse-lint lsp` as a language server, see
[docs/editors.md](docs/editors.md).

## AI agents

`concourse-lint mcp` is an MCP server with the tools `check_pipeline`,
`list_rules` and `explain_rule`, for Claude Code, Claude Desktop, VS Code and
other MCP clients:

```sh
claude mcp add concourse-lint -- concourse-lint mcp
```

Setup for other clients and details: [docs/mcp.md](docs/mcp.md). Agents with
a shell can also use `concourse-lint check --format json`.

## Repository layout

| Folder | Contents |
|--------|----------|
| `spec/` | Specification test cases: pipelines with expected findings, rule catalog |
| `internal/spectest/` | Test runner: reads `spec/` and compares |
| `internal/rules/` | Logic: one file per rule, grouped by area |
| `internal/parse/`, `internal/model/` | Reading YAML, pipeline model with positions |
| `internal/lint/` | Entry point: parsing, rules, suppression |
| `internal/report/` | Output as text, JSON, SARIF, GitHub annotations |
| `internal/lsp/`, `internal/mcp/` | Language server and MCP server |
| `editors/vscode/` | VS Code extension (TypeScript), starts `concourse-lint lsp` |
| `editors/sublime/` | Sublime Text package `LSP-concourse-lint` (Python) |
| `npm/` | npm package `concourse-lint` with the binaries of all platforms (`make npm`) |
| `cmd/concourse-lint/` | The program itself and black-box tests |
| `docs/rules/` | One page per rule |
| `docs/mcp.md` | Setting up the MCP server for AI agents |
| `docs/editors.md` | Setting up the language server in Helix, Emacs and other editors |

## Contributing

Contributions are welcome, including test cases only: no Go needed for those.
Every change starts with an issue and goes through a pull request against
`develop`. See [CONTRIBUTING.md](CONTRIBUTING.md).

```sh
make check    # fmt + vet + staticcheck + test
```

## Security

Please report vulnerabilities privately, see [SECURITY.md](SECURITY.md).

## License

[Apache-2.0](LICENSE). Redistributions must keep the [NOTICE](NOTICE) file.

The binary and the VS Code extension contain third-party software under its
own licenses: `concourse-lint licenses` prints them, and the extension ships
them as `THIRD_PARTY_LICENSES.txt`.

"Concourse" is a trademark of its respective owner. concourse-lint is an
independent project and is not affiliated with or endorsed by the Concourse
project.

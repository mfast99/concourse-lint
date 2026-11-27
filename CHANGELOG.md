# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-11-27

### Added

- `concourse-lint check`: 37 rules for Concourse 8.x pipelines in three
  areas, correctness (`CL000`–`CL024`), security (`CS001`–`CS009`) and best
  practice (`CB001`–`CB003`), with the exact position of every finding and
  a hint how to fix it. A tolerant parser keeps checking after the first
  error.
- Files and folders as input, or stdin; ytt and spruce templates and
  pipeline fragments are recognized instead of reported as broken
  pipelines.
- Output as text, JSON, SARIF and GitHub annotations (`--format github`,
  with a step summary), `--fail-on` for the exit code.
- Suppression comments per line or per file, with a justification.
- Config file `.concourse-lint.yml`: severities, rule options, resource
  types of the workers, excluded paths.
- `concourse-lint rules` and `concourse-lint explain <ID>`, with a page per
  rule in `docs/rules/`.
- Language server (`concourse-lint lsp`): findings while you type, go to
  definition, references, hover, completion and quick fixes.
- VS Code extension with the binary inside, a Sublime Text package and
  setup docs for Helix, Emacs and other LSP clients.
- MCP server for AI agents (`concourse-lint mcp`) with the tools
  `check_pipeline`, `list_rules` and `explain_rule`.
- Release archives for Linux, macOS and Windows (amd64 and arm64) with
  checksums and build provenance attestations, and an npm package with the
  binaries of all platforms.
- Container image `ghcr.io/mfast99/concourse-lint` (Alpine, linux/amd64
  and linux/arm64) with a build provenance attestation.
- Homebrew formula: `brew install mfast99/tap/concourse-lint`.
- GitHub Action `mfast99/concourse-lint-action` that installs the release
  binary, checks its attestation and reports findings as annotations.
- pre-commit hooks `concourse-lint` (built with Go) and
  `concourse-lint-system` (the binary on the `PATH`).
- The Sublime Text package downloads the binary of its version from the
  release and checks its checksum.
- The VS Code extension on the Visual Studio Marketplace and Open VSX, and
  setup docs for Neovim.

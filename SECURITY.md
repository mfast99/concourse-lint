# Security Policy

## Reporting a vulnerability

Please **do not open a public issue** for security problems.

Report them privately via GitHub:
[Report a vulnerability](https://github.com/mfast99/concourse-lint/security/advisories/new)
(Security tab → "Report a vulnerability").

Please include:

* the affected version or commit,
* a description of the issue and its impact,
* steps or a pipeline file to reproduce it (with real secrets removed).

You can expect a first response within 7 days. We will keep you informed about
the fix and credit you in the advisory unless you prefer to stay anonymous.

## What counts as a vulnerability

concourse-lint reads untrusted pipeline files. Examples of what we treat as
security issues:

* crashes, hangs or excessive memory use on crafted input (e.g. YAML alias
  bombs),
* secrets from the analyzed pipeline leaking into places they should not go
  (logs, SARIF uploads, MCP responses) beyond the location that is reported,
* code execution or file access outside the analyzed file.

A security **rule** that misses a problem (e.g. a token format CS002 does not
know) is a regular bug or rule proposal, not a vulnerability. Please open a
normal issue for it.

## Supported versions

Until 1.0.0, only the latest release receives fixes.

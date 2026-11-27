# concourse-lint-action

Checks [Concourse](https://concourse-ci.org) pipelines with
[concourse-lint](https://github.com/mfast99/concourse-lint) in GitHub
Actions: errors, security issues and bad practice, as annotations on the
changed lines of a pull request and as a table in the summary of the run.

```yaml
jobs:
  concourse-lint:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v7
      - uses: mfast99/concourse-lint-action@v0.2.0
```

The version of the action is the version of concourse-lint it installs.
Pin it to a version or a commit SHA; Dependabot updates both.

## Inputs

| Input | Default | Meaning |
|---|---|---|
| `args` | `check --format github .` | The arguments of concourse-lint, e.g. `check --format github --fail-on warning ci/` |
| `version` | the version of the action | Another concourse-lint version to install |
| `token` | `${{ github.token }}` | For `gh attestation verify` |

The action downloads the release archive for the runner (Linux, macOS,
Windows; x64 and ARM64), checks it against `checksums.txt` and its build
provenance attestation, and puts `concourse-lint` on the `PATH`, so later
steps can use it too. A `.concourse-lint.yml` in the repository applies.

This repository is written by the release workflow of concourse-lint; the
source is in [`action/`](https://github.com/mfast99/concourse-lint/tree/main/action).
Issues and pull requests go to
[mfast99/concourse-lint](https://github.com/mfast99/concourse-lint).

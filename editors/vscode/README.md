# Concourse Lint for VS Code

Finds errors, security issues and bad practice in
[Concourse CI](https://concourse-ci.org) pipelines while you type, before
`fly set-pipeline` runs.

The extension contains the `concourse-lint` binary for your platform; you
need nothing else (no Go, no `fly`).

## What it does

* Every open YAML file that looks like a pipeline (a top-level `jobs:` and a
  `plan:`) is checked on every change. Task files, var files and other YAML
  are left alone.
* A file that does not look like a pipeline yet can be marked as one: click
  the language in the status bar and pick **Concourse Pipeline**.
* Each finding links to the page of its rule (click the rule ID).
* A `.concourse-lint.yml` next to the pipeline or above it (up to the Git
  root) switches rules off, changes severities and excludes files, exactly as
  for the CLI. Errors in it are shown in the file itself.

## Settings

| Setting | Default | |
|---|---|---|
| `concourse-lint.path` | empty | Use another `concourse-lint` binary (only in user settings). Empty: the bundled one, else `concourse-lint` from `PATH`. |
| `concourse-lint.trace.server` | `off` | Log the messages to the language server in the output panel. |

Command: **Concourse Lint: Restart Language Server**.

## Suppressing a finding

```yaml
privileged: true  # concourse-lint:ignore CS006 -- needs Docker in Docker
```

See the rule pages and the
[specification](https://github.com/mfast99/concourse-lint/blob/main/spec/README.md)
for all rules and options.

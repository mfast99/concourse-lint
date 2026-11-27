# concourse-lint for AI agents (MCP)

`concourse-lint mcp` is a [Model Context Protocol](https://modelcontextprotocol.io)
server. An AI agent (Claude Code, Claude Desktop, VS Code, Cursor, …) can
check Concourse pipelines through it with a tool call, offline, with the same
findings as the CLI. The contract is in
[spec/README.md, "MCP server contract"](../spec/README.md#mcp-server-contract).

Agents with a shell can also run `concourse-lint check --format json`. The
MCP server adds: clients without a shell, checking YAML the agent just wrote
without a temporary file, and rule explanations.

## Tools

| Tool | Arguments | Result |
|---|---|---|
| `check_pipeline` | `text` (the YAML, optionally with `filename`) **or** `paths` (files or folders) | the JSON of `concourse-lint check --format json`: `findings` and `errors` |
| `list_rules` | none | the JSON of `concourse-lint rules --format json`: every rule with ID, severity in this project, default severity, title and link |
| `explain_rule` | `id`, e.g. `CL002` | the JSON of `concourse-lint explain --format json <id>`: title, severities, what the rule checks, an example, how to fix and how to suppress it |

All three only read. Each finding has a `suggestion`: when it is not empty,
it is the exact text that replaces the finding's range (a "did you mean"
fix).

## The working directory

The server runs in the folder its client starts it in, usually the project.
`paths` (and `filename`) must lie inside that folder, also after following
symlinks; anything else is refused. The nearest `.concourse-lint.yml` applies
as in the CLI. If a client starts the server somewhere else, pass the YAML as
`text`.

## Setup

The binary has to be on your `PATH` (or give its full path below).

**Claude Code**, for the current project only:

```sh
claude mcp add concourse-lint -- concourse-lint mcp
```

or for everyone working on the project, in `.mcp.json` at its root:

```json
{
  "mcpServers": {
    "concourse-lint": {"command": "concourse-lint", "args": ["mcp"]}
  }
}
```

**Claude Desktop**, in `claude_desktop_config.json` (Settings, Developer,
Edit Config):

```json
{
  "mcpServers": {
    "concourse-lint": {"command": "/usr/local/bin/concourse-lint", "args": ["mcp"]}
  }
}
```

Claude Desktop does not start the server in a project folder, so use
`text` there.

**VS Code**, in `.vscode/mcp.json`:

```json
{
  "servers": {
    "concourse-lint": {"type": "stdio", "command": "concourse-lint", "args": ["mcp"]}
  }
}
```

Other clients take the same command: `concourse-lint mcp`, transport stdio.

## Troubleshooting

* The server writes its log to stderr; most clients show it in their MCP log.
* A broken `.concourse-lint.yml` makes `check_pipeline` fail with the
  position of the error (`file:line:col: message`) instead of checking with
  the defaults.
* `concourse-lint mcp` takes no arguments; it ends when the client closes
  stdin.

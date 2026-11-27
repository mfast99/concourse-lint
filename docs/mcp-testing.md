# Checking the MCP server by hand

The MCP server is tested in Go (`internal/mcp` against every case in
`spec/`, and `cmd/concourse-lint` with the binary). This checklist covers
what only a real client shows. Run it before a release and after changes
in `internal/mcp/`.

1. Build the binary and register it in Claude Code, from the repository
   root:

   ```sh
   make build
   claude mcp add concourse-lint-dev -- "$PWD/bin/concourse-lint" mcp
   claude mcp list        # concourse-lint-dev: … ✓ Connected
   ```

   After a rebuild, restart Claude Code (or reconnect in `/mcp`): the
   running server keeps the old binary.

2. Start `claude` in the repository root. `/mcp` shows `concourse-lint-dev`
   with the tools `check_pipeline`, `list_rules` and `explain_rule`.

3. Ask: *"Check spec/rules/correctness/CL002/get-put.yml with
   concourse-lint."* The agent calls `check_pipeline` with `paths`. The
   findings are those of
   `bin/concourse-lint check --format json spec/rules/correctness/CL002/get-put.yml`
   (CL002 on the `# want` lines, with the suggestions `repo` and
   `image`).

4. Ask: *"Check the folder spec/config/exclude."* Files under `generated/`
   and `templates/` are not checked (the config there excludes them).

5. Ask: *"Write a small Concourse pipeline with a get of a resource that is
   misspelled, and check it with concourse-lint without saving it."* The
   agent calls `check_pipeline` with `text`; the result has a CL002 with a
   `suggestion`, and the agent fixes the name.

6. Ask: *"What does concourse-lint rule cs006 check?"* The agent calls
   `explain_rule`; the answer matches `docs/rules/CS006.md`.

7. Ask: *"Which rules does concourse-lint have?"* `list_rules` gives 35
   rules.

8. Ask: *"Check /etc/hosts with concourse-lint."* The tool refuses: the
   path is outside the working directory; nothing is read.

9. Config: in a folder outside this repository (so that `spec/` is not
   affected), `git init`, put a pipeline with a CL002 and a
   `.concourse-lint.yml` with `rules: {CL002: fatal}`, start `claude` there
   (the server from step 1 is registered per folder, so add it there too)
   and ask for a check. The tool reports the config error at `fatal`.
   Change it to `rules: {CL002: off}`: the CL002 is gone.

10. Clean up: `claude mcp remove concourse-lint-dev` (in each folder where
    you added it).

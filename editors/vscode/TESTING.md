# Checking the extension by hand

The language server is tested in Go (`internal/lsp`, against every case in
`spec/`). This checklist covers what only a real editor shows. Run it before
a release and after changes in `editors/vscode/`.

1. Build and install the package for your platform:

   ```sh
   scripts/package-vscode.sh linux-x64        # or darwin-arm64, win32-x64, …
   codium --install-extension dist/vscode/concourse-lint-linux-x64.vsix
   # VS Code: code --install-extension …
   ```

   If the editor is already open, run **Developer: Reload Window** (or
   **Concourse Lint: Restart Language Server**) afterwards: the version does
   not change, so the running language server keeps the old binary.

2. Open the repository folder and these files:

   | File | Expected |
   |---|---|
   | `spec/rules/correctness/CL002/get-put.yml` | squiggles exactly on the lines with `# want CL002`, covering only `rep` and `imgae` |
   | `spec/rules/security/CS008/basic.yml` | warnings (yellow) on the `# want` lines |
   | `spec/syntax/bad-indent.yml` (no `plan:`) | nothing until you pick **Concourse Pipeline** in the status bar, then the CL000 |
   | any task file, e.g. a `platform:`/`run:` file | nothing |

3. Type in `get-put.yml`: change `repo` in line 15 to `rpo` and back. The
   squiggle appears and disappears after a short pause, without flicker.
4. Hover a squiggle: message, `concourse-lint(CL002)`, the rule ID links to
   `docs/rules/CL002.md`.
5. Config: create `.concourse-lint.yml` next to an open pipeline with
   `rules: {CL002: off}` and save. The CL002 squiggles disappear. Change it to
   `rules: {CL002: fatal}`: the config file shows an error at `fatal`, the
   pipeline shows one info "not checked". Delete the file: findings are back.
6. Setting `concourse-lint.path` to a file that does not exist shows an error
   message; empty again uses the bundled binary (output panel "Concourse Lint").
7. **Concourse Lint: Restart Language Server** works without reloading the
   window.
8. Navigation in `spec/editor/navigation/jobs.yml`: F12 on `build` in
   `passed: [build]` jumps to `- name: build`; F12 on `"deploy-*"` offers
   both deploy jobs; Shift+F12 on `- name: build` lists the two `passed:`
   entries and the two group entries.
9. Hover in `spec/editor/hover/resources.yml` on `repo`: "resource `repo`"
   with type, check_every and the jobs. Hover on `rep` in
   `spec/rules/correctness/CL002/get-put.yml`: the rule explanation with
   "Fix" and a link to the rule page, rendered as Markdown.
10. Quick fixes in `spec/editor/quickfix/did-you-mean.yml`: the light bulb
    on `rep` offers 'Change to "repo"' (Ctrl+. applies it), "Suppress CL002
    on this line" and "... in this file"; each removes the squiggle.
11. Completion (Ctrl+Space) in a new pipeline: below `- get: repo` the keys
    of a get step (`trigger`, `passed`, …), after `get: ` the resources,
    in `passed: [` the other jobs. Break the YAML (e.g. an open `[`): keys
    and names are still offered.

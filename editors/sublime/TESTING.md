# Checking the package by hand

`plugin.py` has unit tests (`make sublime-test`); the server is tested in Go.
This checklist covers what only Sublime Text shows. Run it before a release
and after changes in `editors/sublime/`.

1. Install the package **LSP** (2.12 or newer) with Package Control, build the
   binary and link the package (see [README.md](README.md)):

   ```sh
   make build && export PATH="$PWD/bin:$PATH"
   ln -s "$PWD/editors/sublime" ~/.config/sublime-text/Packages/LSP-concourse-lint
   ```

   Start Sublime Text from that shell, so that it sees the `PATH`, and open
   the repository folder (**File › Open Folder…**). If the package was linked
   before LSP was installed, restart Sublime Text once: the package could not
   load without LSP.

2. Open these files:

   | File | Expected |
   |---|---|
   | `spec/rules/correctness/CL002/get-put.yml` | errors exactly on the lines with `# want CL002`, covering only `rep` and `imgae` |
   | `spec/rules/security/CS008/basic.yml` | warnings on the `# want` lines |
   | any task file, e.g. a `platform:`/`run:` file | nothing |

   **LSP: Toggle Log Panel** shows `concourse-lint` among the servers.
3. Type in `get-put.yml`: change `repo` in line 15 to `rpo` and back. The
   error appears and disappears after a short pause.
4. Hover `rep` in line 13: the message with `CL002` and the rule page.
   Hover `repo` in line 15: a summary of the resource.
5. **LSP: Goto Definition** (command palette or the context menu **LSP**) on
   `repo` in line 15 jumps to line 3; **LSP: Code Action** on `rep` in line 13
   offers `Change to "repo"`. F12 is Sublime Text's own go to definition
   unless you bound it to LSP (see [README.md](README.md)); it answers
   "Unable to find repo".
6. Config, with the package **LSP-file-watcher-chokidar** installed: create
   `.concourse-lint.yml` next to `get-put.yml` with `rules: {CL002: off}`,
   once in Sublime Text and once from a terminal. The errors disappear both
   times. Delete the file: they are back. Without the file watcher, they come
   back only after **LSP: Restart Server**.
7. Remove `bin` from the `PATH` and restart Sublime Text: the status bar says
   that concourse-lint was not found. **Preferences: LSP-concourse-lint
   Settings** with `"command": ["/full/path/to/bin/concourse-lint", "lsp"]`
   starts it again.

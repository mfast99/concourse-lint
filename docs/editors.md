# concourse-lint in your editor

`concourse-lint lsp` is a language server (LSP 3.17 on stdin/stdout). Every
editor with an LSP client gets the findings while you type, go to definition,
references, hover, quick fixes and completion. The contract is in
[spec/README.md, "Language server contract"](../spec/README.md#language-server-contract).

The binary has to be on your `PATH`, or give its full path in place of
`concourse-lint` below. VS Code needs neither: the extension brings the binary.

## Which files are checked

Configure the server for all YAML files. It checks only those that look like a
pipeline (a top-level `jobs:` with a `plan:`), the same as
`concourse-lint check` in a folder; other YAML files get no findings. Use
`exclude` in `.concourse-lint.yml` for pipelines that should not be checked.
Only VS Code can mark a single file as a pipeline (language mode **Concourse
Pipeline**).

## The config file

The server applies the nearest `.concourse-lint.yml` and asks the editor to
report its changes (a file watcher, `workspace/didChangeWatchedFiles`). Not
every editor sees every change:

| Editor | Change saved in the editor | Change outside (e.g. `git checkout`) |
|---|---|---|
| VS Code | yes | yes |
| Emacs (Eglot, lsp-mode) | yes | yes |
| Helix | yes | no: run `:lsp-restart` |
| Sublime Text | with LSP-file-watcher-chokidar | with LSP-file-watcher-chokidar; without it run **LSP: Restart Server** |

## VS Code

Install the extension **Concourse Lint**; see
[editors/vscode/README.md](../editors/vscode/README.md).

## Sublime Text

The package `LSP-concourse-lint`; see
[editors/sublime/README.md](../editors/sublime/README.md).

## Helix

In `~/.config/helix/languages.toml` (or `.helix/languages.toml` in the
project):

```toml
[language-server.concourse-lint]
command = "concourse-lint"
args = ["lsp"]

[[language]]
name = "yaml"
language-servers = ["yaml-language-server", "concourse-lint"]
```

`language-servers` replaces the list Helix has for YAML, so name the servers
you already use along with `concourse-lint`. A server that is not installed
is skipped. `helix --health yaml` shows what Helix found.

## Neovim

Neovim 0.11 or newer configures language servers without a plugin. In
`~/.config/nvim/init.lua`:

```lua
vim.lsp.config('concourse_lint', {
  cmd = { 'concourse-lint', 'lsp' },
  filetypes = { 'yaml' },
  root_markers = { '.concourse-lint.yml', '.git' },
})
vim.lsp.enable('concourse_lint')
```

It runs next to `yamlls` if you use that. `:checkhealth vim.lsp` shows
whether the server started. This setup is not in "Checked with" below yet;
please report what does not work. `vim.lsp.enable('concourse_lint')` without
the config above and `:MasonInstall concourse-lint` follow once the server is
in nvim-lspconfig and the Mason registry (#3).

## Emacs

Eglot (built in since Emacs 29) runs one server per buffer, so this replaces
`yaml-language-server` in YAML buffers. If you want both, use lsp-mode
(below).

```elisp
(with-eval-after-load 'eglot
  (add-to-list 'eglot-server-programs
               '((yaml-mode yaml-ts-mode) . ("concourse-lint" "lsp"))))
(add-hook 'yaml-mode-hook #'eglot-ensure)
(add-hook 'yaml-ts-mode-hook #'eglot-ensure)
```

lsp-mode runs concourse-lint as an add-on, next to `yaml-language-server`:

```elisp
(with-eval-after-load 'lsp-mode
  (lsp-register-client
   (make-lsp-client
    :new-connection (lsp-stdio-connection '("concourse-lint" "lsp"))
    :activation-fn (lsp-activate-on "yaml")
    :add-on? t
    :server-id 'concourse-lint)))
(add-hook 'yaml-mode-hook #'lsp-deferred)
(add-hook 'yaml-ts-mode-hook #'lsp-deferred)
```

## Checked with

Helix 25.07.1, Emacs 31.1 with Eglot 1.24 and lsp-mode 20261109. In each: the
findings of a spec case on the `# want` lines, and a `.concourse-lint.yml`
with `rules: {CL002: off}` that removes them (see the table above for changes
outside the editor). Sublime Text 4 build 4215 with LSP 2.16 and
LSP-file-watcher-chokidar: the checklist
[editors/sublime/TESTING.md](../editors/sublime/TESTING.md).

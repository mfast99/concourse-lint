# LSP-concourse-lint

Concourse CI pipelines in Sublime Text: errors, security issues and bad
practice while you type, go to definition, references, hover, quick fixes
and completion. The package starts `concourse-lint lsp` through the
[LSP](https://packagecontrol.io/packages/LSP) package (version 2.12 or newer).

## Install

1. Install the package **LSP** with Package Control.
2. Nothing else: on the first start the package downloads the
   `concourse-lint` of its own version from the
   [GitHub release](https://github.com/mfast99/concourse-lint/releases),
   checks it against `checksums.txt` and keeps it in Sublime Text's package
   storage. Without a connection it uses the `concourse-lint` on your `PATH`.
3. Until the package is in Package Control, link this folder into your
   `Packages` folder (**Preferences › Browse Packages…**) under the name
   `LSP-concourse-lint`. The name matters: LSP finds the settings by it.

   ```sh
   # Linux; macOS: ~/Library/Application Support/Sublime Text/Packages
   ln -s "$PWD/editors/sublime" ~/.config/sublime-text/Packages/LSP-concourse-lint
   ```

## Which files

Every YAML file (`source.yaml`) is sent to the server; it checks only those
that look like a pipeline (a top-level `jobs:` with a `plan:`), the same as
`concourse-lint check` in a folder. Other YAML files get no findings. Use
`exclude` in `.concourse-lint.yml` for pipelines that should not be checked.

## Settings

**Preferences: LSP-concourse-lint Settings** in the command palette. To use
another binary than the downloaded one:

```jsonc
{
	"command": ["/path/to/concourse-lint", "lsp"],
}
```

## Keys

LSP binds no key to go to definition, so F12 stays Sublime Text's own (it
only knows symbols of its index, e.g. "Unable to find repo"). Use
**LSP: Goto Definition** in the command palette or the context menu **LSP**,
or bind F12 in **Preferences › Key Bindings**:

```jsonc
[
	{
		"keys": ["f12"],
		"command": "lsp_symbol_definition",
		"args": {"side_by_side": false, "force_group": true, "fallback": false, "group": -1},
		"context": [
			{"key": "lsp.session_with_capability", "operand": "definitionProvider"},
			{"key": "auto_complete_visible", "operand": false},
		],
	},
]
```

The context keeps F12 for other files as it was. Quick fixes:
**LSP: Code Action**.

## Config file

The server applies the nearest `.concourse-lint.yml` and reads it again when
it changes. Sublime Text reports changes of files only with a file watcher:
install the package **LSP-file-watcher-chokidar**. Without it, run
**LSP: Restart Server** after changing the config file.

// The VS Code extension: starts `concourse-lint lsp` and passes it the YAML
// files. Everything else (which file is a pipeline, the config file, the
// findings) is done by the language server, see spec/README.md, "Language
// server".

import { chmodSync, existsSync, statSync } from 'node:fs';
import * as vscode from 'vscode';
import { LanguageClient, type LanguageClientOptions, type ServerOptions } from 'vscode-languageclient/node';

import { resolveServer } from './binary.ts';

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  context.subscriptions.push(
    vscode.commands.registerCommand('concourse-lint.restart', () => restart(context)),
    vscode.workspace.onDidChangeConfiguration(async (event) => {
      if (event.affectsConfiguration('concourse-lint.path')) {
        await restart(context);
      }
    }),
  );
  await start(context);
}

export async function deactivate(): Promise<void> {
  await stop();
}

async function start(context: vscode.ExtensionContext): Promise<void> {
  const resolution = resolveServer({
    configuredPath: vscode.workspace.getConfiguration('concourse-lint').get<string>('path', ''),
    extensionPath: context.extensionPath,
    platform: process.platform,
    exists: existsSync,
  });
  if ('error' in resolution) {
    void vscode.window.showErrorMessage(`Concourse Lint: ${resolution.error}`);
    return;
  }
  if (resolution.source === 'bundled') {
    makeExecutable(resolution.command);
  }
  const serverOptions: ServerOptions = { command: resolution.command, args: ['lsp'] };
  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ language: 'yaml' }, { language: 'concourse-pipeline' }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher('**/.concourse-lint.{yml,yaml}'),
    },
  };
  client = new LanguageClient('concourse-lint', 'Concourse Lint', serverOptions, clientOptions);
  try {
    await client.start();
  } catch (error) {
    const hint = resolution.source === 'path'
      ? ' Install concourse-lint or set concourse-lint.path.'
      : '';
    void vscode.window.showErrorMessage(`Concourse Lint: the language server did not start (${String(error)}).${hint}`);
  }
}

async function stop(): Promise<void> {
  if (client !== undefined) {
    const running = client;
    client = undefined;
    await running.stop();
  }
}

async function restart(context: vscode.ExtensionContext): Promise<void> {
  await stop();
  await start(context);
}

// makeExecutable: an extension package may lose the executable bit of the
// bundled binary on some systems.
function makeExecutable(path: string): void {
  if (process.platform === 'win32') {
    return;
  }
  try {
    if ((statSync(path).mode & 0o111) === 0) {
      chmodSync(path, 0o755);
    }
  } catch {
    // The start fails with a clear message then.
  }
}

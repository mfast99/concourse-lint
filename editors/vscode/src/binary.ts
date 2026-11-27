// Finds the concourse-lint binary that runs the language server: the setting
// concourse-lint.path, the binary bundled with
// the extension, or concourse-lint from PATH. Kept free of the vscode API so
// that it can be tested with plain Node.

import { join } from 'node:path';

export interface Lookup {
  // The value of concourse-lint.path, '' when not set.
  configuredPath: string;
  extensionPath: string;
  // process.platform, e.g. 'linux', 'darwin', 'win32'.
  platform: string;
  exists: (path: string) => boolean;
}

export type Resolution =
  | { command: string; source: 'setting' | 'bundled' | 'path' }
  | { error: string };

export function bundledBinary(extensionPath: string, platform: string): string {
  const name = platform === 'win32' ? 'concourse-lint.exe' : 'concourse-lint';
  return join(extensionPath, 'bin', name);
}

export function resolveServer(lookup: Lookup): Resolution {
  const configured = lookup.configuredPath.trim();
  if (configured !== '') {
    if (lookup.exists(configured)) {
      return { command: configured, source: 'setting' };
    }
    return { error: `concourse-lint.path is set to "${configured}", but there is no such file.` };
  }
  const bundled = bundledBinary(lookup.extensionPath, lookup.platform);
  if (lookup.exists(bundled)) {
    return { command: bundled, source: 'bundled' };
  }
  return { command: 'concourse-lint', source: 'path' };
}

import assert from 'node:assert/strict';
import { test } from 'node:test';
import { join } from 'node:path';

import { bundledBinary, resolveServer, type Lookup } from './binary.ts';

const extensionPath = join('/ext', 'concourse-lint');

function lookup(overrides: Partial<Lookup>): Lookup {
  return { configuredPath: '', extensionPath, platform: 'linux', exists: () => false, ...overrides };
}

test('the setting wins when the file exists', () => {
  const result = resolveServer(lookup({ configuredPath: '/opt/my-build', exists: () => true }));
  assert.deepEqual(result, { command: '/opt/my-build', source: 'setting' });
});

test('a setting that points to nothing is an error, not a silent fallback', () => {
  const result = resolveServer(lookup({ configuredPath: '/opt/missing', exists: (path) => path !== '/opt/missing' }));
  assert.ok('error' in result);
  assert.match(result.error, /\/opt\/missing/);
});

test('then the bundled binary', () => {
  const bundled = join(extensionPath, 'bin', 'concourse-lint');
  const result = resolveServer(lookup({ exists: (path) => path === bundled }));
  assert.deepEqual(result, { command: bundled, source: 'bundled' });
});

test('the bundled binary has .exe on Windows', () => {
  assert.equal(bundledBinary(extensionPath, 'win32'), join(extensionPath, 'bin', 'concourse-lint.exe'));
  assert.equal(bundledBinary(extensionPath, 'darwin'), join(extensionPath, 'bin', 'concourse-lint'));
});

test('without both, concourse-lint from PATH', () => {
  assert.deepEqual(resolveServer(lookup({})), { command: 'concourse-lint', source: 'path' });
});

test('a setting with only spaces counts as empty', () => {
  assert.deepEqual(resolveServer(lookup({ configuredPath: '  ' })), { command: 'concourse-lint', source: 'path' });
});

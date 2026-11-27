'use strict';

const assert = require('node:assert/strict');
const { join } = require('node:path');
const { test } = require('node:test');

const { binaryPath, supportedTargets } = require('../lib/binary.js');

const packageRoot = join('/', 'node_modules', 'concourse-lint');

test('every target of the release has a binary', () => {
  assert.deepEqual(supportedTargets, [
    'linux-x64',
    'linux-arm64',
    'darwin-x64',
    'darwin-arm64',
    'win32-x64',
    'win32-arm64',
  ]);
});

test('the binary of the platform and the architecture', () => {
  assert.equal(
    binaryPath(packageRoot, 'linux', 'x64'),
    join(packageRoot, 'bin', 'linux-x64', 'concourse-lint'),
  );
  assert.equal(
    binaryPath(packageRoot, 'darwin', 'arm64'),
    join(packageRoot, 'bin', 'darwin-arm64', 'concourse-lint'),
  );
});

test('the binary has .exe on Windows', () => {
  assert.equal(
    binaryPath(packageRoot, 'win32', 'x64'),
    join(packageRoot, 'bin', 'win32-x64', 'concourse-lint.exe'),
  );
});

test('an unsupported platform names the supported ones', () => {
  assert.throws(() => binaryPath(packageRoot, 'freebsd', 'x64'), {
    message:
      'concourse-lint: no binary for freebsd-x64 in the npm package ' +
      '(supported: linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64, win32-arm64); ' +
      'see https://github.com/mfast99/concourse-lint for other ways to install it',
  });
});

test('an unsupported architecture is not mixed up with a supported one', () => {
  assert.throws(() => binaryPath(packageRoot, 'linux', 'ia32'), /no binary for linux-ia32/);
});

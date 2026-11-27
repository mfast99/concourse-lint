'use strict';

// Finds the concourse-lint binary for this machine in the npm package. The
// package carries one binary per target in bin/<platform>-<arch>/, named
// after process.platform and process.arch (the same targets as the VS Code
// extension).

const { join } = require('node:path');

const supportedTargets = [
  'linux-x64',
  'linux-arm64',
  'darwin-x64',
  'darwin-arm64',
  'win32-x64',
  'win32-arm64',
];

function binaryPath(packageRoot, platform, arch) {
  const target = `${platform}-${arch}`;
  if (!supportedTargets.includes(target)) {
    throw new Error(
      `concourse-lint: no binary for ${target} in the npm package ` +
        `(supported: ${supportedTargets.join(', ')}); ` +
        'see https://github.com/mfast99/concourse-lint for other ways to install it',
    );
  }
  const binaryName = platform === 'win32' ? 'concourse-lint.exe' : 'concourse-lint';
  return join(packageRoot, 'bin', target, binaryName);
}

module.exports = { binaryPath, supportedTargets };

#!/usr/bin/env node
'use strict';

// The command of the npm package: runs the concourse-lint binary for this
// machine with the same arguments, stdin, stdout and stderr, and ends with
// its exit code.

const { spawnSync } = require('node:child_process');
const { join } = require('node:path');

const { binaryPath } = require('../lib/binary.js');

// Exit code 2 like a usage error of the binary: nothing was checked.
const cannotRunExitCode = 2;

let binary;
try {
  binary = binaryPath(join(__dirname, '..'), process.platform, process.arch);
} catch (lookupError) {
  console.error(lookupError.message);
  process.exit(cannotRunExitCode);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: 'inherit' });
if (result.error) {
  console.error(`concourse-lint: cannot run ${binary}: ${result.error.message}`);
  process.exit(cannotRunExitCode);
}
// The binary was ended by a signal (e.g. Ctrl+C): end the same way, so that
// the caller sees the signal and not a made-up exit code.
if (result.signal) {
  process.kill(process.pid, result.signal);
}
process.exit(result.status);

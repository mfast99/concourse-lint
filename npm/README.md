# concourse-lint

Linter and language server for [Concourse CI](https://concourse-ci.org)
pipelines: finds errors, security issues and bad practice before
`fly set-pipeline` runs.

This package brings the `concourse-lint` binary along. It contains the
binaries for Linux, macOS and Windows on x64 and arm64 and needs no
download or install script.

```sh
# once
npx concourse-lint check pipeline.yml

# in a project
npm install --save-dev concourse-lint
npx concourse-lint check ci/
```

For example in `package.json`:

```json
{
  "scripts": {
    "lint:pipelines": "concourse-lint check ci/"
  }
}
```

`concourse-lint help` lists the commands, `concourse-lint rules` the rules.
The exit codes are those of the binary: 0 without errors, 1 with errors, 2
for a usage error. The package also exits with 2 when it has no binary for
the machine (supported: `linux`, `darwin` and `win32` on `x64` and `arm64`).

Documentation, editor setup and the rules:
<https://github.com/mfast99/concourse-lint>

"Concourse" is a trademark of its respective owner. concourse-lint is an
independent project and is not affiliated with or endorsed by the Concourse
project.

License: Apache-2.0. Redistributions must keep the NOTICE file. The
license texts of the third-party software in the binaries are in
`THIRD_PARTY_LICENSES.txt`; `concourse-lint licenses` prints them too.

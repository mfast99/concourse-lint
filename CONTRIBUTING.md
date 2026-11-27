# Contributing to concourse-lint

Thanks for your interest! This project is test-driven: the expected behavior
lives in `spec/` as plain YAML pipelines, so you can contribute a test case
without writing any Go.

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).
Security issues: please follow [SECURITY.md](SECURITY.md) and do **not** open a
public issue.

## Workflow at a glance

1. **Open an issue first.** Every change starts with an issue (bug, false
   positive, rule proposal, feature). Wait for a maintainer to confirm the
   direction before investing a lot of time.
2. **Branch off `develop`** using the [branch naming scheme](#branch-names).
3. **Test first:** add or change the case in `spec/`, see it fail, then write
   the code (see [Adding a new rule](#adding-a-new-rule)).
4. Run `make check` (format, vet, staticcheck, tests).
5. **Open a pull request against `develop`.** The PR title follows the
   [commit message scheme](#commit-messages) and the description contains
   `Closes #<issue>`.
6. A maintainer reviews and **squash-merges** it into `develop`.

## Branches

| Branch      | Purpose |
|-------------|---------|
| `main`      | Released code only. Every commit on `main` is a release and carries a tag `vX.Y.Z`. |
| `develop`   | Integration branch for the next release. All pull requests target `develop`. |
| `<type>/…`  | Short-lived work branches, one per issue, branched off `develop`. |
| `hotfix/…`  | Urgent fix for a released version, branched off `main` (maintainers only). |

Nobody pushes directly to `main` or `develop`.

### Branch names

```
<type>/<issue>-<short-description>
```

* `<type>`: one of the commit types below (`feat`, `fix`, `docs`, …) or `hotfix`.
* `<issue>`: the issue number, without `#`.
* `<short-description>`: 2–5 words, lowercase, words separated by `-`.

Examples:

```
feat/12-parser-merge-keys
fix/34-cs001-false-positive-sort-key
docs/40-cl002-rule-page
test/41-cl013-diamond-case
hotfix/57-crash-on-empty-plan
```

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/) and
reference the issue in the footer:

```
<type>(<scope>): <summary>

<optional body: what and why, not how>

Refs #<issue>
```

* **type**
  * `feat`: new rule or user-facing capability
  * `fix`: bug fix, including false positives and false negatives
  * `test`: spec cases or Go tests only
  * `docs`: documentation only
  * `refactor`: code change without behavior change
  * `perf`: performance improvement
  * `build`: build system, dependencies, `go.mod`
  * `ci`: GitHub Actions workflows
  * `chore`: anything else (repo housekeeping)
* **scope** (optional but encouraged): a rule ID (`CL002`, `CS001`) or a
  component (`parse`, `model`, `lint`, `report`, `cli`, `lsp`, `mcp`, `spec`).
* **summary**: imperative mood, lowercase start, no trailing period, at most
  72 characters in total for the first line.
* **footer**: `Refs #<issue>` on every commit. The pull request description
  uses `Closes #<issue>` so the issue is closed when the PR is merged.
* Breaking change: `!` after the type/scope (`feat(cli)!: …`) and a
  `BREAKING CHANGE: …` footer.

Examples:

```
feat(CL002): report get/put steps with an unknown resource

Refs #12
```

```
fix(CS001): ignore keys ending in "sort_key"

Refs #34
```

Pull requests are squash-merged, so the **PR title becomes the commit
message on `develop`**. CI checks the PR title, the branch name and that
the description references an issue.

## Adding a new rule

Always in this order (test first):

1. **Issue:** use the "Rule proposal" template and agree on ID and severity.
2. **Catalog:** add a line to `spec/catalog.txt` (ID, severity, title).
   Areas: `CL` correctness, `CS` security, `CB` best practice.
3. **Specification cases:** create the folder `spec/rules/<area>/<ID>/` with
   one scenario per file (`<scenario>.yml`, each with `# want <ID>`). Cover
   as many distinct situations as you can, without repeating one that another
   file already covers. Warnings and info rules also need a `negatives.yml`
   with near misses that must not fire; it is recommended for errors too.
   `TestSpecLayout` enforces this.
4. Run `make spec`: the new cases must be red.
5. **Logic:** `internal/rules/<area>/<id>_<topic>.go` with one variable
   of type `analysis.Rule`, registered in the area's `All`.
6. Run `make check`: everything green.
7. **Docs:** write `docs/rules/<ID>.md`, with the sections "What is checked?"
   and "Fix": the editor shows them on hover (`TestRulePages`).

How to write spec cases is described in [spec/README.md](spec/README.md).

## Reporting or fixing a false positive

Use the "False positive" issue template and include the smallest pipeline
snippet that triggers it. The fix starts by adding that snippet as a line to
the matching `spec/rules/<area>/<ID>/negatives.yml`, then the code is changed.

## Development setup

You need Go (see `go.mod` for the minimum version) and `make`.

```sh
make test   # all tests          make spec  # spec/ only
make cli    # black-box tests    make fuzz  # 60 s fuzzing
make lint   # staticcheck        make vuln  # govulncheck (needs the network)
make check  # fmt + vet + lint + test, run before every push
make conformance  # field tables against Concourse (module of its own, needs the network)
```

staticcheck and govulncheck have their own module in `tools/`, so that their
dependencies stay out of the `go.mod` of the linter; `go tool
-modfile=tools/go.mod <tool>` runs them in the pinned version. Dependabot
keeps `go.mod`, `tools/go.mod`, the GitHub Actions and the npm packages of
the extension up to date (`.github/dependabot.yml`).

The license texts of the dependencies are shipped with the binary
(`concourse-lint licenses`) and the extension. After adding or updating a
dependency (also in a Dependabot pull request), run
`scripts/third-party-licenses.sh` (needs Go, Node, pnpm and jq) and commit
the two files it writes. While they are out of date, CI fails (and
`make test` for a Go module).

### A new Concourse release

The keys the linter knows (CL018 reports every other one) are checked
against the structs of Concourse (`atc`) by `make conformance`, in the
module `conformance/concourse`. For a new release, point that module at the
commit of the new tag (the Concourse module has no `/v8` suffix, so the tag
itself cannot be used) and run the check:

```sh
cd conformance/concourse
go get github.com/concourse/concourse@<commit of the tag> && go mod tidy
make -C ../.. conformance
```

Every key it reports becomes a spec case first, then the parser learns it.
A deliberate difference goes into `knownDifferences` in
`conformance/concourse/conformance_test.go`, with the reason.

## Releasing (maintainers)

1. Make sure `develop` is green. In `CHANGELOG.md`, move the items from
   "Unreleased" into a new section `## [X.Y.Z] - YYYY-MM-DD`.
2. Open a pull request `develop` → `main` titled `chore(release): vX.Y.Z`.
3. Merge it with a **merge commit** (not squash), so `main` and `develop`
   keep a shared history.

The merge starts `release.yml`; nobody tags by hand. It reads the newest
version section of `CHANGELOG.md` (`scripts/release-version.sh`) and
releases it when that version has no published GitHub release yet; a push
to `main` without a new version section releases nothing. Its jobs (the
`publish-*` jobs start only after github-release succeeded; their names
differ from the CI jobs, because a required check is found by its name):

* **github-release:** the tests, the tag `vX.Y.Z` on the merge commit, the
  archives of all six targets with `checksums.txt` (GoReleaser,
  `.goreleaser.yaml`) as a draft release with the section of the version as
  release notes (`scripts/release-notes.sh`), the container image
  `ghcr.io/mfast99/concourse-lint`, the Homebrew formula in
  `mfast99/homebrew-tap` (secret `HOMEBREW_TAP_TOKEN`), a build provenance
  attestation for every archive and the image, then the release is
  published. A version like `1.0.0-rc.1` becomes a pre-release; it gets no
  `latest` image and no formula.
* **publish-npm** and **publish-vscode:** the npm package and the VS Code extension, each
  only when its repository variable (`PUBLISH_NPM`, `PUBLISH_VSCODE`) is
  `"true"` and each in a GitHub environment of the same name that only
  `main` can use. npm and the Visual Studio Marketplace trust `release.yml`
  in that environment (trusted publishing over OIDC, no secret); Open VSX
  needs the secret `OVSX_PAT`. The VSIX files of all platforms also go to
  the GitHub release.
* **publish-action:** `action/` goes to `mfast99/concourse-lint-action` with this
  version as the default input, tagged `vX.Y.Z` and released there (also
  with `HOMEBREW_TAP_TOKEN`); not for a pre-release.

A failed run can be run again from the Actions tab; it reuses the tag and
replaces the draft release. When the release is published but a channel is
missing (e.g. after a token was fixed), start `release.yml` by hand on
`main` (**Run workflow**): it builds the packages of that version again and
publishes them only where they are missing (npm, Open VSX, the Marketplace,
the action).

`make release-snapshot` builds and checks the same archives locally into
`dist/release/` without publishing anything; CI does that on every push.
GoReleaser has a module of its own (`tools/release/go.mod`), because it
needs the newest Go; `make goreleaser` builds it into `bin/`.

Versions follow [Semantic Versioning](https://semver.org/). Before 1.0.0,
new rules or stricter checks may appear in minor versions.

**Hotfixes:** branch `hotfix/<issue>-…` off `main` and open a PR against
`main` with a new patch version section in `CHANGELOG.md`; merge it with a
merge commit, which releases it. Then bring `main` back into `develop`: a
branch `chore/<issue>-merge-main` off `develop`, `git merge origin/main`, a
PR against `develop`, merged with a **merge commit** (a squash would lose
the shared history). A PR straight from `main` cannot work: `main` is never
up to date with `develop`.

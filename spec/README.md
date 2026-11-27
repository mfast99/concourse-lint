# Specification test cases for concourse-lint

Every `.yml` file is a test case: "Here is a pipeline, at location X I
expect finding Y." You don't need Go to write cases.
`internal/spectest` reads the files, `go test ./...` runs them.
`catalog.txt` defines the ID, severity and title of every rule.

## Folders

| Folder      | Contents |
|-------------|----------|
| `valid/`    | Valid, clean pipelines. Every finding is a false positive. |
| `rules/`    | One folder per rule, grouped by area: `rules/<area>/<ID>/`. See below. |
| `syntax/`   | Broken YAML. Only CL000 is expected. |
| `suppress/` | Suppression via comment. |
| `config/`   | Config file: one folder per scenario (see "Config file"). |

## Rule folders

```
rules/
  correctness/    CL001–CL024   (CL000 is covered by syntax/)
  security/       CS001–CS009
  bestpractice/   CB001–CB003
    <ID>/
      <scenario>.yml   one scenario per file, e.g. alias.yml, in-hook.yml
      negatives.yml    near misses that must NOT fire
```

* Every rule has at least one scenario file; cover as many **distinct**
  situations as you can. Each file tests exactly one situation and expects
  the folder's rule at least once.
* No duplicates: a situation that another file in the folder already covers
  is not repeated, and there are no collection files that combine several
  scenarios. Near misses belong in `negatives.yml` only.
* The rule itself is defined in this README (table and "Defined behavior"),
  not in the comments of the cases.
* Warnings and info rules **must** have a `negatives.yml`, errors may.
  It must not expect the folder's rule.
* Apart from the scenario, every file is a clean pipeline: all resources
  used, images pinned, `https://` only. Intended follow-up findings of
  other rules are listed with their own `# want`.
* File names are lowercase-kebab-case. `TestSpecLayout` checks all of this.

## Writing expectations

```yaml
  - get: rep        # want CL002         exactly one CL002 on this line
    passed: [a, b]  # want CL003 CL003   exactly two CL003 on this line
# want-anywhere CL000                    exactly one CL000, on any line
```

* Compared are line, rule ID and severity, not the message text.
* Findings are **counted**: every rule ID in a `want` stands for exactly one
  finding. A finding first uses up a `# want` of its own line, only then a
  `want-anywhere`. A duplicate finding (e.g. once per use of an anchor) fails.
* Every finding that is not expected makes the test fail.
* A false positive from real-world use: first add it as a line to the matching
  `negatives.yml`, then fix the code.

## Test types

| Test | Checks |
|------|--------|
| `TestSpec` | exactly the expected findings per file, with severity from `catalog.txt` |
| `TestCatalogCoverage` | every rule in the catalog has a test case, every test case names a known rule |
| `TestSpecLayout` | folder structure of `rules/`, at least one scenario per rule, `negatives.yml` for warnings and info |
| `TestCatalogImplemented` | every rule in the catalog is registered in code, with the same severity |
| `TestShiftedLines` | 3 comment lines at the top shift all findings by 3 lines |
| `TestCRLF` | Windows line endings change nothing |
| `FuzzCheck` | no crash on arbitrary input, positions valid (`make fuzz`) |
| `cmd/concourse-lint/cli_test.go` | black box: built binary, exit codes, stdin, JSON contract, SARIF |

## Rules

Severity: **error** fails CI (exit 1). **Warning** only with
`--fail-on warning`. **Info** never fails. All three are always shown.
Rules prone to false positives are at most a warning and have their own
`negatives.yml`.

concourse-lint may be stricter than Concourse: it also reports what fly
accepts but is almost certainly a mistake (e.g. CL018, CB003). Such a check
always has its own rule ID, so that a team can switch it off on its own.

| ID    | Severity | Checks |
|-------|----------|--------|
| CL000 | error    | YAML syntax |
| CL001 | error    | Structure: unknown fields in steps, inline task `config` and `across`, wrong types, required fields (a task needs `file` or `config`), steps, no jobs, invalid `check_every` |
| CL002 | error    | get/put, `resource:` or `groups[].resources` refers to a missing resource |
| CL003 | error    | `passed:` names a missing job |
| CL004 | error    | Resource type neither built in nor defined |
| CL005 | error    | Duplicate names within a category (resources, resource types, prototypes, var sources, jobs, groups; in a job: `get` and `load_var` names) |
| CL006 | error    | Resource is never used |
| CL007 | error    | `file:` does not start with an artifact available at this point |
| CL008 | error    | `((.:var))` without a preceding `load_var`, or outside its `across` |
| CL009 | error    | Job in `passed:` does not use the resource |
| CL010 | error    | Group names a missing job, or a glob matches none |
| CL011 | error    | Inline task `inputs` without an available artifact |
| CL012 | error    | `input_mapping`/`output_mapping` points to nothing |
| CL013 | error    | Cycle via `passed:` |
| CL014 | warning  | Task output shadows an artifact that is not an input |
| CL015 | warning  | Suppression comment without a rule ID or with an unknown one |
| CL016 | error    | Job in no group although groups exist |
| CL017 | warning  | Unquoted value that Concourse passes on in a different spelling: a number in `source`, `params` or `vars` (`1.10` becomes `1.1`), or a number or boolean in a string field outside steps (`name: yes` becomes "true") |
| CL018 | warning  | Unknown field that Concourse silently ignores (everywhere outside steps, inline task `config` and `across`), e.g. `sorce:` |
| CL019 | error    | Value that fly accepts but that fails every time the step or check runs: `timeout`, `check_timeout`, load_var `format` |
| CL020 | warning  | Unknown word in `version` of a get or `inputs` of a put, which Concourse silently ignores |
| CL021 | info     | Literal tab inside a plain or quoted string (not in `\|`/`>` blocks, not the escape `\t`); YAML and Concourse accept it, but it is almost never meant |
| CL022 | info     | ytt, spruce, ERB or Jinja template instead of a pipeline: one finding at the first marker instead of all other findings |
| CL023 | info     | Pipeline fragment: get or put steps but no top-level `resources:` (merged into a pipeline by spruce, aviator, …): one finding at `jobs:` instead of all other findings |
| CL024 | warning  | Content after the first YAML document (after a `---` or `...` line), which Concourse ignores; only the first document is checked |
| CS001 | warning  | Secret-like key with a fixed value |
| CS002 | error    | Known token format (AWS, GitHub, GitLab, Slack, Stripe live, PEM private key), also in comments |
| CS003 | warning  | TLS verification disabled |
| CS004 | warning  | `http://` or `git://` in a `source` |
| CS005 | warning  | Image in `image_resource`/`resource_types` without tag or digest, or with `latest` |
| CS006 | warning  | Task with `privileged: true` |
| CS007 | warning  | Resource type with `privileged: true` |
| CS008 | warning  | Job with `public: true` |
| CS009 | info     | `load_var` with `reveal: true` |
| CB001 | info     | Resource type is not used |
| CB002 | warning  | `check_every` below 1 minute |
| CB003 | warning  | Job without steps (no `plan:` or an empty one) |

## Defined behavior

* **Positions:** The line of the wrong value is reported. If the value is a
  block (list or mapping) of the wrong type, the line of its key. For missing
  fields or invalid steps the first line of the element, for duplicates
  every further occurrence, for CS001/CS002 the line of the key (also for
  block scalars `|`), for CS005 the `tag` line, otherwise the `repository` line.
  Unused or ungrouped elements (CL006, CL016, CB001) at the line of the name.
  CL015 at the line of the suppression comment.
* **Anchors:** An error inside an anchor is reported once, at the definition.
  This also applies to merge keys (`<<`).
* **Follow-up findings:** Elements without a name produce none. With a
  syntax error there is only CL000. CS002 replaces CS001 at the same location.
  A step without a step type (CL001, e.g. the removed `aggregate:`) has an
  unknown content: it may use any resource and fetch any artifact, so CL006,
  CL009 and the artifact rules do not report what it could provide.
* **Booleans:** unquoted `yes`/`on` count as `true` and `no`/`off` as `false`
  (YAML 1.1, like Concourse). A quoted `"true"` is a string: in a boolean
  field that is a wrong type (CL001), as in Concourse, and not `true`.
* **Built-in types:** bosh-io-release, bosh-io-stemcell, cf, docker-image,
  git, github-release, hg, mock, pool, registry-image, s3, semver, time,
  tracker. More can be added in the config file (`resource_types:`).

### Artifacts, variables and groups (CL006–CL014, CL016)

* **Artifacts** inside a job come from `get` (named after the step, not after
  `resource:`), from task `outputs` (renamed by `output_mapping`) and from the
  implicit get of a `put` (named after the put step; none with `no_get: true`).
  An artifact is available to *later* steps only: not in another job, and not
  to siblings inside the same `in_parallel`, unless it has `limit: 1` (then
  its steps run one after another, in list order). After an `in_parallel` or
  `do` block its artifacts are available. A step with `across` runs every
  iteration in its own scope: its artifacts and local variables are not
  available after it. Only the success path counts: what an `on_failure`,
  `on_error` or `on_abort` hook produces is not available to later steps
  (they don't run after a failure), except inside `try`; job hooks see
  everything the plan can have produced.
* **Tasks from `file:`:** their inputs and outputs are unknown, so CL011, the
  `output_mapping` check of CL012 and CL014 do not apply to them, and after
  such a task CL007, CL011 and CL012 do not report names that are not known
  otherwise (an unknown output could provide them).
* **Hint for a missing artifact (CL007, CL011, CL012):** "did you mean" with
  the closest available artifact. The artifact of a `get` or `put` without
  `resource:` whose resource is undefined (a CL002) is never suggested,
  because its name is the typo. When the missing name is the resource that
  CL002 suggests for that step, the hint points to the step instead: `the get
  "rpeo" in line 11 probably means resource "repo"; fix it there`. A name
  chosen with `resource:` stays a suggestion.
* **CL006:** error because Concourse rejects the pipeline
  ("resource 'x' is not used"). A resource is used by `get`, `put` or `resource:`, anywhere in the
  plan including hooks. The name after `get:` when `resource:` is set, and a
  name at the start of a `file:` path, are not a use.
* **CL007:** the first segment of a `file:` path (task, `load_var`,
  `set_pipeline`) must be an available artifact. An absolute path never is.
  `((var))` later in the path is fine.
* **CL008:** `((.:name))` needs a `load_var: name` or a `get: name` (also the
  implicit get of a `put`, which sets the version metadata as a local
  variable) earlier in the same job, or an `across` var of the step it is in (including the nested steps of an
  `across` on `do`). `((.:name.field))` refers to `name`. `((name))` without
  the dot is never CL008.
* **CL009:** the job in `passed:` must use the same resource (via `get` or
  `put`, also with an alias). The artifact name does not matter.
* **CL010:** entries are exact job names or globs (`*`, `deploy-*`), both
  case-sensitive. A glob must match at least one job. A glob on which the
  glob library panics for one of the job names (an unclosed or empty `{}`
  alternative at the end, e.g. `build{` with the job `build`) is reported
  too, because fly crashes on it.
* **CL016:** only when groups exist. Globs count. Concourse rejects a job in
  no group ("job 'x' belongs to no group"), so this is an error.
* **CL011:** every input of an inline task must be an available artifact.
  Exceptions: `optional: true`, and inputs provided by `input_mapping` (those
  are checked by CL012).
* **CL012:** an `input_mapping` value must be an available artifact (the
  artifact name, not the resource name). An `output_mapping` key must be an
  output of the inline task.
* **CL013:** the graph of jobs via `passed:`, across all resources. Every
  `passed:` entry that is part of a cycle is reported, also a job that waits
  for itself. Jobs that only hang off a cycle are not.
* **CL014:** an output (or `output_mapping` target) with the name of an
  available artifact that is not also an input of the same task. Input and
  output with the same name is "modify in place" and fine.

### Unknown fields (CL001, CL018)

* **CL001:** Concourse rejects unknown fields in a step (a typo, a field of
  another step kind, `tags` on `load_var`, `set_pipeline` or `try`,
  `fail_fast` without `across`), in an inline task `config` and in an
  `across` variable.
* **CL018:** everywhere else Concourse ignores unknown fields without a word:
  at the top level, in resources, resource types, prototypes, jobs, groups,
  `var_sources`, `display`, `build_log_retention`, the block form of
  `in_parallel`, and below a task `config` (`image_resource`, `run`,
  `inputs`, `outputs`, `caches`). Only `fly validate-pipeline --strict`
  rejects them. A warning, because such a typo (`sorce:`) is almost always a
  bug.
* **Not CL018:** a top-level key whose value contains an anchor (holding
  anchors there is a common pattern), and keys inside free-form blocks
  (`source`, `params`, `vars`, …), which Concourse passes on unchecked.
* Both are reported at the key. If a known key is close (e.g. `sorce` →
  `source`), the hint is "did you mean …?". A field of another step kind gets
  the step kinds it belongs to as the hint.

### Values that fly does not check (CL019, CL020)

* **CL019:** fly accepts `timeout` (any step), `check_timeout` (resources) and
  `format` (load_var) without checking them, so that `((vars))` work. Concourse
  parses them when the step or check runs, and an invalid value fails it
  every time. `timeout` and `check_timeout` must be Go durations (`10m`,
  `1h30m`; no days), `format` one of `json`, `yaml`, `yml`, `raw`, `trim`.
  Reported at the value; hint "did you mean" or the duration units.
* **CL020:** `version` of a get knows only `latest` and `every` (or a mapping
  of version fields), `inputs` of a put only `all` and `detect` (or a list).
  Concourse ignores any other word without a word of its own and falls back
  to its default. Reported at the value with a "did you mean" hint.
* **Not CL019/CL020:** values with `((vars))`, a missing value, and
  `platform` (teams can have their own worker platforms).

### Values (CL017)

* **CL017:** fly reads YAML 1.1 and turns free-form values (`source`,
  `params`, `vars`, `across` values and everything below them) into JSON
  before Concourse sees them. An unquoted value that YAML 1.1 reads as a
  number is passed on in the spelling of that number. CL017 fires when that
  spelling differs from the text in the file: trailing zeros (`1.10`, `2.0`),
  leading zeros (`0022` is octal, so 18; `08` becomes 8), exponents (`1e3`),
  underscores (`1_000`), a sign (`+1`, `-0`), a missing leading zero (`.5`),
  hex, octal and binary (`0x1F`, `0o17`, `0b101`). Reported at the value,
  once per value. Hint: quote it.
* **Not CL017:** values that stay the same (`1.5`, `10`, `0`, very large
  numbers), values that are no number (`1.10.0`, `v1.10`, `3.20-alpine`,
  `1:20`, `2024-01-01`), quoted values and block scalars (`|`), `yes`/`no`/
  `on`/`off` (turned into booleans on purpose, e.g. for
  `skip_ssl_verification`), and fields that are numbers anyway (`attempts`,
  `limit`, `max_in_flight`).
* **CL017 in string fields outside steps:** a string field of a resource,
  resource type, job or group (e.g. `name`, `icon`, `serial_groups`, `jobs` of
  a group) gets an unquoted number or boolean as text, in the spelling
  Concourse uses: `yes` becomes `"true"`, `1.10` becomes `"1.1"`, `0777`
  becomes `"511"`, `1234567.0` becomes `"1.234567e+06"` (32-bit precision,
  checked with fly). CL017 fires when that text differs from what is
  written; `name: 12` stays `"12"` and is fine. Inside steps the same value
  is a wrong type (CL001), because fly rejects it there.

### Security (CS001–CS009)

* **CS001 heuristic:** A key is secret-like if its last word
  (split at `_`, `-` and case changes, case-insensitive) is `password`,
  `passwd`, `secret` or `token`, or the last two are
  `private|api|access|secret` + `key`. Checked in `source`, task `params`
  (from `file:` and inline), `set_pipeline` `vars` and the `config` of
  `var_sources`. No finding for a value
  that is only a `((var))` or `((.:var))`, empty values, booleans and numbers,
  `x-oauth-basic` (GitHub's fixed password for token auth over HTTPS), and
  placeholders: `REDACTED`, text in `<…>`, `YOUR-…` and `REPLACE-…`.
  A value that only *contains* a `((var))` has a fixed part and is a finding.
* **CS002 formats** (anywhere in the file, also in comments):
  AWS `AKIA` + 16 characters, GitHub `ghp_` + 36, GitLab `glpat-` + 20,
  Slack webhook `https://hooks.slack.com/services/T…/B…/…` and `xoxb-` bot
  tokens, Stripe `sk_live_` + at least 24, PEM/OpenSSH
  `-----BEGIN … PRIVATE KEY-----`. Shorter values, public keys, certificates
  and Stripe test keys are no finding, nor a private key frame with fewer
  than 20 base64 characters between BEGIN and END (a placeholder like `###`
  or a `((var))`, which does not count). A header without END is a finding.
* **CS003 keys:** truthy `skip_ssl_verification`, `insecure`, `disable_ssl`,
  or a non-empty `insecure_registries`. Checked in the `source` of resources,
  resource types and `image_resource`, and in `get_params` of a put. Task
  `params` are the task's business.
* **CS004:** values in `source` (also nested in maps and lists) that *start*
  with `http://` or `git://`, scheme case-insensitive. `localhost`,
  `127.0.0.1` and `[::1]` are exempt, but only as the whole hostname. Task
  `params` are the task's business.
* **CS005:** only `image_resource` and `resource_types` of type
  `registry-image` or `docker-image`. Regular resources track new versions on
  purpose. Pinned means a tag other than `latest` (also a `((var))`) or a
  digest via `version: {digest: …}`.
* **CS006–CS009** check only the flag itself: `privileged` on a task step,
  `privileged` on a resource type, `public` on a job, `reveal` on `load_var`.
  Fields with the same name in `source`, `params` or `defaults` do not count.

### Best practice (CB001–CB003)

* **CB001:** a type is used by a resource, by another used type, or as the
  `type` of an `image_resource`. A type that overrides a built-in one is used
  when resources of that type exist. In a chain of unused types, only the end
  (the type nothing refers to) is reported.
* **CB002:** Go durations (`500ms`, `10s`, `0m45s`). `1m` and more, and `never`,
  are fine. An invalid value is CL001.
* **CB003:** a job whose `plan` is missing or empty. Concourse accepts it (fly
  8.3.4), but the job does nothing: usually the plan was forgotten or the job
  is a leftover. Reported at the line of the job name; a job without a name
  is not reported.

### Templates (CL022)

* A file is a template when it has a ytt marker or a spruce operator, read
  from the YAML tokens (so also when the YAML is broken):
  * **ytt:** a comment that starts with `#@` (`#@ load(...)`, `#@ for ...`,
    `key: #@ value`, `- #@ helper()`, `#@overlay/match ...`). `# @todo`,
    `#@` inside a plain or quoted string and in a block scalar are no marker.
  * **spruce:** a plain or quoted value that is a whole spruce operator call
    (`(( grab meta.repo ))`, `(( append ))`) with a space inside. Operators:
    the 32 of spruce `doc/operators.md`, including the array merge
    operators. Not a marker: a Concourse variable (`((grab))`; Concourse
    allows no space inside, see `interpolationRegex` in `vars/template.go`),
    an operator call inside a longer value, an unknown word
    (`(( grabber x ))`), a comment, a block scalar.
  * **ERB and Jinja** (read from the lines, because such a template is often
    no YAML): an ERB tag `<% … %>`/`<%= … %>`, a Jinja tag `{% … %}` or
    comment `{# … #}`, and a line that starts with `<%`, `%>` or `{%` (a tag
    over several lines). Not a marker: a comment line, a comment behind a
    value, a block scalar. `{{ … }}` alone is no marker: it is also the old
    Concourse variable syntax (CL000 with a hint).
* A template gets exactly one finding: CL022 at the first marker. No other
  rule runs, also no CL000 for broken YAML: a template is no pipeline before
  it is rendered.
* With CL022 off (config) or suppressed (`ignore` at the marker line or
  `ignore-file`), the file is checked like a pipeline, also in a folder.
  To skip templates silently, use `exclude`.

## Suppression

```yaml
privileged: true  # concourse-lint:ignore CS006 -- justification
# concourse-lint:ignore CS001,CS006        applies to the next line
# concourse-lint:ignore-file CS004         applies to the whole file
```

* `ignore` applies to its own line, or, as a comment line of its own, to the
  line directly below it (no blank line in between).
* `ignore-file` applies to the whole file, wherever it is in the file.
* The rule list follows after one space: IDs separated by commas, no spaces.
  A justification after ` -- ` is optional.
* A suppression that suppresses nothing (the rule does not fire there) is fine.
* **CL015:** a comment without a rule ID, or with an ID that is not in
  `catalog.txt`, is a finding. Such an ID suppresses nothing; the valid IDs of
  the same list still apply. Rule IDs are case-sensitive (`cs006` is CL015).
* Every rule can be suppressed, also the ones from the parser (e.g. CL018),
  except CL000 (the comments of broken YAML are not reliable) and CL015 (a
  broken suppression comment must not switch itself off).

## Config file

A project can adapt the linter with a file `.concourse-lint.yml` (or
`.concourse-lint.yaml`):

```yaml
rules:
  CS005: error                          # change the severity: error, warning, info
  CL018: off                            # switch a rule off
  CB002: {severity: info, min_interval: 30s}
  CS001: {extra_words: [credential]}    # options, see below
  CS004: {allowed_hosts: [git.internal]}
resource_types: [my-worker-type]        # types the workers provide
exclude: ["vendor/**", "ci/templates"]  # globs, relative to the config file
```

* **Search:** from the folder of the checked file upwards, up to and
  including the folder that contains `.git` (the Git root), or the file
  system root outside of Git. The nearest file wins; files are not merged.
  Stdin uses the current folder. `--config <file>` skips the search and
  applies to every input. Both `.concourse-lint.yml` and
  `.concourse-lint.yaml` in one folder is an error.
* **rules:** a rule ID with a severity (`error`, `warning`, `info`), `off`,
  or a mapping with an optional `severity` and the options of the rule. A
  configured severity applies in both directions (stricter or milder). Every
  rule can be switched off except CL000. Options:
  * CS001 `extra_words`: more words that make a key secret-like (compared to
    the last word of the key, case-insensitive).
  * CS004 `allowed_hosts`: host names that may use `http://`/`git://`
    (exact, case-insensitive), in addition to the loopback hosts.
  * CB002 `min_interval`: the shortest `check_every` that is fine (a Go
    duration, default `1m`).
* **resource_types:** more built-in types for CL004 (types the workers
  provide without a definition in the pipeline).
* **exclude:** globs relative to the folder of the config file (`*` within a
  folder, `**` across folders). A pattern that matches a folder excludes
  everything in it. Excluded files are not checked, also when they are named
  on the command line (e.g. by pre-commit); stdin is never excluded.
* **Errors:** an unknown key, an unknown rule ID, an invalid severity, an
  unknown option or a wrong value is an error: the CLI prints it as
  `file:line:col: message` (the position of the offending key or value) and
  exits with 2 without checking anything. Suppression comments still apply on top
  of the config.
* **Spec cases:** `spec/config/<scenario>/` holds a `.concourse-lint.yml`
  and pipelines with `# want` (severities as configured). A pipeline that the
  config excludes contains `# want-excluded`. A config that must be rejected
  contains `# want-config-error`.

## CLI contract (for the black-box tests)

* Usage: `concourse-lint check [--format text|json|sarif|github] [--fail-on error|warning|info] [--config <file>] <file|folder|->...`.
  Flags may come before, between or after the files; after `--` every
  argument is a file name. `-` reads stdin, which is reported as `<stdin>`.
* A folder is checked recursively: every `*.yml`/`*.yaml` in it that has a
  top-level `jobs:` key with a `plan:` key in it, in sorted order. Other YAML
  files (task files, var files, BOSH manifests, CI files of other tools)
  folders whose name starts with a dot (`.git`, `.github`) and symlinks
  whose target is missing (a submodule not checked out) are skipped.
  ytt, spruce, ERB and Jinja templates (CL022) and pipeline fragments
  (CL023) are skipped too, unless the rule is off or
  suppressed for the file. A file given by name is always checked.
* Text output: one line per finding, `file:line:col: SEVERITY RULE message`,
  with the severity in lowercase and the hint, if any, in parentheses after
  the message.
* JSON: `{"findings": [{"file", "rule", "severity", "pos": {"line", "col", "end_line", "end_col"}, "message", "hint", "suggestion", "docs"}], "errors": [{"file", "message"}]}`.
  `end_line`/`end_col` are right after the last character of the key, value
  or token the finding is about (a finding about a whole block covers only
  its first key); a finding without such a text has end = start.
  `docs` is the link to the rule page in `docs/rules/`. `suggestion` is
  the exact text that replaces the range `pos` to fix the finding (the
  closest name of a "did you mean" hint, with the quotes of a quoted
  value, e.g. `"repo"` for `"rep"`).
  Every field is always present (`hint` and `suggestion` are `""` when
  there is none, empty lists are `[]`). `errors` lists inputs that could not be read; the other
  files are still checked.
* SARIF 2.1.0 for GitHub Code Scanning; every rule carries a `helpUri` to
  its page in `docs/rules/`. Regions have `endLine`/`endColumn` when the
  finding has an end. A finding with a suggestion has a `fixes` entry: one
  replacement of its region by the suggestion, described as
  `Change to "x"`.
* GitHub (`--format github`): annotations in a GitHub Actions run, without
  code scanning. See "GitHub format" below.
* Exit 0: nothing at or above `--fail-on` (default: `error`). Exit 1: findings
  at or above `--fail-on`. Exit 2: usage error, a broken config file (nothing
  is checked then), an input could not be read (after the other files were
  checked), or the report could not be written.
* `--config <file>` uses that config file for every input instead of the
  search (see "Config file").

### GitHub format

`--format github` writes GitHub Actions workflow commands to stdout, which
GitHub turns into annotations on the lines of the pull request. Nothing else
is written to stdout; without findings and errors stdout is empty.

* **Paths:** when `GITHUB_WORKSPACE` is set, a path inside it is written
  relative to it, with `/` as separator (GitHub matches annotations against
  paths relative to the repository root). A path outside it, `<stdin>`, or
  any path when the variable is not set stays as given. This applies to the
  annotations and to the summary.
* **One line per finding**, in the order of the text output:
  `::LEVEL file=FILE,line=LINE,col=COL,endLine=END_LINE,endColumn=END_COL,title=concourse-lint RULE::MESSAGE`.
  LEVEL is `error`, `warning` or `notice` (for `info`). The end is the same as
  `end_line`/`end_col` in JSON (end = start without an end). When the finding
  spans several lines, `col` and `endColumn` are left out (GitHub accepts
  columns only within one line). MESSAGE is the message with the hint in
  parentheses, as in the text output.
* **`<stdin>`:** no `file`, `line`, `col`, `endLine` or `endColumn`; the
  message starts with `<stdin>:LINE:COL: ` instead, so the position is not
  lost.
* **Inputs that could not be read:** after the findings, one
  `::error file=FILE,title=concourse-lint::MESSAGE` each (in addition to
  stderr).
* **Escaping:** in MESSAGE `%`, `\r` and `\n` become `%25`, `%0D` and `%0A`;
  in property values (FILE, title) also `:` and `,` become `%3A` and `%2C`.
* **Limit:** GitHub shows only 10 annotations per level and step and 50 per
  job (checked in a pull request on 2026-11-26: 45 findings in one step gave
  10 errors, 10 warnings, 10 notices; 100 shown over six steps gave 50). The
  step summary below lists all findings.

**Step summary:** with `--format github` and the variable
`GITHUB_STEP_SUMMARY` set, the CLI also appends a Markdown summary to the
file it names (append, never overwrite: several runs in one step all show
up). Other formats never write a summary.

```markdown
### concourse-lint

2 errors, 1 warning, 1 info in 3 files.

| Location | Severity | Rule | Message |
|---|---|---|---|
| [`ci/pipeline.yml:13:10`](https://github.com/org/repo/blob/SHA/ci/pipeline.yml#L13) | error | [CL002](https://…/docs/rules/CL002.md) | resource 'rep' is not defined (did you mean 'repo'?) |

- Could not read `missing.yml`: open missing.yml: no such file or directory
```

* **Count line:** per severity in the order error, warning, info, counts of
  zero left out; `error`/`warning`/`file` get a plural `s`, `info` does not.
  The number of files is the number of checked files (with and without
  findings). Without findings: `No findings in 3 files.` and no table.
* **Location:** `file:line:col` as code (a `|` in it as `\|`, which would
  otherwise end the table cell). It links to the line in the
  repository (`$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/blob/$GITHUB_SHA/FILE#LLINE`,
  every path segment URL-escaped) when these three variables are set and the
  path was made relative to `GITHUB_WORKSPACE`; otherwise it is plain code.
  `<stdin>` never links.
* **Rule:** the rule ID linked to its page in `docs/rules/`.
* **Message:** message and hint as in the text output, escaped so that
  Markdown shows it as written: a backslash before `\`, `` ` ``, `*`, `_`,
  `[`, `]` and `|`; `&`, `<` and `>` as `&amp;`, `&lt;` and `&gt;`; line
  breaks become spaces.
* **At most 500 rows** (GitHub allows 1 MiB per step); after the table one
  line `… and N more findings (use --format json to see all).` (`finding`
  without `s` for N = 1).
* **Inputs that could not be read:** at the end, one list item each:
  ``- Could not read `FILE`: MESSAGE`` (MESSAGE escaped as above).
* Blocks (heading, count line, table, the "more findings" line, the list)
  are separated by one empty line; the summary ends with a newline.
* If the summary cannot be written, the CLI reports it on stderr and exits
  with 2 ("the report could not be written"); the annotations on stdout are
  already written then.

### Rules and explain

```
concourse-lint rules   [--format text|json] [--config <file>]
concourse-lint explain [--format text|json] [--config <file>] <ID>
```

`rules` lists every rule of `catalog.txt`, sorted by ID. `explain` shows
one rule; the ID is case-insensitive. Flags may come before or after the
ID. The output is the same as the MCP tools `list_rules` and
`explain_rule`.

* **Config file:** searched as for stdin (from the current folder up to
  the Git root); `--config <file>` uses that file. Without a config file
  the default severities apply. A broken config file is exit 2 with
  `file:line:col: message`, and nothing is written to stdout.
* **Severity:** `severity` is the effective severity: the configured one,
  `off` for a rule that is switched off, else the default.
  `default_severity` is the severity of `catalog.txt`.
* **JSON of `rules`:** `{"rules": [{"id", "severity", "default_severity",
  "title", "docs"}]}`; `docs` is the link to the rule page.
* **JSON of `explain`:** `{"id", "severity", "default_severity", "title",
  "checked", "example", "fix", "suppress", "docs"}`. `checked`, `example`,
  `fix` and `suppress` are the sections "What is checked?", "Example",
  "Fix" and "Suppress" of the rule page in `docs/rules/`, as Markdown.
  Relative links in them are made absolute (against the address of the
  rule pages), because the reader has no repository to resolve them in.
* JSON is indented by two spaces and ends with a newline; the same text
  is the text block of the MCP result.
* **Text of `rules`:** one line per rule: ID, severity and title in
  aligned columns. A severity that differs from the default reads
  `error (default: warning)`, also `off (default: error)`. After a blank
  line: `Severities from <config file>.` when a config file was used, then
  `Details: concourse-lint explain <ID>`.
* **Text of `explain`:** the line `<ID>: <title>`, the line
  `Severity: <severity>` (with `(default: …)` as above), then the sections
  "What is checked?", "Example", "Fix" and "Suppress", each under its
  heading, then the link to the rule page. Markdown is rendered for the
  terminal: headings are followed by a line of `-`, `**` is dropped,
  inline code keeps its backticks, code blocks are indented by four
  spaces, links read `text (url)`, list items start with `- `, and
  paragraphs and list items are wrapped at 80 columns (code blocks and
  words longer than a line are never broken).
* **Colors:** only when stdout is a terminal, `NO_COLOR` is not set (or
  empty) and `TERM` is not `dumb`; on Windows only when the console
  accepts ANSI sequences. Colors only add styles: without the escape
  sequences the text is the same as without colors. Only bold, underline
  and the 16 basic colors; all decoration is ASCII.
* **Exit codes:** 0 on success. 2 for a usage error (unknown flag or
  format, no ID or more than one, an argument for `rules`), an unknown rule
  ID (on stderr, with a "did you mean" hint when one is close and a pointer
  to `concourse-lint rules`), or a broken config file.

### Licenses

`concourse-lint licenses` prints the license texts of the third-party
software in the binary (the Go standard library and every Go module it is
built from), so that they come with every copy of the binary, also one
from `go install`. The text is `cmd/concourse-lint/third_party_licenses.txt`,
written by `scripts/third-party-licenses.sh`; every module that Go records
in the binary appears in it with its version. The npm package ships the
same text as `THIRD_PARTY_LICENSES.txt` (it has no npm dependencies of its
own). Exit 0; an argument is a usage error (exit 2, nothing on stdout).

## Language server contract

`concourse-lint lsp` speaks LSP 3.17 on stdin/stdout (`--stdio` is accepted
and changes nothing). Tests: `internal/lsp` (every case of `spec/` through the
protocol) and the black-box tests in `cmd/concourse-lint`.

* **Lifecycle:** `initialize` (once), `initialized`, `shutdown`, `exit`.
  Requests before `initialize` fail with -32002, after `shutdown` with
  -32600, unknown requests with -32601; unknown notifications are ignored.
  Exit code 0 after `shutdown` and `exit`, 1 when `exit` comes without
  `shutdown` or the stream closes. Responses from the client (to the
  server's own requests) are read and never answered.
* **File watcher:** when the client offers
  `workspace.didChangeWatchedFiles.dynamicRegistration`, the server answers
  `initialized` with a `client/registerCapability` request for
  `workspace/didChangeWatchedFiles` with the watchers
  `**/.concourse-lint.yml` and `**/.concourse-lint.yaml` (two patterns, not
  `{yml,yaml}`, since not every client knows braces). Without that
  capability it registers nothing; the client then has to send the events on
  its own (the VS Code extension does).
* **Documents:** full text sync (`textDocumentSync.change` = 1). A document
  is checked 150 ms after its last change; a result for an outdated version
  is dropped. Diagnostics carry the document version. Closing a document
  clears its diagnostics.
* **Which documents:** a document is checked when its language ID is
  `concourse-pipeline` (the user marked it) or when its text looks like a
  pipeline (a `plan:` in the top-level `jobs:`, as for folders in the CLI).
  Other documents get an empty list.
* **Positions:** `positionEncoding` is `utf-16`. Ranges cover the key, value
  or token of the finding (see the JSON `end_line`/`end_col`).
* **Diagnostic:** severity error = 1, warning = 2, info = 3; `code` is the
  rule ID, `codeDescription.href` its page in `docs/rules/`, `source` is
  `concourse-lint`, the hint is appended to the message as ` (hint)`.
* **Config file:** found and applied per document as in the CLI (`file:`
  URIs only; an `untitled:` document uses the defaults), cached per folder,
  read again on `workspace/didChangeWatchedFiles` (see "File watcher"). A
  broken config file shows its error in that file (at the key or value) and
  in each pipeline one info "not checked: the config file is broken: …";
  nothing else is checked then.
  An open `.concourse-lint.yml` is checked with its text in the editor.
  Excluded documents get an empty list.
* **Go to definition and find references** (`textDocument/definition`,
  `textDocument/references`, M6) for resources, jobs and resource types,
  in documents checked as pipelines (others answer `null`). References:
  a get or put names a resource (by `resource:` or by its step name),
  `groups[].resources` names resources, `passed:` and `groups[].jobs` name
  jobs, `type:` of a resource, of a resource type and of an
  `image_resource` names a resource type. A glob in `groups[].jobs` refers
  to every job it matches. Names with `((var))` refer to nothing; a
  built-in resource type has no definition. The cursor may stand right
  after the name. No result is `null`. Cases: `spec/editor/navigation/`.
* **Hover** (`textDocument/hover`, Markdown, M6): on a finding, the rule
  from its page in `docs/rules/` (title, "What is checked?", "Fix", link;
  the pages are embedded in the binary). On a name, a summary: a resource
  with `type`, `check_every` and the jobs that get or put it; a job with
  its number of steps (nested steps and hooks count), its groups (also
  through globs) and the jobs in its `passed:`; a resource type with its
  `type` or "built-in" and what uses it; a glob with the jobs it matches.
  Nothing from `source:` or `params:` (they may hold secrets). A name that
  is defined nowhere has no summary. Both parts are separated by a rule;
  nothing is `null`. Cases: `spec/editor/hover/` (the summary in its
  one-line form `kind name | fact | fact`).
* **Quick fixes** (`textDocument/codeAction`, kind `quickfix`, M6) for the
  findings that touch the range, computed again on the server:
  * `Change to "x"` (preferred) when the finding has a suggestion: the
    closest name of a "did you mean" hint, for findings whose range is
    exactly the misspelled word (CL001/CL018 unknown keys, CL002, CL003,
    CL004, CL010, CL012, CL019 load_var format, CL020). The range is
    replaced by the suggestion, which keeps the quotes of a quoted value
    (the same text as `suggestion` in the JSON). Cases:
    `spec/editor/quickfix/`.
  * `Suppress CLxxx on this line`: adds the rule to an `ignore` comment
    that stands alone on the line above, or inserts
    `# concourse-lint:ignore CLxxx` above with the line's indentation. It
    is only offered when a check of the changed text shows the finding
    gone.
  * `Suppress CLxxx in this file`: inserts
    `# concourse-lint:ignore-file CLxxx` as the first line.
  * No suppression for CL000 and CL015. The CLI has no `--fix` (principle
    10).
* **Completion** (`textDocument/completion`, M6). The place at the cursor
  comes from the text (indentation for block style, the cursor's line for
  `[a, b]` and `{a: b}`), so it works while the YAML is broken; names come
  from the last version of the document without a syntax error. Items
  replace the word before the cursor (a `textEdit`), sorted by label:
  * keys of the object at the cursor that are not there yet, inserted as
    `key: ` (from the parser's field tables); in a step the keys of its
    kind and the modifiers, in a step without a kind the step kinds and
    the modifiers; nothing in free-form blocks (`source:`, `params:`, …);
  * resources after `get:`, `put:`, `resource:` and in
    `groups[].resources`; jobs in `passed:` (without the job itself) and
    in `groups[].jobs`; resource types (own and built-in) after `type:`;
  * the keywords of `version:` (get), `inputs:` (put) and `format:`
    (load_var);
  * the artifacts available at the step for `image:`, `input_mapping`
    values and the list in `inputs:` of a put, and as `name/` for
    `file:`.
  Cases: `spec/editor/completion/`.
### Editor cases (spec/editor/)

One folder per editor feature, each file a clean pipeline (TestSpec checks
it too) with marker lines: a comment whose `^` points at the cursor on the
nearest line above that is no marker line, then the kind and the expected
answer:

```yaml
  - get: repo
#         ^ definition 5:9
#         ^ references none
```

Positions are `line:column` of the start of the name, in the order of the
file; `none` is no result. References are listed without the definition.
A hover expects a quoted one-line summary, e.g.
`^ hover "resource repo | type: git | used by: build (get)"`. A fix
expects the suggestion of the finding at the cursor, e.g. `^ fix "repo"`,
or `none`; the file then has `# want` lines for those findings.
Completion has three kinds: `^ complete a b` expects exactly these labels
(sorted), `^ complete-has a b` at least these, `^ complete-not a b` none of
them.

## MCP server contract

`concourse-lint mcp` is an MCP server for AI agents on stdin/stdout. It takes no arguments (any argument is a usage error, exit 2). Tests:
`internal/mcp` (every case of `spec/` through the protocol) and the
black-box tests in `cmd/concourse-lint`.

* **Transport:** JSON-RPC 2.0, one message per line (no `Content-Length`
  header, unlike the LSP). Logs go to stderr only. A line that is not JSON
  gets error -32700, a JSON array (batch) or a message without `method` gets
  -32600, a line longer than 64 MiB is dropped with -32700. The server ends
  with exit code 0 when stdin closes.
* **Protocol versions:** `2025-11-25`, `2025-06-18`, `2025-03-26` and
  `2024-11-05`. `initialize` answers with the version the client asked for
  when it is one of these, otherwise with `2025-11-25`.
* **Lifecycle:** `initialize` (once; a second one fails with -32600),
  `notifications/initialized`. Requests before `initialize` fail with
  -32002, except `ping`. `ping` answers `{}`. Unknown requests fail with
  -32601; unknown notifications (also `notifications/cancelled`) are
  ignored. Requests are answered one after the other, in order.
* **initialize result:** `serverInfo` `{"name": "concourse-lint", "version":
  <version>}`, capabilities `{"tools": {"listChanged": false}}`, and an
  `instructions` text: check a Concourse pipeline with `check_pipeline`
  after writing or changing it, fix findings with severity `error` first,
  use `suggestion` when it is not empty, and ask `explain_rule` about a
  rule.
* **Tools** (`tools/list`, no pagination). All three have the annotations
  `readOnlyHint: true`, `idempotentHint: true`, `openWorldHint: false`
  (protocol `2025-03-26` and later). An unknown tool name fails with
  -32602. Arguments that do not fit the input schema give a tool error
  (`isError: true`, the problem as text), so the agent can correct itself.
* **Results:** from protocol `2025-06-18` on, every tool has an
  `outputSchema`, and a result carries the JSON object in
  `structuredContent` and the same JSON as one text block in `content`.
  For older versions only the text block. A tool error has `isError: true`
  and one text block with the message.
* **`check_pipeline`:** arguments `text` (a string), `filename` (a string,
  only with `text`) and `paths` (a non-empty list of strings); exactly one
  of `text` and `paths`.
  * The result is exactly the JSON document of the CLI (see "CLI
    contract": `findings` and `errors`, every field present). For the same
    files, `check_pipeline` with `paths` gives the same document as
    `concourse-lint check --format json` in the same working directory.
  * `text` is checked as one pipeline, also when it does not look like
    one. Its `file` in the result is `filename`, or `<text>` without it.
    The config search starts in the folder of `filename` (relative to the
    working directory), or in the working directory without it. Nothing
    is read from disk except the config file, and `exclude` does not apply
    (like stdin). A `filename` outside the working directory is a tool
    error, as for `paths`.
  * `paths` are files or folders, absolute or relative to the working
    directory the server was started in, handled as in the CLI (folders
    recursively, only files that look like pipelines; config search per
    file; `exclude` applies, also to files named directly). `file` is the
    path as given (in a folder: joined with the folder as given).
  * Only paths inside the working directory are allowed, after resolving
    symlinks. A path given in `paths` that lies outside is a tool error,
    and nothing is checked. A file found in a folder that resolves to
    outside the working directory (a symlink) is not read and goes into
    `errors`. A path that cannot be read (e.g. it does not exist) goes
    into `errors`; the other files are still checked.
  * The config search goes up to the Git root as in the CLI, also above
    the working directory. A broken config file is a tool error with
    `file:line:col: message`, and nothing is checked.
  * Findings are never a tool error.
* **`list_rules`:** no arguments. The result is exactly the JSON document
  of `concourse-lint rules --format json` in the working directory (see
  "Rules and explain"): every rule with its effective severity under the
  config file found from the working directory, read at every call.
* **`explain_rule`:** argument `id` (a rule ID, case-insensitive). The
  result is exactly the JSON document of `concourse-lint explain --format
  json <id>` in the working directory. An unknown ID is a tool error with
  a "did you mean" hint when one is close.
* For both, a broken config file is a tool error with
  `file:line:col: message`, as for `check_pipeline`.

## Verified with fly

`scripts/check-with-fly.sh` compares the cases against `fly validate-pipeline`.
Run it again after changing cases. Results with fly 8.3.4 (2026-11-24):

* Every case in `valid/`, `suppress/` and every `negatives.yml` is accepted.
* Unquoted `yes`/`on` are accepted as booleans (`valid/08-yaml11-booleans.yml`).
* A quoted `"true"` in a boolean field is rejected ("cannot unmarshal string
  into ... of type bool"), so it is CL001, not a security finding.
* Unknown fields (corrected after M0): rejected only in steps, in an inline
  task `config` and in `across` variables (CL001). Everywhere else fly
  accepts them ("looks good") and only `--strict` rejects them (CL018,
  warning). Even `--strict` accepts unknown fields below a task `config`
  (e.g. in `run:`).
* Required fields (corrected after M0): fly rejects a missing `name` or `type`
  of resources, resource types and prototypes, a missing `name` of jobs and
  groups, a missing `name`/`type` of var sources, an inline task `config`
  without `platform` or with a nameless input or output, `set_pipeline` and
  `load_var` without `file`, `run` without `type`, `across: []` and an empty
  step name. It accepts a job without `plan` (hence CB003), a resource
  without `source`, a config without `run` or image, an `image_resource`
  without `type`, and an `across` variable without `var` or `values`.
* Values with a fixed format: fly rejects a `check_every` that is no Go
  duration (`soon`, `1d`, `10 m`, the number `10`; `never` and `""` are
  fine), a cache `ttl` that is no duration, an `across` `max_in_flight` other
  than a number or `all`, and a `version` that is a list. It accepts without
  a check: `timeout: soon` and `check_timeout: soon` (the step or check fails
  when it runs), load_var `format: xml` (fails when it runs), `version: lates`
  and put `inputs: al` (silently ignored), and any `platform`.
* String fields outside steps (`name`, `icon`, `serial_groups` …): fly turns
  an unquoted number or boolean into text, in its own spelling: `yes` →
  `"true"`, `1.10` → `"1.1"`, `0777` → `"511"`, `0x1F` → `"31"`, `1e3` →
  `"1000"`, `1234567.0` → `"1.234567e+06"`, `1.23456789` → `"1.2345679"`
  (32-bit precision: a `passed:` with `"1.23456789"` no longer finds the
  job). Inside steps the same values are rejected (`get: 123`, `file: 123`).
* CL005: repeated `get` names in a job (also with `resource:`, inside
  `in_parallel` and in hooks) are rejected ("repeated name"), and so are
  repeated `load_var` names ("repeated var name"), duplicate var sources and
  duplicate prototypes. Repeated `task`, `put` and `set_pipeline` names are
  accepted.
* CL002 in groups (2026-11-25): a group that lists an unknown resource is
  rejected ("group 'all' has unknown resource 'rpo'"). The entries are exact
  names: `"r*"` is rejected as well, although a resource `repo` exists.
* CL006: an unused resource is an error ("resource 'x' is not used").
* CL009: rejected ("job 'x' does not interact with resource 'y'").
* CL010: a glob that matches no job is an error ("no jobs match 'x'").
  `build{` with a job `build` makes fly 8.3.4 panic ("slice bounds out of
  range", in `github.com/gobwas/glob`), checked 2026-11-25.
* CL016: a job in no group is an error ("job 'x' belongs to no group").
* `syntax/duplicate-key.yml`: duplicate keys are rejected, not silently
  overwritten.
* Anchors (`syntax/`): an unknown anchor, an alias above its anchor, an
  anchor that contains itself and an alias bomb ("excessive aliasing") are
  rejected. An alias refers to the latest anchor definition above it. Merge
  keys (`<<`): of two equal keys the one further down wins, also against a
  key written in the mapping itself; in `<<: [*a, *b]` the first anchor wins.
* CL017: all cases are accepted, and `--output` shows the changed values
  (`1.10` → `1.1`, `0022` → `18`, `1.20` → `1.2`); every value in
  `negatives.yml` comes out unchanged. `.inf` and `.nan` make fly fail on the
  whole file ("unsupported value: +Inf").
* fly accepts everything that needs data flow or a heuristic (artifacts,
  `((.:var))`, security, best practice): these rules go beyond fly.
* CL021 (2026-11-25): fly accepts a literal tab in plain, single-quoted and
  double-quoted strings and in block scalars, also inside a flow list
  (`args: ["-c", "a<TAB>b"]`). goccy v1.19.2 read a tab in a double-quoted
  string wrongly (false CL000, goccy issue #948); fixed in the fork
  `github.com/mfast99/go-yaml` (goccy PR #949).
* CL022 (2026-11-26): fly accepts the negatives and four of the eight
  templates (`spruce-grab`, `spruce-quoted`, `ytt-annotation`, `ytt-value`):
  to fly a template is often a valid pipeline with odd values, so fly
  cannot tell a template from a pipeline.
* YAML edge cases (2026-11-26): fly accepts `valid/09` to `valid/14` (an
  empty block scalar before a comment, a multi-line value in a flow mapping,
  `|2-` with a blank last line, a `- ` line inside a multi-line plain value,
  an empty `!!str`, a byte order mark) and rejects
  `syntax/value-on-key-column.yml` ("could not find expected ':'"). goccy
  v1.19.2 got all seven wrong; the fork `github.com/mfast99/go-yaml` carries
  the open goccy PRs that fix them (#942, #953, #975, #996, #999, #907, #955).
  fly also accepts `valid/15` and `valid/16` (a `%YAML` or `%TAG` directive
  before `---`): goccy returns a directive as a document of its own, so we
  read the first document that is not a directive (was a false CL001).
* CL022 ERB and Jinja cases (2026-11-26): fly accepts `erb-value` and
  rejects the other three.
* CL023 (2026-11-26): fly accepts the negatives and rejects both fragments
  (the get and put name undefined resources).
* CL024 (2026-11-26): fly accepts all cases: it reads only the first
  document. One exception found while writing the cases: yaml.v2 scans a
  little past the `---`, so a scanner error right at the start of the second
  document (`foo: [` after a multi-line plain scalar) makes fly reject the
  file, while CL024 only warns. The real-world cases (a Dockerfile after
  `---`) are accepted.
* CS001 and CS002 negatives with placeholders (2026-11-26): accepted.

## Verified with a real Concourse

fly cannot answer questions about artifacts, so these were run on a local
Concourse 8.3.4 (`docker compose`, 2026-11-24):

* CL007/CL011, siblings in `in_parallel`: without `limit`, a task whose input
  comes from a slower sibling errors with "missing inputs: out", and a task
  whose `file:` comes from a sibling errors with "unknown artifact source:
  'repo' in file path". With `limit: 1` the steps run one after another in
  list order and both work. `limit: 2` with two steps behaves like no limit.
  A sibling that happens to finish first (e.g. a cached get) can make it work
  by chance; that race is exactly why CL007/CL011 report it.
* Artifacts and `across`: every iteration runs in its own artifact scope
  (`exec/across_step.go`, `NewLocalScope`). An output of a step with
  `across` is not available after it ("missing inputs: out"); inside an
  `across` on `do`, the nested steps see each other's artifacts as usual.
  Hooks wrap `across` (they are outside it), so they don't see its
  artifacts either.
* Hooks: an artifact produced in an `on_success` hook is available to the
  following steps of the plan.
* `try` with an `on_failure` hook inside: the artifact the hook produces is
  available to the steps after the `try`.
* A `load_var` with `across` defines its variable only inside that step;
  afterwards `((.:name))` fails ("undefined vars: .:answer").

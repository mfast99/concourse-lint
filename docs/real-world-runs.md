# Real-world runs

Before a release, concourse-lint checks a large collection of public
Concourse pipelines. Every false positive becomes a spec case first, then a
fix. Run it with:

```sh
scripts/real-world-run.sh
```

The corpus is `scripts/real-world-repos.txt` (repositories at pinned
commits). The script clones only their YAML files into
`~/.cache/concourse-lint/real-world`, checks every repository as a folder and
prints the findings per rule. Nothing from these repositories is copied into
this repository; secret values found there are never quoted here.

## 2026-11-26

**Corpus:** 611 repositories with about 3,200 pipelines (2,428 distinct
files). They were found with GitHub code search for `resource_types`,
`in_parallel`, `load_var`, `set_pipeline`, `groups`, ytt `#@` and spruce
`(( grab` next to `jobs`/`plan`. Forks and copies are left out. The corpus
also includes the repositories of the first run on 2026-11-24
(concourse/ci, concourse/examples, concourse/docs, pivotal-cf/pcf-pipelines,
starkandwayne/concourse-tutorial, cloudfoundry). The run takes about 10
seconds once the repositories are cached.

**Findings per rule** (after the fixes below):

| Rule | Findings | Rule | Findings | Rule | Findings |
|---|---:|---|---:|---|---:|
| CL000 | 547 | CL011 | 5 | CS002 | 11 |
| CL001 | 1519 | CL012 | 4 | CS003 | 404 |
| CL002 | 51 | CL014 | 39 | CS004 | 16 |
| CL003 | 2 | CL016 | 24 | CS005 | 1717 |
| CL004 | 74 | CL017 | 2 | CS006 | 933 |
| CL006 | 36 | CL018 | 60 | CS007 | 12 |
| CL007 | 13 | CL024 | 9 | CS008 | 1185 |
| CL008 | 6 | CS001 | 49 | CS009 | 48 |
| CL010 | 29 | CB001 | 154 | CB002 | 14 |
|  |  |  |  | CB003 | 18 |

CL019, CL020 and CL021 have no findings; CL022 and CL023 have none in
folders, because templates and fragments are skipped there. Named one by
one, CL022 fires on 477 files (370 spruce, 107 ytt) before ERB and Jinja
were added, always at a real template marker.

**Checked:** every file with a CL000, CL001, CL003, CL006, CL010 or CL016
was compared with `fly validate-pipeline`. fly rejects all of them except 4
CL000 files (see open questions). CL000 is mostly the old `{{var}}` syntax
and `aggregate:` (removed in Concourse 7, CL001 with a hint). CL002 to
CL017, CS002, CS004, CS007 and CB002 were read one by one; CL018, CS001,
CS003, CS005 and CB001 by group of message. The config file
(`resource_types`, `exclude`, rules off, severities, an unknown rule ID) was
tried on concourse/concourse, alphagov-mirror/paas and
sparameswaran/offline-gen. No crash.

**Fixed:**

* CL008: a quoted name with a dot, `((.:"local.var"))`, was read as the
  undefined name `"local` (Concourse's own testflight fixture).
* Folder detection: a CircleCI config (a step `terraform-plan:`) and BOSH
  deployment manifests (`plan:` as a property) counted as pipelines. Now
  `plan:` only counts inside the top-level `jobs:`.
* CS001: `password: x-oauth-basic` (GitHub token auth, 10 resources).
* CS001: placeholders (`REDACTED` 83 times, `<replace-me>`, `YOUR-…`,
  `REPLACE-…`) are no secret. Artifact paths in secret fields
  (`BOSH_GW_PRIVATE_KEY: ssh-private-key/id_rsa`) stay a finding.
* CS002: a private key frame around a placeholder (`###`) or a `((var))`.
* Folders: a dangling `*.yml` symlink (a submodule not checked out, an Emacs
  lock file) gave exit code 2 for the whole folder; it is skipped now.
* CL022 also recognizes ERB and Jinja templates (26 files got CL000 with the
  hint for the old `{{var}}` syntax).
* New CL023: a pipeline fragment (get or put steps, no `resources:`, merged
  by spruce or aviator) gets one info finding instead of a CL002 for every
  step, and is skipped in folders.
* New CL024: Concourse reads only the first YAML document. Only that one is
  checked now; content after `---` gets one warning instead of a CL000 in a
  document fly never reads.

**Real bugs found in public pipelines** (examples): a GitHub token, Slack
webhooks and private keys in plain text (CS002); a job without the
`concourse-image` get in concourse/ci (CL007, as in the first run);
`background_image` one level too high, so the `display:` block is empty
(CL018); a trailing `\` after `((…))` in a private key (CS001); task
inputs and `input_mapping` artifacts that the job never fetches (CL011,
CL012); `tag: 1.0` that becomes `1` (CL017).

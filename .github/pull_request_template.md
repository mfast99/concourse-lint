<!--
Title: Conventional Commit, e.g. "feat(CL002): report unknown resources".
It becomes the commit message on develop (squash merge).
Target branch: develop.
-->

Closes #

## What and why

<!-- Short description of the change and the reason for it. -->

## Checklist

- [ ] There is an issue for this change and it is referenced above
- [ ] Spec case added or changed first (`spec/`), and it was red before the fix
- [ ] `make check` passes locally
- [ ] Rule docs updated (`docs/rules/`), if a rule changed
- [ ] `CHANGELOG.md` updated under "Unreleased", if user-facing

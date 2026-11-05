---
title: Markdown Runner AB17AF01 Executor Memory
---

## Outcome

* Applied the operator-approved 084-DL decision for stash AB17AF01: append
  `!.git/**` to the pinned Markdown lint invocation in both runners
* Changed `scripts/md-lint.ps1` and `scripts/md-lint.sh` only; no rule, pin,
  source glob, Git metadata, or unrelated work was changed
* The literal Windows gate and Git Bash runner both exited 0 with 7,731 files
  and zero issues; Git Bash syntax check exited 0
* Focused Markdown and plugin integration contracts and the full CLI build
  passed with Go 1.24.0
* The previously reported full-suite timeout and fixture failures remain
  unresolved; the full suite was not retried and remains required before merge

## Handoff

Preserve the other authors' uncommitted changes. This execution does not
claim a shipment, merge a PR, or change Go source. The existing installer
and lockfile findings remain out of scope.

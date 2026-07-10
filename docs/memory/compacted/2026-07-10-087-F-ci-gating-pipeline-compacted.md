---
chunk_strategy: h1-h2-h3
description: Compacted Orchestrator memory for CI gating pipeline work and shipment-add parity verification.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-087-F-ci-gating-pipeline-compacted.md
title: Compacted memory - 087-F CI gating pipeline and shipment-add parity
---
## Summary

This orchestrator memory recorded the CI gating pipeline (`087-F`) and a separate shipment-add parity verification. PR #189 merged the workflow-gating feature, while stash `8A87C3A7` was verified as already implemented on main and archived without a new PR.

## Archived originals

* `docs/archive/memory/orchestrator-087-ci-gating-pipeline-memory.md`

## Decisions and outcomes

* CI workflow gating used `dorny/paths-filter` with a fail-safe denylist and `predicate-quantifier: every`; the default `some` semantics would make a leading `**` match every file and defeat negations.
* Heavy test and drift jobs were gated at job/step level while docs-lint stayed always-on.
* Repository branch protection was actually a ruleset requiring approval, last-push approval, `test (1.24)`, thread resolution, and merge-method=merge; admin bypass was used only after substantive gates passed.
* PR #189 merged by true merge commit `305bd4ff494c3b8274183563490c1bdeaaa7f778` and branch cleanup completed.

## Files and verification

* `.github/workflows/ci.yml` and `cli-reference-drift.yml` gained the `changes` job, gated heavy matrix/drift behavior, and SHA-pinned `dorny/paths-filter`.
* Copilot caught the critical `predicate-quantifier: every` issue that three pre-PR adversarial reviewers missed; fixed in `9815866`, replied, resolved, and re-review was clean.
* Gate-run path was proven by the feature PR; skip-path behavior was validated from corrected dorny semantics, not a separate live docs-only PR.
* Remaining stash entries were evaluated and left in place because they were contingent, external, or too broad for autonomous action.

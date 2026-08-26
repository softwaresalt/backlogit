---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 076-S Stage harvest docline frontmatter hardening.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-076-S-harvest-docline-frontmatter-compacted.md
title: Compacted memory - 076-S harvest docline frontmatter hardening
---
## Summary

Shipment `076-S` hardened Stage plan/harvest docs so generated implementation plans are born docline-compliant and linted before harvest. It was triggered by the invalid `doc_type: exec-plan` frontmatter that blocked 075-S CI.

## Archived originals

* `docs/archive/memory/2026-07-02-stage-076-S-harvest-docline-frontmatter-hardening.md`
* `docs/archive/memory/2026-07-02-ship-076-S-post-merge-closure-session.md`

## Decisions and outcomes

* The root cause was split between `.github/skills/impl-plan/SKILL.md` lacking docline frontmatter guidance and `.github/skills/harvest/SKILL.md` lacking a pre-commit lint gate.
* The CI entrypoint `make docs-lint` was made authoritative for self-linting; stale installed binaries are not sufficient for lint-gate parity.
* Upstream template/harness drift was deferred to `EED25928` because it was larger and partly outside workspace containment.
* PR #166 merged by true merge commit `ef9dc20468d865bbaf7d7b1e9b982ff7f4045422`; `backlogit shipment ship 076-S` archived `076.001-T`, `076.002-T`, `076-F`, and `076-S` with clean reconcile.

## Files and verification

* `.github/skills/impl-plan/SKILL.md` received the docline contract for plans.
* `.github/skills/harvest/SKILL.md` received a pre-harvest docline lint gate.
* Runtime verification showed a compliant plan lints clean while the 075-S defect replica fails with three violations.
* Operator accepted three Copilot P3 comments as won't-fix after replies and thread resolution.
* Knowledge graduation reinforced `docs/compound/2026-06-26-docline-frontmatter-contract.md`; follow-up stash `B55985DD` captured wording cleanup for `make docs-lint --path` references.

---
doc_type: memory
schema_version: "1.0"
title: "Stage PR #404/#405 Remediation — Post-Merge Closure (2026-09-03)"
---

# Stage PR #404/#405 Remediation — Closure

Follow-up PR **#410** merged to `main` via merge commit **191cf64b** (no admin
bypass). Fix commits: `3eae413f`, `5c195add`, `90749d58`. CI green; P-018
Copilot-review gate PASS for HEAD `90749d58`.

## Outcome

- Original review threads: **23** total (20 on #404, 3 on #405). **15 substantively
  fixed + resolved**; **8 left unresolved** (not falsely resolved).
- Copilot re-review of #410: **14 threads across 3 iterations, all resolved.**

## Governed backlog fixes (repo-local backlogit.exe)

- Dependency `164.002-T → 164.001-T`; `160.004-T` (U3) → all eight checks.
- S8 split: `160.002/003-T` narrowed + new `160.005–160.010-T`; `142-S` manifest updated.
- Deliberation linkage: `062/064/063-DL` embedded in queued features `155/163/164-F`;
  `061-DL` archived + informs-linked to `153-F`.
- Bounded fuzz task `158.008-T` (dep `158.001-T`) added to `140-S`.
- `165.002-T` truthful cross-repo acceptance, status `blocked`.

## Truthful review record

Genuine multi-persona cross-model plan-review re-dispatch overturned the false
uniform PASS: **BLOCKED (8 FAIL / 1 ADVISORY)**. Hardening + genuine verdicts added
to S2/S3/S4/S6/S8/S9/S10/S12/S13.

## Recorded blockers (NOT resolved — require action beyond this session)

1. **Checkpoint quarantine (5 threads)** — 3 schema-invalid checkpoints
   (`082159`, `084348`, `091629`) need destructive operator-approved quarantine
   (AFK). Files intentionally preserved.
2. **Provenance backfill (#404-3/#404-7/#404-31)** — `source_stash_id` for 25
   plain-archived entries + the `A2C91FE5` duplicate archive have no governed path
   (`stash correct` requires prior harvest; cache hand-edit forbidden). Follow-up
   `156F65EB`.
3. **136-S topology** — `PREDECESSOR_CLOSURE_INCOMPLETE` (135-S closure evidence,
   Ship-owned, P-001). 136-S must remain unclaimed.

## Captured follow-ups (untriaged)

`6FDC4A49` (checkpoint-create hardening), `156F65EB` (provenance-backfill op),
`FF6D467A` (re-author S2/S3/S4/S6/S8/S9/S10/S12).

Operator-owned `start.ps1` never modified. Out-of-scope stash `3F06493B`,
`7B71AD77` untouched.

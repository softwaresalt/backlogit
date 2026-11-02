---
chunk_strategy: h1-h2-h3
description: 'Implementation plan to compact docs/closure (87 files / ~582 KB, over the compact-context 500 KB and 40-file thresholds) by consolidating completed and stale (>14d) per-release-unit closure records into born-docline-compliant summaries that remain in docs/closure, and moving the verbose originals archive-only to docs/archive/closure, keeping the Docline frontmatter gate green.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-04-closure-docs-compaction-plan.md
title: 'Compact docs/closure archive (housekeeping)'
---

## Source

- Deliberation: `docs/decisions/2026-07-04-closure-docs-compaction-deliberation.md` (decided, promoted_to: plan; lightweight).
- Stash: `2EF8B7AD` (kind=task, priority=low) — run compact-context (target=closure) as a dedicated housekeeping unit.
- Skill contract: `.github/skills/compact-context/SKILL.md` (Phase 3, Closure).
- Prior art: `docs/memory/compacted/2026-06-27-pre-067-S-memory-compaction.md` (near-exact precedent), `docs/docline-frontmatter-authoring-guide.md` (scope boundary), `docs/closure/2026-06-26-archived-from-migration-closure.md` (frontmatter exemplar), `docs/decisions/archival-policy.md`.

## Problem frame

`docs/closure/` holds per-release-unit closure artifacts (typically triples: `*-closure.md`, `*-runtime-verification.md`, `*-compound-refresh.md`, sometimes `*-post-merge-closure.md`), currently 87 files / ~582 KB spanning 2026-04-07 -> 2026-07-04 — over both the 500 KB and 40-file compact-context triggers. The remedy: consolidate completed + stale (>14d) units into per-unit/per-range summaries that stay under `docs/closure/**`, and move the verbose originals archive-only to `docs/archive/closure/**`. Never delete; preserve the audit trail; preserve the newest unit's records.

Critical correctness boundary: **docline lint scope = `docs/**` EXCEPT `docs/memory/**` and `docs/archive/**`.** Files moved to `docs/archive/closure/**` leave lint scope (their stale `source:` is harmless). The consolidated summary that **remains** under `docs/closure/**` stays *in* scope and MUST be born docline-compliant.

## Requirements trace

| Source requirement | Implementation unit |
|---|---|
| `2EF8B7AD`: consolidate completed-shipment closure artifacts into summaries | Unit A (AC-1, AC-2) |
| `2EF8B7AD`: move verbose originals to docs/archive/closure/ (archive-only) | Unit A (AC-3) |
| Keep the Docline frontmatter gate green (summary in-scope, originals out-of-scope) | Unit A (AC-4) |
| Bring docs/closure back under the compact-context thresholds | Unit A (AC-5) |

## Implementation units

### Unit A — Consolidate + archive stale docs/closure records (docs)

- **Files (domain: docs)**: writes/updates one or a few `docs/closure/{YYYY-MM-DD}-{slug}-closure-summary.md` consolidation file(s); creates `docs/archive/closure/` and moves the verbose stale originals into it (git-tracked moves). Although the raw file count moved is large, this is a single mechanical, skill-driven housekeeping operation (one atomic milestone), not a multi-file code change — the 2-hour-rule granularity heuristic is satisfied by the single-domain, single-milestone, reversible nature of the work.
- **Approach (compact-context target=closure)**:
  1. Enumerate `docs/closure/` and classify each unit as **stale** (shipment closed AND newest record >14 days old, i.e. on/before ~2026-06-20) or **preserve** (newest completed unit and anything within the 14-day window — e.g. `2026-07-04-080-S-*`).
  2. For each stale unit (or contiguous range of old units, per the memory precedent), write a consolidated `*-closure-summary.md` capturing: what was verified, healthy/failure signals, monitoring status, follow-ups — removing verbosity, not substance.
  3. Move the verbose originals to `docs/archive/closure/<same-filename>` (mirror mapping). Never delete.
  4. Include an **archived-file index** in the summary listing every moved original (traceability).
- **Acceptance criteria**:
  - **AC-1**: Every stale, shipment-closed unit >14d old is represented by a consolidated summary in `docs/closure/` that preserves its decisions/verification substance.
  - **AC-2**: The newest completed unit's records (and any <14d records) are **preserved in place**, not compacted.
  - **AC-3**: Verbose originals are **moved** (not deleted) to `docs/archive/closure/<same-name>`; `docs/archive/closure/` is created if absent; the summary carries a complete archived-file index.
  - **AC-4**: Every summary remaining under `docs/closure/**` passes `backlogit docs lint --path docs/closure` with **0 violations** (born-compliant `doc_type: closure`, top-level `title` + `source` = own path, `docline: { ms.date, ms.topic }`, with `#`-bearing scalars single-quoted). Archived originals under `docs/archive/closure/**` are confirmed out of lint scope.
  - **AC-5**: `docs/closure/` is back under the compact-context thresholds (< 500 KB and <= 40 files) OR a documented, justified residual is recorded in the summary.
- **Execution posture**: migration-first / archive-only housekeeping — body-preserving frontmatter handling; git-tracked moves are the reversibility mechanism.
- **Atomic milestone**: `docs/closure/` compacted under threshold with 0 docline violations and no deletions; archived originals mirrored under `docs/archive/closure/`.

## Dependency graph

- Single implementation unit; no intra-feature dependencies. No cycles.

## Decisions and rationale

- **Consolidate into `docs/closure/`, archive originals to `docs/archive/closure/`**: per compact-context Phase 3; keeps a durable summary in the linted corpus while removing verbosity from it.
- **`threshold_days = 14`, preserve newest unit**: compact-context default; enough to bring the directory under budget without touching active/recent context.
- **Born-compliant summary frontmatter (not frontmatter-less like memory/compacted)**: `docs/closure/**` is *in* docline scope (unlike `docs/memory/**`); the summary must lint clean.
- **Archive-only, mirror-mapped, indexed**: satisfies `archival-policy.md` (no silent data movement; auditable trail); reversible via git.

## Risks and caveats

- **A summary breaks the Docline gate.** Mitigation: self-lint `backlogit docs lint --path docs/closure` (0 violations) before handoff; quote `#`-bearing scalars; match the closure exemplar's frontmatter shape.
- **Accidental deletion / data loss.** Mitigation: archive-only (git-tracked move), mirror mapping, archived-file index; reversible via revert.
- **Compacting an active/recent unit.** Mitigation: preserve newest unit + <14d records; only compact completed + >14d units.
- **Blast radius**: docs-only; no runtime, code, schema, or config surface touched.

## Plan Hardening Signals (REQUIRED)

- **public API, schema, or contract change** — ABSENT: docs-only; no API/schema/contract touched.
- **security, auth, permission, or compliance-sensitive behavior** — ABSENT: no security-sensitive surface; the only gate involved (Docline) is satisfied by born-compliant summaries.
- **migration, backfill, destructive data/config action, or irreversible step** — ABSENT (reversible): files are **moved archive-only** (git-tracked), never deleted; fully reversible by revert. No destructive action.
- **external integration, operator checkpoint, or external dependency** — ABSENT: no external systems; runs entirely on local docs via the compact-context skill.
- **high runtime, rollout, or rollback risk** — ABSENT: no runtime surface; rollback is a git revert of file moves.

**Requires plan hardening: no**

## Constitution Check

- **Principle I (Backlog-native, query-driven)**: N/A to execution mechanics; the work is triaged from stash `2EF8B7AD` and will be harvested into a backlog chore + task.
- **Principle II (2-hour rule / single-domain / atomic milestone)**: single docs-domain unit, one atomic milestone (compacted + archived), reversible; satisfies the granularity heuristic despite the raw file count (mechanical, skill-driven, single operation).
- **Principle III (Test/verify before done)**: verification is the docline lint (0 violations) + threshold check + `git status` rename confirmation (no deletions).
- **Principle IV (In-tree only)**: operates entirely on in-repo `docs/**`; no external/out-of-tree targets.
- **Principle V (Reversibility / no destructive action)**: archive-only, git-tracked moves, mirror-mapped, indexed; fully revertible. Never deletes.
- **Security/compliance**: no security-sensitive surface; the only gate involved (Docline) is satisfied by born-compliant summaries.

## Runtime verification and closure

- **Runtime surface changed**: none (documentation only). No CLI/API/UI/background-job behavior changes.
- **Runtime verification**: `backlogit docs lint --path docs/closure` -> 0 violations; `docs/closure/` file count/size back under threshold; `git status` confirms moves are renames (no deletions), archived originals present under `docs/archive/closure/`.
- **Operational closure artifact**: the consolidation summary itself doubles as the closure record (with the archived-file index). No monitoring/rollback trigger beyond "revert if a summary fails lint or a needed original is missing."

## Plan Review

<!-- plan-review-attempt: 1 -->

### Round 1 — verdict: PASS

A lean 2-persona panel (Constitution Reviewer, Scope Boundary Auditor) reviewed the plan.

- **Scope Boundary Auditor = PASS** (4 P3 advisories). Confirmed `Requires plan hardening: no` is justified: docs-only, archive-only, reversible, single milestone. No scope creep / YAGNI / complexity concerns. P3s: disambiguate the "migration-first" posture wording (it is archive-only, not a data migration); declare the freeze-scope (`docs/closure/**` + `docs/archive/closure/**`) so no concurrent writer clobbers the move; cross-check git rename detection so moves show as renames not delete+add; bound the AC-5 documented residual.
- **Constitution Reviewer = ADVISORY** (1 P2 + P3s, no P0/P1). P2: add an explicit "Constitution Check" section mapping the change to each principle. P3s: scope the AC-4 lint to changed files (or note the pre-existing-green presumption); note intercom events if the pack is active.
- **Resolution**: added the **Constitution Check** section above (resolves the P2). The P3 advisories are folded into the execution notes / carried forward to Ship: archive-only posture is already stated (Decisions, Risks); freeze-scope and git-rename confirmation are captured in the runtime-verification `git status` check (AC-3 + verification step); the AC-5 residual is already explicitly bounded ("documented, justified residual"). 

**Gate outcome: PASS** — cleared for harvest. No hardening required (`Requires plan hardening: no`, confirmed by review).

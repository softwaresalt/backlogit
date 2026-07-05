---
chunk_strategy: h1-h2-h3
description: 'Lightweight deliberation for stash 2EF8B7AD (compact/archive the docs/closure directory, now 87 files / ~582 KB, over the compact-context 500 KB threshold). Confirms the covering-chore scope, the staleness cutoff, the per-release-unit consolidation grouping, and the docline-scope constraint on the remaining summary.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-04-closure-docs-compaction-deliberation.md
title: 'Compact docs/closure archive: cutoff, grouping, and docline-scope decisions'
stash_id: 2EF8B7AD
decision_status: decided
promoted_to: plan
tags:
  - housekeeping
  - compact-context
  - docs
  - archive
---

## Source

- Stash: `2EF8B7AD` (kind=task, priority=low, age 0d) — "Compact/archive the docs/closure backlog: directory reached 87 files / ~581 KB, exceeding the compact-context 500 KB threshold, with stale closure records back to 2026-04-07. Run compact-context (target: closure) as a dedicated housekeeping unit... move verbose originals to docs/archive/closure/." Deferred out of 080-S post-merge closure to keep that PR scoped.
- Session: operator authorized a full autonomous Stage->Ship run (AFK); recommends the closure compaction be its own shipment and a good first Ship (lowest risk).
- Current measured state (verified this session): `docs/closure/` = 87 files, ~582 KB (over both the 500 KB and 40-file compact-context triggers); `docs/archive/closure/` does not yet exist.

## Depth: lightweight

The approach is prescribed by the `compact-context` skill (target=closure) and the accepted `docs/decisions/archival-policy.md`. There is no genuine architectural choice — only two minor parameters to fix (staleness cutoff and consolidation grouping) and one correctness constraint (docline scope). Lightweight deliberation, then promote to plan.

## Grouping rationale (Step 1.5)

Processed as a **solo group / covering chore**, separate from `D760E508` (see that deliberation). This is docs-only housekeeping in a distinct skill domain from the CI-workflow change; bundling would dilute review and enlarge blast radius. It is classified as a **chore** (maintenance housekeeping that ships as a coordinated docs-only release unit), not a feature.

## Problem frame

`docs/closure/` accumulates per-release-unit closure artifacts (typically triples: `*-closure.md`, `*-runtime-verification.md`, `*-compound-refresh.md`, sometimes `*-post-merge-closure.md`). At 87 files / ~582 KB spanning 2026-04-07 -> 2026-07-04 it exceeds the compact-context thresholds, clutters agent context, and slows triage. The remedy is to consolidate completed + stale units into per-unit (or per-range) summaries that remain in `docs/closure/`, and move the verbose originals to `docs/archive/closure/` — **archive-only, never delete**.

## Research findings

Prior art surfaced by the learnings-researcher (confidence: high):

- `docs/memory/compacted/2026-06-27-pre-067-S-memory-compaction.md`: a near-exact prior run of the same operation on `docs/memory/` — 150 files moved archive-only with a relative-path **mirror mapping** and a compaction **index**. Reuse this structure verbatim (chained by release-unit range).
- `.github/skills/compact-context/SKILL.md` (Phase 3, Closure): write the consolidated summary to `docs/closure/{YYYY-MM-DD}-{slug}-closure-summary.md`; move verbose originals to `docs/archive/closure/`; default `threshold_days=14`; never delete; preserve the most recent record per completed unit.
- `docs/docline-frontmatter-authoring-guide.md`: **docline scope = `docs/**` EXCEPT `docs/memory/**` and `docs/archive/**`.** Therefore files moved to `docs/archive/closure/**` leave the lint scope (their stale `source:` is harmless), but the consolidated summary that **remains** under `docs/closure/**` is **in scope** and MUST be born docline-compliant: `doc_type: closure`, top-level `title` + `source` (= its own repo-relative path), plus `docline: { ms.date, ms.topic }` as in the exemplar.
- `docs/closure/2026-06-26-archived-from-migration-closure.md`: exemplar closure frontmatter/body shape to match.
- `docs/compound/2026-06-26-docline-frontmatter-contract.md`: **quote** any frontmatter scalar containing `#` (closure summaries cite PR numbers), `:`, or leading specials — an unquoted `#` is silently truncated as a comment. Be body-preserving (rewrite only frontmatter).
- `docs/decisions/archival-policy.md`: archive only records tied to already-shipped/closed work; preserve the audit trail; never premature/silent.

## Decision

Promote to `impl-plan` as the covering chore **"Compact docs/closure archive (housekeeping)"** with these fixed parameters:

1. **Staleness cutoff**: `threshold_days = 14` (compact-context default). Given today is 2026-07-04, compact closure records for units completed on/before ~2026-06-20. **Explicitly preserve** the newest / most recent unit's records — e.g. `2026-07-04-080-S-release-docs-hygiene-closure.md` and any 079-S/078-S records still within the window — so active/recent context stays in place.
2. **Consolidation grouping**: per-release-unit (or per-contiguous-range of already-shipped units, following the memory precedent's chained-range style). Consolidate each completed unit's closure triple into a single summary entry; a range summary may cover several old units.
3. **Move mechanics**: move verbose originals to `docs/archive/closure/<same-filename>` (mirror mapping); create `docs/archive/closure/` if absent; **never delete**; record every moved original in a compaction **index** inside the summary for traceability.
4. **docline correctness**: the consolidated summary file(s) under `docs/closure/**` MUST pass `backlogit docs lint` (0 violations) — born-compliant `doc_type: closure` frontmatter with quoted `#`-bearing scalars. Archived originals under `docs/archive/closure/**` are out of scope and need no edits.
5. **Exit condition**: `docs/closure/` back under the 500 KB / 40-file thresholds (or a documented, justified residual), with no deletions and a green Docline gate.

Because this is docs-only, archive-only, reversible (git-tracked moves), and touches no runtime surface, it carries **no plan-hardening signals** — `impl-plan` will conclude `Requires plan hardening: no`, and plan-review runs directly.

Task scope confirmed (single width-isolated docs task under the covering chore): "Consolidate + archive stale docs/closure records via compact-context (target=closure)," with the five acceptance criteria above.

## Rejected alternatives

- **Delete stale closure files**: rejected — violates the archive-only constraint and `archival-policy.md` audit-trail requirement.
- **Frontmatter-less summaries** (as done for `docs/memory/compacted/**`): rejected for closure — `docs/closure/**` is *in* docline scope (unlike `docs/memory/**`), so the summary must carry valid frontmatter.
- **Compact everything including the newest unit**: rejected — preserve the most recent completed unit's records per compact-context and archival-policy.
- **Aggressive cutoff (< 14 days)**: rejected — no need; the default threshold already brings the directory under budget.

## Unresolved questions

- Exact file-by-file partition (which units are in-window vs. stale) is an execution detail Ship resolves by reading `docs/closure/` timestamps + shipment closure status at run time; the plan specifies the rule (>14d and shipment closed), not the enumerated list.

## Risks and mitigations

- **Risk**: a moved/consolidated summary breaks the Docline gate. **Mitigation**: born-compliant `doc_type: closure` frontmatter + self-lint (`backlogit docs lint --path docs/closure`) before handoff; archived originals leave scope.
- **Risk**: accidental deletion / data loss. **Mitigation**: archive-only (git-tracked move), mirror mapping, compaction index; reversible via revert.
- **Risk**: compacting an active/recent unit's records. **Mitigation**: preserve most-recent unit; only compact completed + >14d units.

---
chunk_strategy: h1-h2-h3
description: "Adversarial review findings for shipment 130-S checkpoint top-level key disposition"
doc_type: closure
ingested_at: "2026-09-03T03:35:16Z"
schema_version: "1.0"
source: docs/closure/2026-08-24-130-s-adversarial-review.md
title: "Adversarial Review: 130-S Checkpoint Top-Level Key Disposition (PR #377, Cycle 11)"
---

**Date**: 2026-08-24
**Reviewers**: 3 independent models (Tier 1: gemini-3.7-flash, Tier 2: claude-sonnet-4.6, Tier 3: claude-sonnet-5)
**Review range**: `4fd065f5..bac4afd2` on `chore/stage-130-s`
**PR**: #377 (docs-only staging for 147-F / 130-S)

## Review Outcome

**READY_WITH_FOLLOWUPS**

No HIGH-confidence P0/P1 findings block push. Two MEDIUM-confidence P1 findings require explicit acknowledgment (fix or defer with rationale). Five LOW-confidence observations are preserved for human judgment.

---

## Consensus Findings

### HIGH Confidence (all 3 reviewers)

*None.*

### MEDIUM Confidence (2/3 reviewers) — Require Acknowledgment

#### F1 — Plan U7d body contradicts cycle-11-corrected task 147.025-T [P1]

| Field | Value |
|---|---|
| **Severity** | MAJOR |
| **Confidence** | MEDIUM (Reviewers B, C) |
| **File** | `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` |
| **Section** | Implementation Units → U7d (line ~630) |
| **Issue** | Plan U7d body states: `"ErrCheckpointCannotResolveAbandoned and ErrCheckpointCorrupt map to validation_failed through domainError today and would fall to that function's default: InternalError tail if rerouted wholesale."` This asserts `ErrCheckpointCannotResolveAbandoned` maps to `validation_failed` today, directly contradicting cycle-11 ROOT 4's correction and the corrected 147.025-T task body, both of which state it has **no explicit case** in `domainError` and falls to `default: InternalError`. The cycle-11 remediation appendix at the bottom of this plan correctly records the fix, but the canonical Implementation Units U7d description was never updated. |
| **Fix** | Change the U7d paragraph to: `"ErrCheckpointCorrupt maps to validation_failed through domainError today; ErrCheckpointCannotResolveAbandoned has no explicit case and falls to the default: InternalError tail. A wholesale swap would regress both."` |
| **Priority Score** | 6 (MEDIUM×3 + MAJOR×3 = 2×3 = 6) |
| **Action Class** | `gated_auto` — confirm before applying |

#### F2 — Plan U7d Expected Red lists case 4 as regression guard [P1]

| Field | Value |
|---|---|
| **Severity** | MAJOR |
| **Confidence** | MEDIUM (Reviewers B, C) |
| **File** | `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` |
| **Section** | Implementation Units → U7d → Expected red (line ~649) |
| **Issue** | Plan says `"cases 3 and 4 are declared regression guards"` — but cycle-11 ROOT 4 changed case 4 to a genuine red delta (`InternalError` → `validation_failed`). The corrected 147.025-T task body says `"case 4 fails"`. The plan's own cycle-11 appendix records this correction but the canonical description was not updated. |
| **Fix** | Change Expected Red to: `"cases 1 and 2 fail (routing and both remediation-verb assertions); case 4 fails (pre-implementation returns InternalError, post-implementation returns validation_failed); case 3 is a declared regression guard."` |
| **Priority Score** | 6 |
| **Action Class** | `gated_auto` |

### LOW Confidence (1/3 reviewers) — Human Judgment Required

#### F3 — 147.025-T Files list missing `internal/mcp/errors.go` [P1]

| Field | Value |
|---|---|
| **Severity** | CRITICAL |
| **Confidence** | LOW (Reviewer C only) |
| **File** | `.backlogit/queue/147.025-T.md` |
| **Section** | Body, "Files:" line |
| **Issue** | 147.025-T (U7d) body explicitly states: `"Adding that explicit mapping in domainError is a genuine red delta owned by this unit (U7d)"`. The `domainError` function lives in `internal/mcp/errors.go`, which is exclusively scoped to 147.013-T (U7). But U7d's Files list is only `internal/mcp/tools.go, internal/mcp/checkpoint_disposition_test.go`. No task's Files list covers the `errors.go` edit U7d's body claims to own. |
| **Fix** | Either add `internal/mcp/errors.go` to 147.025-T's Files list, or move the `ErrCheckpointCannotResolveAbandoned` mapping into 147.013-T/U7's acceptance criteria and have 147.025-T only assert it from the handler side. |
| **Priority Score** | 4 (LOW×1 + CRITICAL×4 = 4) |
| **Action Class** | `gated_auto` — unusual single-source CRITICAL |

#### F4 — Plan U9 missing "no administrative disposition" qualifier [P2]

| Field | Value |
|---|---|
| **Severity** | MAJOR |
| **Confidence** | LOW (Reviewer C only) |
| **File** | `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` |
| **Section** | Implementation Units → U9 (line ~854) |
| **Issue** | Plan U9's four-class table is headed `"For a status: 'active' target"` but does not include the `"with no administrative disposition"` qualifier that 147.017-T correctly has. The plan's own cycle-11 appendix records this correction was made to the task, but the canonical U9 plan section was not updated. |
| **Fix** | Add `"with no administrative disposition"` to the U9 table header, matching 147.017-T. |
| **Priority Score** | 3 |
| **Action Class** | `advisory` |

#### F5 — Plan U8b Tests(3) byte-identity claim not scoped [P2]

| Field | Value |
|---|---|
| **Severity** | MAJOR |
| **Confidence** | LOW (Reviewer C only) |
| **File** | `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` |
| **Section** | Implementation Units → U8b → Tests (3) (line ~786) |
| **Issue** | Plan says `"every fixture file is byte-identical after all three surfaces have been exercised"` without scoping to refused-mutation paths. 147.016-T's corrected acceptance criteria explicitly scope byte-identity to refused-mutation paths only and state the conforming-active row asserts rewrite/archive outcome. The plan's U8b detailed Expected Red section below partially addresses this via the fresh-copy mechanism, creating ambiguity about what "fixture file" means. |
| **Fix** | Qualify the Tests(3) bullet: `"...and that every canonical fixture file is byte-identical after all three surfaces have been exercised (mutating paths exercise copies; the canonical bytes are unchanged)"` — or scope explicitly to refused-mutation rows. |
| **Priority Score** | 3 |
| **Action Class** | `advisory` |

#### F6 — Stale cycle-9 shape "27/43/28" in checkpoint resume_hint [P3]

| Field | Value |
|---|---|
| **Severity** | MINOR |
| **Confidence** | LOW (Reviewer B only) |
| **File** | `.backlogit/checkpoints/checkpoint-20260824-191617.json` |
| **Section** | `resume_hint` field, inline cycle-9 narrative |
| **Issue** | An embedded cycle-9 sentence reads `"shape remains 27 tasks/43 edges/28 members"` — stale after cycle-10 reduced it to 26/42/27. The correct shape is stated later in the same field and elsewhere. |
| **Fix** | Qualify the inline as `"cycle-9 left shape unchanged at (then) 27/43/28"` or remove. |
| **Priority Score** | 2 |
| **Action Class** | `advisory` |

#### F7 — Plan cycle-11 ROOT 2 claims `archived_reason` but archive file has none [P3]

| Field | Value |
|---|---|
| **Severity** | MINOR |
| **Confidence** | LOW (Reviewer B only) |
| **File** | `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` |
| **Section** | PR #377 Copilot review remediation, cycle 11, ROOT 2 |
| **Issue** | ROOT 2's remediation description lists `"archived_from, archived_status, archived_reason, status: archived"` but the actual `.backlogit/archive/147.010-T.md` has no `archived_reason` field. |
| **Fix** | Remove `archived_reason` from the ROOT 2 text. |
| **Priority Score** | 2 |
| **Action Class** | `advisory` |

---

## Remediation Queue

Ordered by `confidence_weight × severity_weight`, then by file path:

| # | Finding | Score | Action Class | File |
|---|---|---|---|---|
| 1 | F1 — Plan U7d stale ErrCheckpointCannotResolveAbandoned claim | 6 | `gated_auto` | plan |
| 2 | F2 — Plan U7d case 4 mislabeled as regression guard | 6 | `gated_auto` | plan |
| 3 | F3 — 147.025-T Files list missing errors.go | 4 | `gated_auto` | 147.025-T |
| 4 | F4 — Plan U9 missing disposition qualifier | 3 | `advisory` | plan |
| 5 | F5 — Plan U8b byte-identity not scoped | 3 | `advisory` | plan |
| 6 | F6 — Stale shape in checkpoint resume_hint | 2 | `advisory` | checkpoint |
| 7 | F7 — archived_reason claim inaccurate | 2 | `advisory` | plan |

---

## Coverage Statement: Nine Intended Fixes

| # | Fix Claim | Verdict | Evidence |
|---|---|---|---|
| 1 | U10b mirror uses `<scratch>/mirror/.backlogit/checkpoints/` and `--cwd` with bare filenames | **VERIFIED** | 147.026-T body and acceptance criteria explicitly state the mirror path and `--cwd docs/scratch/checkpoint-verification/mirror` with bare filename arguments. Plan U10b section matches. |
| 2 | 147.010-T archived via canonical lifecycle | **VERIFIED** | Archive file has `archived_status: done`, `archived_from: .backlogit/queue/147.010-T.md`, `status: archived`. File is absent from `.backlogit/queue/`, absent from 130-S items list (27 members, no 147.010-T). Cycle-10 and cycle-11 memories document the retirement. |
| 3 | U8b fresh fixture copy per mutating case; byte-identity only for refused mutations | **VERIFIED** | 147.016-T body and acceptance criteria explicitly state "each mutating test case gets a fresh copy" and "Byte-identity postcondition applies only to refused-mutation paths". Plan U8b Expected Red section matches (fresh copies described). Minor drift in plan's Tests(3) summary (F5). |
| 4 | ErrCheckpointCannotResolveAbandoned genuine red delta in U7d | **VERIFIED WITH CAVEAT** | 147.025-T correctly states case 4 is genuine red (InternalError → validation_failed). Plan U7d body and Expected Red are stale (F1, F2). 147.025-T Files list gap (F3). Task/plan cycle-11 appendix records are correct. |
| 5 | U9 gen-docs no-diff for output-only changes | **VERIFIED** | 147.017-T body explicitly states "CLI Reference Drift check verifies no-diff when only output projection changed" and acceptance criteria say "committed only when actual Cobra metadata changes cause a diff". |
| 6 | U8b unit IDs correct | **VERIFIED** | 147.016-T Expected Red section correctly references: validity gate `147.006-T / U3`, conformance gates `147.007-T / U3b` and `147.008-T / U4`. Cross-checked against those tasks' frontmatter and titles. |
| 7 | I3 ownership → U5/147.009-T | **VERIFIED** | 147.017-T says `"I3 scoping, pinned by U5 / 147.009-T"`. 147.009-T contains the absorbed U5b regression guards. Plan I3 discussion and U5b retirement notice are consistent. No stale U5b ownership reference in current-state text (U5b appears only in historical/superseded sections). |
| 8 | Cycle-10 and cycle-11 memory files exist | **VERIFIED** | Both files exist under `docs/memory/2026-08-24/`. Checkpoint `memory_path` references both. Cycle-11 memory covers all 9 roots. Compaction threshold (11 files / ~313KB) below both triggers. No volatile head/CI claims masquerade as durable truth — checkpoint explicitly states "GitHub PR #377 is authoritative". |
| 9 | U9 four-class table scoped to active/no-administrative-disposition | **VERIFIED** | 147.017-T correctly scopes the table to "active documents with no administrative disposition" and adds a separate precedence section for `disposition: abandoned` → `ErrCheckpointCannotResolveAbandoned`. Plan U9 summary is slightly less precise (F4) but the task is correct. |

## Cross-Surface Consistency Check

| Surface | Status |
|---|---|
| Plan ↔ Tasks | Two stale plan passages (F1, F2) and one precision gap (F4); cycle-11 appendix records are correct; tasks are authoritative |
| Tasks ↔ Checkpoint | Consistent (shape 26/42/27, memory paths resolve, volatile claims properly qualified) |
| Tasks ↔ Memory | Consistent (cycle-10 and cycle-11 memories match task changes) |
| Shipment ↔ Queue | Consistent (27 items, 26 queued tasks + 147-F, no archived 147.010-T) |
| Archive ↔ Queue | Consistent (147.010-T in archive only, lock files present in both) |
| Topology: 26/42/27/147.001-T | Confirmed by prior validation; 130-S has 27 items (26 tasks + 1 feature) |
| Stale U5b references | None in current-state text; appears only in historical/superseded sections |
| Stale 27/43/28 shape | One inline in checkpoint resume_hint (F6); all current-state claims use 26/42/27 |
| P-004 all-guards exemption | Removed from Test-First posture section and Constitution Check II; no surviving instances |
| Shell/path contract | POSIX-safe throughout; Windows-first-workspace rationale consistently documented |
| Destructive-action approval | U10b teardown and U9b restore-abort both require Principle VII approval |
| Impossible/contradictory acceptance criteria | None found (U8b's apparent byte-identity contradiction resolved by fresh-copy mechanism) |

## Determination

**No HIGH-confidence P0/P1 findings exist.** Push is not blocked.

The two MEDIUM-confidence P1 findings (F1, F2) are plan-body staleness: the cycle-11 remediation appendix in the same plan file correctly records the corrections, and the authoritative task files (147.025-T) are correct. These are documentation drift within the plan, not contract contradictions that would cause implementation errors — an implementer reading the cycle-11 appendix would get the right information.

The one LOW-confidence CRITICAL (F3) is a Files-list gap on 147.025-T that should be addressed before implementation begins but does not affect the staging PR's merge-readiness.

**Recommendation**: Fix F1+F2+F3 in the next commit before push. F4–F7 are advisory.

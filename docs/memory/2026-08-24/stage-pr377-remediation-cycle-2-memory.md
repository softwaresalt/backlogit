---
chunk_strategy: h1-h2-h3
description: "Stage session memory for PR #377 Copilot review remediation cycle 2 — 15 unresolved threads plus 3 suppressed review-body findings reconciled across the 147-F / 130-S planning and backlog artifacts"
doc_type: memory
docline:
    date: 2026-08-24T00:00:00Z
    status: accepted
    tags:
        - session-memory
        - D3CE9E81
        - 147-F
        - 130-S
        - checkpoint
        - dark-mode
        - review-remediation
schema_version: "1.0"
source: docs/memory/2026-08-24/stage-pr377-remediation-cycle-2-memory.md
title: "Stage Session Memory — PR #377 Review Remediation Cycle 2"
---

# Stage Session Memory — PR #377 Review Remediation Cycle 2

**Date**: 2026-08-24
**Agent**: Stage
**Session scope**: PR #377 review remediation only, bounded to stash `D3CE9E81` / feature `147-F` / shipment `130-S`
**Dark mode**: `DARK_MODE_ACTIVE` under P-017, `admin_fallback_pre_authorized=false`
**Branch / worktree**: `chore/stage-130-s` in the dedicated Stage planning worktree
**Reviewed HEAD**: `74065273f2d5aeee5fdc86391c15fb0f519ab1f1` (local == remote, CI green)

## Summary

Second Copilot review remediation cycle on staging PR #377. The fresh review
`PRR_kwDORzozKM8AAAABKr0WsA` covers the current head and raised 15 unresolved
threads plus 3 suppressed review-body findings. All 18 were confirmed against
live source and frozen artifacts, all 18 were **valid**, and all 18 were fixed.
Nothing was declined, deferred, or partially accepted.

No task was added, no dependency edge was added, and shipment membership did not
change: the backlog shape stays 22 width-isolated tasks under `147-F` with 34
`blocks` edges and a ready set of exactly `147.001-T`. Every fix landed in
planning, backlog, checkpoint, or memory artifacts — Stage wrote no Go source
and ran no build, test, or lint command.

## Data-safety fixes (the three that mattered)

### 1. `context` key collision in the hand-repair path

The U9b repair procedure told operators to move every unmodeled top-level key
"under `context`". `CheckpointContext.UnmarshalJSON`
(`internal/events/checkpoint_schema.go:196-220`) decodes the same bytes twice and
**skips** any `context` member whose name is `strings.EqualFold`-equal to a
modeled field (`shipment_id`, `feature_id`, `task_ids`, `branch`) when populating
`Extra`. A flattened top-level `shipment_id` landing beside an existing
`context.shipment_id` therefore destroys one of the two values on the next
re-marshal — the exact loss class this feature exists to close.

**Fix**: every moved key nests under the single unmodeled container
`context.legacy_top_level`, preserving original key names verbatim. The container
name is unmodeled, so it round-trips through `Extra` intact, and nesting *every*
key makes the rule uniform instead of requiring per-key fold comparisons. If the
container already exists, merge into it and refuse to overwrite an existing
member.

### 2. Archive filename collision and quarantined-evidence deletion

The post-quarantine restore said "copy the archived bytes back from
`archive/checkpoints/`". That leaves one filename live in **both**
`.backlogit/checkpoints/` and `archive/checkpoints/`. The next
`CleanupCheckpoints` sweep archives the active copy over the quarantined one, and
on Windows it calls `os.Remove(dst)` before the rename
(`internal/events/checkpoint_lifecycle.go:238-242`) — destroying the quarantined
evidence outright.

**Fix**: restore now reconciles the archive first. Rename the archived bytes
**and** their `.disposition.json` sidecar aside as a pair to
`<filename>.quarantined-<disposition_at>` (compact UTC basic format
`20060102T150405Z`, legal on Windows). The evidence name no longer ends in
`.json`, so it can never be a `CleanupCheckpoints` destination nor match the
`checkpoint-*.json` glob. Then **copy** — never move — the preserved bytes into
the now-free active filename, stopping if that path already exists. The sidecar
stays in the archive because it describes the quarantine event, not the restored
working copy; carrying it into the active directory would put a
`checkpoint-*.json.disposition.json` file inside the checkpoint glob. The renamed
evidence pair is retained, not deleted.

### 3. Impossible cross-package fallback seam

U2f's fallback named an **unexported** `rewriteCheckpointFile` in
`internal/events`, but the seam has two consumers in two packages:
`ResolveCheckpoint` in `internal/events` (U3b) and `AbandonCheckpoint` in
`internal/core` (U4). Go cannot reference an unexported identifier across that
boundary, so the helper would have gated exactly one of the two rewrite paths —
the hole I1 exists to close.

**Fix**: the seam is the **exported**
`events.RewriteCheckpointFile(path string, cp *CheckpointV1, original []byte) error`,
the affected-file list now names `internal/core/checkpoint_disposition.go`, and
under the fallback the write-site enumeration allow-list must cover
`internal/core` as well.

## Acceptance-contract fixes

| Finding | Fix |
|---|---|
| `get_checkpoint` promised `conforming:false` for *every* document with unmodeled keys | Scoped to schema-valid documents: `GetCheckpoint` returns `ErrCheckpointInvalid` before any conformance verdict exists |
| U8 demanded named offending keys for a legacy file | Split into a schema-invalid refusal (validity gate fires first, no key list exists) and a valid-but-non-conforming refusal (named keys); still three scenarios, so the 2-Hour Rule holds |
| U9's three-class table was not total | Now four classes; `parses but schema-invalid` is broken out because that is the exact shape of the nine legacy files |
| I3 table row 4 said `ErrCheckpointNotActive` | Now `ErrCheckpointUseQuarantine` / `ErrCheckpointNonConforming`, matching its own footnote — `ErrCheckpointNotActive` is reachable only by a resolved document that parses, validates, and conforms |
| Plan U7b drifted from `147.014-T` | Plan reconciled to the task's delta table, declared the single source of truth |
| Plan U8b drifted from `147.016-T` | Plan widened to three surfaces (CLI + MCP + `events`), `Depends on` gained U6c, posture corrected to declared regression guard |
| Deliberation claimed unqualified totality | Scoped to `status: "active"`, citing I3 and the U5b-pinned double refusal |

## Red-phase honesty

U2 case 3, U2b cases 2 and 3, and U3b cases 2 and 3 all expect `nil` or assert
already-shipped behaviour, so none of them can fail in a red phase. They are now
**declared regression guards** in both the plan and the harvested tasks, and the
plan's test-first posture section gained a "Declared regression guards" paragraph
so the distinction is stated once rather than re-litigated per unit.

## Stale handoff checkpoint

`.backlogit/checkpoints/checkpoint-20260824-191617.json` still recorded
`branch: main`, `commit: 185fe23b`, `commit_pushed: false`. Regenerated against
the reviewed head with a **branch-relative** pointer rather than a pinned SHA
(`branch: chore/stage-130-s`, `pr`, `push_state`, `resume_ref`), because any SHA
written during a remediation cycle goes stale the moment the remediation commit
lands. `.backlogit/memories.json` was updated in the same pass so the two agree.

## Files changed

* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
* `docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md`
* `.backlogit/queue/147.002-T.md`, `147.003-T.md`, `147.007-T.md`, `147.014-T.md`,
  `147.015-T.md`, `147.017-T.md`, `147.018-T.md`, `147.019-T.md`, `147.021-T.md`
* `.backlogit/checkpoints/checkpoint-20260824-191617.json`
* `.backlogit/memories.json`
* `docs/memory/2026-08-24/stage-pr377-remediation-cycle-2-memory.md` (this file)

## Not done by Stage

Push, review-thread replies, thread resolution, review re-request, shipment
claim, and merge are all forbidden by the Stage Role Boundary (PR row: Allowed
"—", Forbidden "Create, push, or merge pull requests"), and the fail-closed rule
in `.github/instructions/role-enforcement.instructions.md` treats the unlisted
PR-mutating operations the same way. Ship or the operator owns all of them. The
exact reply bodies for each comment ID were produced in the session handoff.

## Next steps

1. Ship or the operator pushes `chore/stage-130-s`.
2. Post the cycle-2 replies against the 15 thread IDs, resolve them, and note the
   3 suppressed findings in the PR conversation (they carry no thread).
3. Re-request Copilot review and re-run the §1.9 readiness gate against the new
   head before presenting the PR for the P-014 merge approval.
4. Refresh the SQLite index before trusting query output — these edits were made
   in a dedicated planning worktree and were not synced.

---
doc_type: memory
session: stage-d3ce9e81-2026-08-24
cycle: 11
date: 2026-08-24
author: stage-agent
---

# Stage PR #377 Remediation Cycle 11 Memory

Cycle 11 was an operator-authorized extension resolving 9 Copilot review roots.

## Roots Addressed

### ROOT 1 — U10b mirror workspace (147.026-T)

The recovery sweep described the mirror at `docs/scratch/checkpoint-verification/mirror/` but CLI operations (`checkpoint resolve`, `checkpoint abandon`, `checkpoint list`) use `--cwd` to locate `.backlogit/checkpoints/`. The mirror must be a proper workspace.

**Fix:** Mirror copy target changed to `docs/scratch/checkpoint-verification/mirror/.backlogit/checkpoints/`; all sweep CLI invocations updated to use `--cwd docs/scratch/checkpoint-verification/mirror` with bare filename arguments. Acceptance criteria updated. Plan U10b section updated.

### ROOT 2 — Canonical archive lifecycle for 147.010-T (147.010-T)

The archive file was placed manually without going through the backlogit `archive_item` operation, so it lacked `archived_status: done` and proper lifecycle history (JSONL log events).

**Fix:** 147.010-T restored to queue in its pre-archive shape, hand-constructed archive copy removed. Status transitioned queued → active → done via `backlogit update --status`, then archived via `backlogit archive 147.010-T --cwd`. The canonical `core.ArchiveItem` operation produced `archived_status: done`, `archived_from`, `status: archived`, and three JSONL log events (`pre_task_completion_gate_passed` ×2, `archived`) in `.backlogit/logs/147.010-T.jsonl`. Hook queue events emitted to `.backlogit/hooks_queue.jsonl`.

### ROOT 3 — U8b fixture isolation (147.016-T)

The conforming-active row accepts `abandon`, which mutates the checkpoint. The task stated every fixture is byte-identical after all surfaces are exercised, which cannot hold when an accepted mutation rewrites the file.

**Fix:** Task body clarified: one canonical fixture per state; each mutating test case gets a fresh copy; byte-identity postcondition applies only to refused mutation paths; the conforming-active row's `abandon` acceptance asserts the intended rewrite/archive outcome, not byte identity. Acceptance criteria updated.

### ROOT 4 — ErrCheckpointCannotResolveAbandoned mapping (147.025-T)

The task body incorrectly stated that `ErrCheckpointCannotResolveAbandoned` already mapped to `validation_failed` through `domainError`. In fact, it has no explicit case and falls to `default: InternalError`. Case 4 therefore asserted a pre-existing behavior that did not exist.

**Fix:** Task body corrected: `ErrCheckpointCannotResolveAbandoned` falls to `default: InternalError` before U7d adds the explicit mapping. Case 4 is now a genuine red delta (pre-impl: `InternalError`; post-impl: `validation_failed` with exact message `"resolve checkpoint: backlogit: checkpoint has been administratively abandoned; resolve is refused"`). Expected red updated to include case 4 as red.

### ROOT 5 — gen-docs/output projection mismatch (147.017-T)

`gen-docs` renders Cobra help metadata, not runtime JSON projection. U8/U8c are output-only changes (runtime projection), not Cobra help-text changes, so `gen-docs` produces no diff for them.

**Fix:** Task body and acceptance criteria updated: CLI Reference Drift check verifies no-diff when only output projection changed; committed `docs/cli-reference/` files are updated only when actual Cobra metadata changes cause a diff.

### ROOT 6 — Reversed U8b unit IDs (147.016-T)

The red-state explanation named `147.008-T / U3` for the validity gate (should be `147.006-T / U3`) and `147.008-T / U3b` / `147.007-T / U4` for conformance gates (should be `147.007-T / U3b` / `147.008-T / U4`).

**Fix:** Corrected all three reversed references in the expected red section of 147.016-T.

### ROOT 7 — I3 owner points to retired U5b (147.017-T)

The I3 totality scoping in 147.017-T was attributed to `U5b` which is now retired.

**Fix:** `I3 scoping, pinned by U5b` changed to `I3 scoping, pinned by U5 / 147.009-T`. Table header updated to include "with no administrative disposition" qualifier.

### ROOT 8 — Dangling cycle-10 memory reference

The checkpoint referenced `docs/memory/2026-08-24/stage-pr377-remediation-cycle-10-memory.md` which did not exist.

**Fix:** Created cycle-10 memory file. Created this cycle-11 memory file. Checkpoint `memory_path` will be updated. Compaction threshold check: 11 total files in `docs/memory/2026-08-24/`, total memory directory ~313KB — below both thresholds (>40 files or >500KB).

### ROOT 9 — U9 matrix not total over administrative disposition (147.017-T)

`ResolveCheckpoint` checks disposition FIRST — a valid conforming active document with `disposition: abandoned` returns `ErrCheckpointCannotResolveAbandoned` before any validity/conformance gate.

**Fix:** Four-class table header explicitly scoped to "active documents with no administrative disposition". Added a separate precedence section documenting that `disposition: abandoned` → `ErrCheckpointCannotResolveAbandoned` fires before the classification table, and noting this is a regression guard for the implemented check order. Acceptance criteria updated.

## Backlog Shape

- Unchanged: 26 tasks / 42 edges / 27 shipment members
- No task added, no edge changed, no shipment member changed (ROOT 2 corrects the archive lifecycle metadata only; the task was already retired in cycle 10)

## Files Modified

- `.backlogit/queue/147.016-T.md` — ROOTs 3, 6
- `.backlogit/queue/147.017-T.md` — ROOTs 5, 7, 9
- `.backlogit/queue/147.025-T.md` — ROOT 4
- `.backlogit/queue/147.026-T.md` — ROOT 1
- `.backlogit/archive/147.010-T.md` — ROOT 2 (canonical archive lifecycle)
- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — ROOT 1 plan update, cycle-11 section
- `.backlogit/checkpoints/checkpoint-20260824-191617.json` — memory_path updated for cycles 10/11
- `.backlogit/memories.json` — entry updated with cycle-11 record
- `docs/memory/2026-08-24/stage-pr377-remediation-cycle-10-memory.md` — created (ROOT 8)
- `docs/memory/2026-08-24/stage-pr377-remediation-cycle-11-memory.md` — created (ROOT 8, this file)

## Compaction Threshold Check

- `docs/memory/` directory: prior archive-only compaction reduced 41 to 27 files at an earlier point; measured footprint at current state is 36 files / 316.2 KiB, still below mandatory triggers (>40 files or >500 KB); no new compaction required.

## Adversarial Review Follow-Up (Cycle 11)

A three-model adversarial review returned READY_WITH_FOLLOWUPS with zero HIGH-confidence blockers. Seven synchronization findings (2 MEDIUM P1, 2 LOW P1, 2 LOW P2, 1 LOW P3) were resolved:

1. Plan U7d body: corrected false `validation_failed` claim for `ErrCheckpointCannotResolveAbandoned` to `InternalError` fallthrough; synchronized with 147.025-T
2. Plan U7d Expected Red: case 4 reclassified from regression guard to genuine RED assertion
3. 147.025-T and plan U7d Files list: added `internal/mcp/errors.go`
4. Plan U9 four-class matrix: added `with no administrative disposition` qualifier matching 147.017-T
5. Plan U8b Tests(3): scoped byte-identity to refused mutation paths only
6. Checkpoint resume_hint: cycle-9 shape `27/43/28` marked as historical
7. Plan cycle-11 ROOT 2: removed unsupported `archived_reason` claim

Backlog shape unchanged: 26 tasks / 42 edges / 27 shipment members.

## Next Steps

- Ship to receive handoff at PR #377 head
- All 9 roots resolved; commit locally and hand off for push/review/merge per normal Stage Role Boundary

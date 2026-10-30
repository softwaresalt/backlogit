# Dark mode 141-S to 145-S: halted at preflight

- Time: 2026-10-16T18:15Z (UTC)
- Events: `DARK_MODE_START`, `DARK_MODE_SCOPE`, `DARK_MODE_HALTED`
- Scope (ordered): 141-S, 142-S, 143-S, 144-S, 145-S. Cursor: last completed none, next 141-S.
- Pre-authorized: merge approval and admin fallback (operator instruction). Neither is used because the run halted.

## Halt reason: STAGING_GATE_FAIL (unclassified dirty path)

`.backlogit/hooks_queue.jsonl` is modified on `main` (14 appended hook events, seq 3791-3794 onward). It is outside the Step 1.5 continuity allowlist (`.backlogit/stash.jsonl`, `docs/memory/**`, `start.ps1`, `.gitignore`), so the Orchestrator may not commit it. Ship's branch gate (Step 0.5 item 3a) also needs a clean `main`.

## Other preflight findings

- One `active` checkpoint exists: `checkpoint-20261008-073934.json` (ship, 153-S, phase post-merge-closure). 153-S is already shipped and archived. It was not selected or resolved because that needs explicit operator selection.
- `main` equals `origin/main` (fc54f61d). No active shipments. One worktree.
- DAG: 141-S has all predecessors archived. 142-S through 145-S are chained by `blocks` edges. The pipeline-topology pre_claim gate was not run because of the halt.

## Resolution (2026-10-16T18:35Z)

The operator ruled that `.backlogit/hooks_queue.jsonl` belongs on the continuity allowlist. It was carried forward through the Step 1.5 staging PR for this run. The permanent allowlist edit in the Orchestrator template is still to do (see Next steps). The 153-S checkpoint is left untouched, and the new-scope dark-mode command was treated as selecting no checkpoint to resume.

## Next steps for the operator

1. Choose one of:
   - Commit `.backlogit/hooks_queue.jsonl` as a carry-forward (prior precedent: commit e9d18805).
   - Add `.backlogit/hooks_queue.jsonl` to the Orchestrator allowlist.
2. Resolve or abandon the stale 153-S checkpoint.
3. Re-run "Run pipeline in dark factory mode" for the 141-S to 145-S scope.

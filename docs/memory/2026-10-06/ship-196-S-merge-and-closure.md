# Ship 196-S merge and post-merge closure

## Context

- Shipment `196-S` (feature `196-F`). P-017 dark mode, scope `[196-S]`, operator AFK.
- `merge_approval_pre_authorized: true`, `admin_fallback_pre_authorized: false`.
- ROUTING_DEGRADED: the requested route was gpt-6-luna / openai / xhigh; the runtime was Claude.
- Resumed from `checkpoint-20261006-073956.json` under the Erratum E1 approval (`d9e00a6d`).
  The checkpoint was resolved after a confirmed resume.

## Build and PR

- All nine tasks are done. 196.001-T and 196.003-T used the E1.4 per-task
  clean-tree/fresh-baseline rule, with no git stash used. Each task's completion evidence cites
  E1.2.
- Wave gates 2–5 passed, and so did the final unfiltered `go test -timeout=30m ./...`.
- Local adversarial review: READY_WITH_FOLLOWUPS, P0/P1 = 0.
- PR #478 went through Copilot passes 1–5:
  - Pass 1 out-of-scope comments were captured as deferred entry `8F1CF1E1`.
  - The non-required `pipeline-topology (ambient)` CI failure, caused by a PyPI autoharness
    version skew, was captured as `F88FE051`.
  - Fixes landed in `e7e7da02`, `c2d309f1`, `cea1b183`, and `2a123282`.
- The copilot-review gate was SATISFIED, and required CI was green.
- Merged with a merge commit (P-009), without `--admin`, as
  `651b066b749d216b88e9b5d406a586f26a69cead`.
- POST_MERGE_SYNC_OK: local `main` == `origin/main`.

## Closure

- Branch: `post-merge/196-f-served-root-handoff-explicit-feature-reconcile`.
- The topology lifecycle gate passed. Shipment-reconcile pre-mode returned PROCEED and the
  lock was held through post-mode.
- `backlogit_ship_shipment` returned an MCP client timeout (-32001), but the governed operation
  completed server-side about 10 minutes later. I did not retry and did not restore the archive.
  I verified completion via the shipment archived event and the hooks_queue `ship_shipment` hook.
  The learning is in `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md`.
- Archive commit `4130b5e2`. P-007 is clean, and post-mode reconcile passed.
- Stash `227A2930` and `B83081F5` were already archived at harvest. `359D8F32` is left open.
- Compound-refresh: six entries kept, one new learning.
- compact-context, target all: `compaction_status: degraded`. Originals were not moved, because
  path references to them remain. `359D8F32` owns the broader memory review.
- Closure artifact: `docs/closure/196-S-served-root-handoff-post-merge-closure.md`, with
  `closure_status: READY_WITH_CONDITIONS`.
- `.git/info/exclude`:
  - Removed the 196-S temporary block.
  - Kept a standalone never-commit line for `checkpoint-20261005-052350.json`
    (operator decision E). Flagged for operator review.
- `logs\optionA\` was deleted, as pre-approved.
- CLOSURE_INDEX_SYNC_OK: 1908 items indexed.

## Residual risks

- `1EF5BDC6`, `33C4B816`, the E1.2 process deviation, and the #477 P-001/P-016 deviation.
- A3.6 items: `CDBCB258`, `F6F3AA0E`, `147BD825`, `3B25D37F`, and the R10 interim limitation.
- `F88FE051`.

## Next steps

- Closure PR: run local review, open the PR, run the Copilot loop, merge with a merge commit, and sync main.
- Then emit DARK_MODE_COMPLETE.

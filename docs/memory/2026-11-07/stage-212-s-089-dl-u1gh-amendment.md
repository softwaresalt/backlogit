---
title: "Stage prerequisite & P-002.6 red-deliverable amendment for 212-S (089-DL / U1g 211.013-T, U1h 211.014-T, U2i 211.015-T, M12 -> M15)"
description: "Bounded C6 deliberation 089-DL, M14->M15 manifest amendment adding P-002.6 persistent red-deliverable U2i (211.015-T) to mechanically implement the operator-approved 212-S-only Option 1 gate-cadence exception across Waves 3-8."
doc_type: "memory"
schema_version: "1.0"
source: "docs/memory/2026-11-07/stage-212-s-089-dl-u1gh-amendment.md"
chunk_strategy: "h1-h2-h3"
date: "2026-11-07"
shipment_id: "212-S"
feature_id: "211-F"
deliberation_ids:
  - "089-DL"
task_ids:
  - "211.013-T"
  - "211.014-T"
  - "211.015-T"
phase: "stage-amendment-complete-ship-custody-released"
status: "active"
docline:
  shipment_id: "212-S"
  feature_id: "211-F"
  phase: "stage-amendment-complete-ship-custody-released"
  status: "active"
---

## Executive Summary and Custody Handoff (`M1`)

After Ship completed `211.013-T` (`U1g`) and `211.014-T` (`U1h`) at `HEAD` `b40a2a9fb9decbc9b808aaeaf5443ee2277ef475` (`9/14` tasks `done`), Ship paused at Step 4.6 before `U3` (`211.003-T`) admission (`docs/memory/2026-11-08/212-S-M14-forward-20261108T012035Z.md`) because `open_red_deliverables_k` was empty, and under P-002.6 (`_ship.agent.md` Step 4.6 item 3) an empty `open_red_deliverables_k` requires an immediate unfiltered full-suite run rather than a deferral.

Stage completed the narrow same-contract P-002.6 compliance amendment (`M14 -> M15`, `S15 -> S16`) by adding **`211.015-T` (`U2i`: *Scaffold feature-root multi-session MCP runtime and CLI reference RED acceptance harness across U3-U7*, `red_deliverable: true`)** under Feature `211-F` in Shipment `212-S`:

* **Why `M14` without a `red_deliverable: true` task member in `M` could not defer Step 4.6 under P-002.6**:
  1. `_ship.agent.md` Step 3 item 2 defines `M = { id in S : artifact_type(id) = task }`, excluding feature `211-F` (`excluded_non_task_members = [(211-F, feature)]`), and Step 3 item 3 parses `<!-- BEGIN:red-deliverable-contract -->` only from task members in `M`.
  2. `_ship.agent.md` Step 4.0 item 9 computes `open_red_deliverables_k` strictly from **completed (`status: done`) `red_deliverable: true` task members of `M`** at least one of whose declared `green_maker_tasks` in `M` is not yet `done`.
  3. All `9` completed tasks in `M14` (`211.001-T`, `211.002-T`, `211.008-T`..`211.014-T`) completed GREEN (marking any of them `red_deliverable: true` would halt Step 4.6 with `WAVE_RED_DELIVERABLE_EARLY_GREEN`), while the `5` unstarted tasks (`211.003-T`..`211.007-T`) are not `done` yet and cannot use `build-feature` Step 0.5 (`RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED` / `harness-exempt` mutual exclusion).
* **How `211.015-T` (`U2i`) implements the approved Option 1 final-only cadence under P-002.6**:
  * `211.015-T` (`U2i`, `status: queued`, `priority: high`, `size: XS`, `complexity: low`, `dependencies: [211.001-T, 211.002-T, 211.013-T, 211.014-T]`) is the sole member of **Wave 3 (`ready_3 = {211.015-T}`)** and blocks `211.003-T` (`U3`, Wave 4).
  * Its `red-deliverable-contract` declares `red_deliverable: true`, `red_selector_command: go test -count=1 -run '^TestU2i_' ./internal/cli`, `green_maker_tasks: 211.003-T, 211.004-T, 211.005-T, 211.006-T, 211.007-T`, and `green_maker_closes_wave: 8`.
  * In Wave 3, `harness-architect` scaffolds `internal/cli/mcp_feature211_acceptance_red_test.go` (`TestU2i_Feature211MultiSessionDaemonShimAndDocsAcceptance`, asserting all five `U3`–`U7` surfaces using compilable `NewRootCommand()`, `go/ast`, and `os.Stat`/`os.ReadFile`), `build-feature` Step 0.5 validates compile PASS + named assertion RED + empty delta (`RED_DELIVERABLE_ACCEPTED`), and Ship completes `211.015-T` RED at Step 4.5, registering `^TestU2i_` in `open_red_deliverables_k`.
  * Across Waves 3, 4 (`U3`), 5 (`U4`), 6 (`U5`), and 7 (`U6`), `open_red_deliverables_k = [211.015-T]` is non-empty and `^TestU2i_` is re-confirmed RED at each Step 4.6 gate, so Step 4.6 item 3 records `FULL_SUITE_DEFERRED: wave {k}` (`k = 3..7`).
  * At Wave 8 (`U7` `211.007-T`), when `211.007-T` reaches `done`, all 5 green-makers are `done`, `211.015-T` moves into `newly_closed_8` (confirmed GREEN at Step 4.6 item 2 bullet 5), `open_red_deliverables_8` becomes empty, and Step 4.6 item 3 runs the **ONE unfiltered 50-package convergence suite (`go test -count=1 -p=1 -timeout=30m -json ./...`, `2,700s` outer / `1,800s` package budgets unchanged)** before PR creation.

## Final Stage-Authored Artifact SHA-256 Manifest (M15)

| Path | SHA-256 Digest |
|---|---|
| `.backlogit/archive/089-DL.md` | `97a8f6202e3a943b70c5da1be2f145ed1dcf4e630bf8507b351d9fb27a89caa8` |
| `.backlogit/queue/211.015-T.md` | `6ff9c5d2e9e7f1144adcc730f3c4e633093d01ff9eabd89b3f4b98e56ee8d134` |
| `.backlogit/queue/211.003-T.md` | `7a29fb09d82a5b756c08ac8ab1ee9f1e92a60b46924c06a28d4f1d6877cddf31` |
| `.backlogit/queue/211-F.md` | `6e17e7c0e8fcd6eab4b073c3b34e6478fef12fb91d2b672ccc6b1b51d453f401` |
| `.backlogit/queue/212-S.md` | `bf2216467d437fed92affb3eb2b58eecbfd9a63df32169305501a58d1f06cb68` |
| `.backlogit/stash.jsonl` | `59e3b07cb6128a36a6fc404b5382be3bf161fc5a0d539d992bb3de7f1e30acec` |
| `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` | `f5f1edb4e73fa17e3453431a673750ab45fe70ba9bed1d4b4969c2e868952880` |
| `docs/decisions/2026-11-07-212-s-delivery-blockers-rca.md` | `3a923a83cdf3ad67ffb9a391204dccd3e2a1852aee269b36f39cb441fc775257` |
| `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` | `2a4b8b16301afbb80451ae60cac8cae7c4c10f6d30047ba4cf35f08795fce3f5` |
| `docs/compound/test-failures/windows-host-hibernation-during-supervised-test-runs-2026-11-07.md` | `97b4be2d7f2dddd8216cee1aef84f8636d49a289eab97fac33d187e9d0b8f750` |
| `docs/memory/2026-11-07/212-S-rca-handoff.md` | `ff208e7ca391f803691a9d7e4d284d96cfd46a77fa50b614c379e614d9f9691b` |

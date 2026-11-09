---
title: "Stage Option 5 / P-002.1 Wave 5 declaration-vs-behavior split amendment for 212-S (089-DL / U3a-U5a 211.017-T..211.019-T, M16 -> M19)"
description: "Bounded C6 / P-002.1 amendment adding Wave 5 source-shape declaration prerequisite tasks U3a (211.017-T), U4a (211.018-T), and U5a (211.019-T) under 211-F / 212-S (M16 -> M19, S17 -> S20), shifting behavioral implementation tasks U3-U7 to Waves 6-10 and updating green_maker_closes_wave: 10 on 211.015-T (U2i)."
doc_type: "memory"
schema_version: "1.0"
source: "docs/memory/2026-11-08/stage-212-s-089-dl-u3a-u5a-m19-amendment.md"
chunk_strategy: "h1-h2-h3"
date: "2026-11-08"
shipment_id: "212-S"
feature_id: "211-F"
deliberation_ids:
  - "089-DL"
task_ids:
  - "211.017-T"
  - "211.018-T"
  - "211.019-T"
  - "211.003-T"
  - "211.004-T"
  - "211.005-T"
  - "211.006-T"
  - "211.007-T"
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

Under the operator's active **Option 5** authorization, Stage completed the bounded `M16 -> M19` (`S17 -> S20`) amendment for Feature `211-F` / Shipment `212-S` at `HEAD` `905d0a43` (following `211.016-T` `fd038b8feba15e43d7f02ed2742a0e91479b16b3` and `docs/memory/2026-11-08/212-S-longpaths-wave34-convergence.md`), resolving the Wave 5 `harness-architect` structural halt (`P-002.1` cycle-31 / `.github/skills/harness-architect/SKILL.md` Step 1 item 7 and Step 4 items 3, 5, 6):

* **No Stage git commit or source/skill/script/checkpoint mutation (`NOFEATURECOMMIT`)**: Stage authored only planning, backlog, and memory artifacts; Stage did not run `git commit`, `git add`, or `git push`, and did not modify any Go, PowerShell, skill, or checkpoint file. `checkpoint-20261108-230851.json` (`07eba14fb1505487695df61a991c333958c7b70312b696a67f6c8634780a190e`), `checkpoint-20261106-210317.json` (`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`), and `.backlogit/logs/211.003-T.jsonl`..`211.007-T.jsonl` are byte-untouched.
* **Three Disjoint Wave 5 Source-Shape Declaration Tasks (`ready_5 = {211.017-T, 211.018-T, 211.019-T}`)**:
  1. **`211.017-T` (`U3a`)**: *Declare multi-session ClientSession stream dispatcher types and methods with AST shape harness (U3a)* (`internal/mcp/multi_session_shape_test.go`, `internal/mcp/session_mux.go`; `go test -timeout=30m -count=1 -run '^TestU3a_' ./internal/mcp`).
  2. **`211.018-T` (`U4a`)**: *Declare cross-platform local IPC listener, dialer, and process liveness signatures with AST shape harness (U4a)* (`internal/cli/mcp_ipc_shape_test.go`, `internal/cli/mcp_ipc_windows.go`, `internal/cli/mcp_ipc_unix.go`; `go test -timeout=30m -count=1 -run '^TestU4a_' ./internal/cli`).
  3. **`211.019-T` (`U5a`)**: *Declare background MCP daemon, attachment verification, prewarm, and session shim signatures and metadata fields with AST shape harness (U5a)* (`internal/cli/mcp_daemon_shape_test.go`, `internal/cli/mcp_daemon.go` — **without** touching `internal/cli/root.go` so `TestRegistryParity_*` stays green and `^TestU2i_` stays assertion RED through Wave 9 until `U7` completes in Wave 10; `go test -timeout=30m -count=1 -run '^TestU5a_' ./internal/cli`).
* **Downstream Behavioral Waves (`Waves 6–10`) and `211.015-T` (`U2i`) `green_maker_closes_wave: 10`**:
  * `211.003-T` (`U3`, Wave 6) depends on `211.017-T` (`U3a`); `harness-architect` scaffolds only `internal/mcp/session_mux_test.go` with zero production stub edits.
  * `211.004-T` (`U4`, Wave 7) depends on `211.003-T` (`U3`) and `211.018-T` (`U4a`); `harness-architect` scaffolds only `internal/cli/mcp_ipc_test.go` with zero production stub edits.
  * `211.005-T` (`U5`, Wave 8) depends on `211.004-T` (`U4`), `211.008-T` (`U1b`), and `211.019-T` (`U5a`); `harness-architect` scaffolds only `internal/cli/mcp_daemon_multisession_test.go` with zero production stub edits.
  * `211.006-T` (`U6`, Wave 9) depends on `211.005-T` (`U5`) and `211.019-T` (`U5a`); `harness-architect` scaffolds only `internal/cli/mcp_session_shim_test.go` with zero production stub edits.
  * `211.007-T` (`U7`, Wave 10, `harness-exempt: docs-only`) depends on `211.006-T` (`U6`), closing `211.015-T` (`U2i`, `green_maker_closes_wave: 10`) GREEN at Wave 10 and discharging the single unfiltered 50-package convergence suite before PR creation.

## Final Stage-Authored Artifact SHA-256 Manifest (`M19` / `S20`)

| Path | SHA-256 Digest |
|---|---|
| `.backlogit/archive/089-DL.md` | `c0bbc59b1a6220d53bac90c7093666291cc2e3df256252845f29b38fdd401de9` |
| `.backlogit/archive/211.015-T.md` | `b56e34f7120ac75695c4e201961a469f8ccb763ed5a80b762c72d1c8da2a7d8b` |
| `.backlogit/queue/211.017-T.md` | `5cf19bd2c70232e2d2fe4cd33018ab017773eaef3e731f8350adc5b3ae41f387` |
| `.backlogit/queue/211.018-T.md` | `d03d01e30a70066ffb13d1df4e298a1a8b9d7faadf44f22c4a3b9b1d73764621` |
| `.backlogit/queue/211.019-T.md` | `dc976064ed1ca43db3c00345fa9e66337d6d592fb3819d42bb4460a63a30e493` |
| `.backlogit/queue/211.003-T.md` | `b084d47c8c1b18e599801930ecf3d472e0c3d2337d3923dd0b3a40748086b0df` |
| `.backlogit/queue/211.004-T.md` | `0cdf750ec175b44dc5b2dee32bd22e115543a81be5caeae01c090c4e0187a853` |
| `.backlogit/queue/211.005-T.md` | `e9b4155036274efe9a1596aeae6c20156998b281eb38cc3ac6da36cafbfbc4a9` |
| `.backlogit/queue/211.006-T.md` | `efb2cf366425ebd0d3d3be8d6a5290c017fa28e8b883b147b1cc7cc5963e067d` |
| `.backlogit/queue/211-F.md` | `59fe2f8f00776260a113ae83e72c568b23444dcdf97dba3e25aed31bdbd84deb` |
| `.backlogit/queue/212-S.md` | `97bcc602c7d27f9e7b2f9204cef608b658fd6f62c3f1a1c06532f9eed1709088` |
| `.backlogit/logs/211.017-T.jsonl` | `d758bb2f2e523783c87930f09a1ef3e27d2cf238b5a2a5eae23659af3c68174c` |
| `.backlogit/logs/211.018-T.jsonl` | `ac817f2fd08ce8948811ebe7ed9fcf88bf677c83d22f6fa4617395eae1f0f0f0` |
| `.backlogit/logs/211.019-T.jsonl` | `1d66bf1dde5cabea5280f9df8236908be046fbdd53869c755d43dbcc256abf84` |
| `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` | `b4be2ba91ce9546fcb0202abbe8fbd72b99720eea31e18feab96d7864fed78e6` |
| `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` | `5215e4437357a88a1779c26e67fb386644a8e00eac0d7b2950810bc71af49f5e` |

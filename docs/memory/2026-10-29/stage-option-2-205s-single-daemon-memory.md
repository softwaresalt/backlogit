---
chunk_strategy: h1-h2-h3
description: "Stage Option 2 session memory: deliberate, plan, review, harvest stash 760670A3 into feature 204-F (204.001-T..204.005-T), and assemble shipment 205-S."
doc_type: memory
docline:
  date: 2026-10-29T21:17:00Z
  feature_id: 204-F
  shipment_id: 205-S
  status: done
  tags:
    - stage
    - 760670A3
    - 077-DL
    - 204-F
    - 205-S
    - mcp-daemon
schema_version: "1.0"
source: docs/memory/2026-10-29/stage-option-2-205s-single-daemon-memory.md
title: "Stage memory: Option 2 single-daemon-per-workspace (stash 760670A3, feature 204-F, shipment 205-S)"
---

## Summary

* **Mode and route**: Stage Option 2 (`760670A3`, Single-Daemon-Per-Workspace) under operator directive (`2026-10-29T20:06:47Z`); model route `gemini-4-argon` / `google` / `high` / `long_context` (P-013.5).
* **Tool availability**: `backlogit` MCP tools active (`INDEX_SYNC_OK`); `engram` degraded (`TOOL_DEGRADED: engram`).
* **Startup recovery**: `backlogit_list_checkpoints(consumer_id: "stage")` returned zero active or quarantined checkpoints; `backlogit_poll_hook_events(consumer_id: "stage")` returned zero events.

## Pipeline Execution

### 1. Stash Triage and Deliberation (`760670A3` -> `077-DL`)

* Classified stash `760670A3` (`high` / `feature`) as feature-shaped.
* Authored deliberation `docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md` evaluating:
  * **Option A (Chosen)**: CLI-Only OS-Lock Single-Instance Workspace Guard (`<runtime_dir>/mcp-daemon.lock` + `<runtime_dir>/mcp-daemon.json`) + CLI lifecycle controls (`backlogit mcp --replace`, `backlogit mcp status`, `backlogit mcp stop`).
  * **Option B (Rejected)**: Detached background daemon with named pipe / Unix domain socket and stdio proxy.
  * **Option C (Rejected)**: External wrapper script only.
* Created backlogit deliberation `077-DL` via `backlogit_deliberate(stash_id: "760670A3")`, transitioned `077-DL` through `active` to `done`, and archived it to `.backlogit/archive/077-DL.md`.

### 2. Implementation Plan, Hardening, and Plan Review

* Authored `docs/exec-plans/2026-10-29-single-daemon-per-workspace-plan.md` with 5 sequential single-domain units (`U1`..`U5`), full `## Constitution Check` (`Constitution Check: pass`), `## Plan Hardening Signals` (`Requires plan hardening: yes`), `## Plan Hardening`, and `## Plan Review` (`dispatch_mode: single-agent-declared-degradation`, `decision: PASS`, `operator_authorization: approved`, `P0=0, P1=0, P2=0, P3=2 applied`).

### 3. Harvest and Shipment Assembly (`204-F`, `204.001-T`..`204.005-T`, `205-S`)

* Harvested `760670A3` into covering feature `204-F` (`Enforce single backlogit MCP daemon per workspace`), automatically archiving `760670A3` to `.backlogit/archive/stash.jsonl`.
* Created 5 atomic tasks under `204-F` in sequential dependency order (`U1 -> U2 -> U3 -> U4 -> U5`):
  * `204.001-T` (`size: S`, `domain-code`): Declare `ErrDaemonAlreadyRunning` sentinel and MCP daemon lock types + `go/ast` source-shape harness
  * `204.002-T` (`size: M`, `domain-code`, depends on `204.001-T`): Implement single-instance OS lock acquisition, metadata sidecar, and crash-recovery status probe
  * `204.003-T` (`size: M`, `domain-code`, depends on `204.002-T`): Implement `StopMCPDaemon` and `--replace` takeover + wire `mcp status` and `stop` CLI subcommands
  * `204.004-T` (`size: S`, `domain-tests`, depends on `204.003-T`): Update CLI-only registry parity allowlist and verify cross-process subprocess lock contention
  * `204.005-T` (`size: XS`, `domain-docs`, `harness-exempt: docs-only`, depends on `204.004-T`): Regenerate CLI reference documentation for `backlogit mcp`, `status`, and `stop`
* Assembled shipment `205-S` (`title: "Enforce single backlogit MCP daemon per workspace (204-F)"`, `priority: high`, `queue_position: 1400`, `dependencies: [154-S]`, `items: ["204-F", "204.001-T", "204.002-T", "204.003-T", "204.004-T", "204.005-T"]`).

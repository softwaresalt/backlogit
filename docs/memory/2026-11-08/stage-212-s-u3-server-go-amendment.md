---
title: "Stage 212-S Wave 6 U3 Server-Instance Session State File-Boundary Amendment"
shipment_id: "212-S"
feature_id: "211-F"
phase: "stage-wave6-u3-contract-amendment"
status: "ready-for-ship"
---

## Amendment Summary

Under Operator Option 5 authorization, Stage completed the surgical affected-files contract amendment for `211.003-T` (`U3`, Wave 6) in Shipment `212-S` (`211-F`), resolving the Wave 6 file-boundary halt reported in [212-S M19 Wave 6 missing Server state prerequisite halt](212-S-M19-wave6-server-state-halt.md):

* **Per-instance `*Server` session tracking in `internal/mcp/server.go` (`U3` / `211.003-T`)**: `211.003-T` (`U3`) explicitly lists `Affected files (3): internal/mcp/server.go, internal/mcp/session_mux.go, internal/mcp/session_mux_test.go` (within the 3-file TDD bound) and permits adding unexported per-instance session-registry/mutex/drain fields on `struct Server` (with optional initialization in `newServer`) in `internal/mcp/server.go` so `(*Server).ServeSessionStream`, `(*Server).ActiveSessionCount`, and `(*Server).CloseAllSessions` maintain per-instance state and multiple `*Server` instances in tests remain isolated without package-level globals.
* **Preserved scheduler, task, and log state**:
  * `211.003-T` retains `status: active`, `labels: [code, track-b, mcp, multi-session, u3, harness-ready]`, `custom_fields.scheduler_baseline_claim: 212-S`, and its existing `.backlogit/logs/211.003-T.jsonl` with its single `WORK_STARTED: 212-S` event (`2d3aba5293faa4cdd0506681db953e870b44079b3c79cbcbb65b7e9131019873`).
  * Checkpoints `checkpoint-20261109-040035.json` (`75bcabdeacf0d18e1cb8576c985307007a961183c0bd230ec40a1f4fcaa466c2`) and `checkpoint-20261106-210317.json` (`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`), Go source/test files (`internal/mcp/server.go`, `internal/mcp/session_mux.go`, `internal/mcp/session_mux_test.go`), and git history (`NOFEATURECOMMIT`) are untouched.

## Updated Stage-Owned File Digests (SHA-256)

| File | SHA-256 Digest |
|---|---|
| `.backlogit/queue/211.003-T.md` | `463e0630fa8c839305cb08d377d54f194928a8eb3f2a0dbcbb72430953066c3e` |
| `.backlogit/archive/089-DL.md` | `c671201e34d1dd834ddb053f13f259b3cb1c8894e8ec1e1d10020537467030d2` |
| `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` | `9a2df45d1ac6413b620441c65b80e9147ccee81114ca3b8592954788a182bce9` |
| `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` | `82ba9c20949ac01016120009470856bf6460806b2e15667cda9191c1baee0b19` |

## Verification Gate Results

* `backlogit_sync_index`: PASS (`indexed: 2261`)
* `backlogit_docs_lint` (`docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md`): PASS (`valid: true`, `violation_count: 0`)
* `backlogit_docs_lint` (`docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md`): PASS (`valid: true`, `violation_count: 0`)
* `backlogit_docs_lint` (`docs/memory/2026-11-08/stage-212-s-u3-server-go-amendment.md`): PASS (`valid: true`, `violation_count: 0`)

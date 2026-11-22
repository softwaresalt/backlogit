---
title: "221-S post-merge runtime verification"
description: "Post-merge runtime verification evidence on synchronized main for Shipment 221-S (Feature 220-F)."
doc_type: closure
source: docs/closure/221-S-post-merge-runtime-verification.md
ingested_at: "2026-11-22T03:02:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-22"
  mode: post-merge
  shipment_id: 221-S
  feature_id: 220-F
  merge_commit: a079f4527a9243413ea992ad0701225fac722cc5
  verification_verdict: PASS
---

## Scope and validator contract

Post-merge runtime verification for Shipment `221-S` (`220-F`, merged at `a079f4527a9243413ea992ad0701225fac722cc5`, PR `#670`) executed on synchronized `main` against `internal/mcp/session_mux.go`, `internal/mcp/session_mux_test.go`, `docs/configuration.md`, and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` per `.gsd/workspace-profile.yaml` (`validator_manifest.surfaces: []`, `validation_expectations.required: false`, `releasability.required: true`).

## Environment prechecks

* Synchronized local `main == origin/main == a079f4527a9243413ea992ad0701225fac722cc5` (`POST_MERGE_SYNC_OK`).
* Built post-merge binary (`go build -o logs/backlogit-221s-postmerge.exe ./cmd/backlogit`).
* Verified `TestU1_ValidateSessionPathMixedAndForwardSeparators` (`-race`) and `EXEMPT_VERIFY_OK:220.002-T`.
* Verified MCP daemon status and repo-wide Docline linting with the built post-merge CLI.

## Structured validator evidence

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| `go build -o logs/backlogit-221s-postmerge.exe ./cmd/backlogit` | Binary builds cleanly on merged `main` | `0` | Built `logs/backlogit-221s-postmerge.exe` |
| `go test -timeout=30m -count=1 -race -v -run '^TestU1_ValidateSessionPathMixedAndForwardSeparators$' ./internal/mcp` | Exact `1` top-level `PASS`, `0` skips/failures | `0` | `POSTMERGE_U1_PASS=1` |
| `.\logs\backlogit-221s-postmerge.exe --no-update-check mcp status --format json` | Running MCP daemon reports healthy IPC status | `0` | `"running": true, "ipc_transport": "named_pipe", "protocol_version": "backlogit-mcp-ipc/v1"` |
| `.\logs\backlogit-221s-postmerge.exe --no-update-check docs lint --format json` | `valid: true, violation_count: 0` | `0` | `{"valid":true,"violation_count":0,"findings":[]}` |
| `backlogit_docs_lint` (`ingestion` on `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`) | `valid: true, violation_count: 0` | `0` | `valid: true, violation_count: 0` on both |

## Verdict

`PASS`: all post-merge build, `validateSessionPath` containment, MCP daemon status, and Docline checks passed on synchronized `main` (`a079f4527a9243413ea992ad0701225fac722cc5`).

---
title: "218-S post-merge runtime verification"
description: "Post-merge runtime verification evidence on synchronized main for Shipment 218-S (Feature 217-F)."
doc_type: closure
source: docs/closure/218-S-post-merge-runtime-verification.md
ingested_at: "2026-11-19T16:25:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-19"
  mode: post-merge
  shipment_id: 218-S
  feature_id: 217-F
  merge_commit: 37966bf4b2af72ca8c822bfd29f186180a27cb08
  verification_verdict: PASS
---

## Scope and validator contract

Post-merge runtime verification for Shipment `218-S` (`217-F`, merged at `37966bf4b2af72ca8c822bfd29f186180a27cb08`, PR `#662`) executed against the newly built `.\backlogit.exe` (`go build ./cmd/backlogit`) on synchronized `main` using the `cli` adapter per `.gsd/workspace-profile.yaml` (`validator_manifest.surfaces: []`, `validation_expectations.required: false`, `releasability.required: true`).

## Environment prechecks

* Synchronized local `main == origin/main == 37966bf4b2af72ca8c822bfd29f186180a27cb08` (`POST_MERGE_SYNC_OK`).
* Built `.\backlogit.exe` via `go build ./cmd/backlogit` (`exit 0`).
* Host `.bin\backlogit.exe` remains locked by running MCP daemon processes (`PID 7816`, `PID 25480`); isolated probe workspaces exercised the freshly built `.\backlogit.exe` copied to `.bin\backlogit.exe` and `custom-bin\backlogit.exe`.

## Structured validator evidence

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| Probe 1 (`.bin\backlogit.exe` + `.mcp.json` -> `mcp prewarm`, `mcp status`, `mcp stop`) | Daemon starts from `.bin\backlogit.exe`, attaches cleanly, and stops cleanly | `0` | `running: true, protocol_version: backlogit-mcp-ipc/v1, stopped: true` |
| Probe 2 (`tools_dir: custom-bin` in `toolchain.yaml`, `./custom-bin/backlogit.exe` in `.mcp.json` -> `mcp status --toolchain --strict`, `mcp prewarm`, `mcp stop`) | Toolchain audit reports `valid: true, violations: 0` and daemon starts cleanly | `0` | `valid: true, violations: 0, running: true, stopped: true` |
| Probe 3 (`120` branches `refs/heads/hist-001`..`hist-120` -> `add --type feature`) | Branch-tree ID scan completes under 2s without timeout | `0` | `443ms Created feature: 001-F` |

## Verdict

`PASS`: all post-merge CLI probes passed on synchronized `main` (`37966bf4b2af72ca8c822bfd29f186180a27cb08`) with zero blocked prerequisites.

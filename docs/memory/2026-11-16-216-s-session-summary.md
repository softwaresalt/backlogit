---
title: "216-S session execution summary"
doc_type: memory
source: docs/memory/2026-11-16-216-s-session-summary.md
ingested_at: "2026-11-16T14:45:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 216-S
  feature_id: 215-F
  branch: feat/216-s-self-contained-sandbox-and-multi-session-runtime-hardening
---

## Scope and execution waves

Shipment `216-S` (`215-F`: Self-Contained Sandbox and Multi-Session Runtime Hardening (CAP-2)) executed across three dependency-ordered P-002.6 waves on branch `feat/216-s-self-contained-sandbox-and-multi-session-runtime-hardening`:

* **Wave 1**: `215.001-T` (`U1`, `internal/atomicfile` & `WriteToolchainManifest`), `215.003-T` (`U3a`, `mcp.Server.CloseCoordinator`/`Shutdown` and `CloseAllSessions` session-only semantics), and `215.005-T` (`U4`, `ExecGitRunner.Verify` TEMP ceiling).
* **Wave 2**: `215.002-T` (`U2`, aliased `backlogit` validation and `--cwd` binding), `215.004-T` (`U3b`, `RunMCPDaemon` shutdown wiring), and `215.006-T` (`U5a`, `.locks` ID sequence reservation & `id_allocator.lock` isolation).
* **Wave 3**: `215.007-T` (`U6`, `init`/`install` self-install runtime & ignore wiring) and `215.008-T` (`U5b`, `AdoptItem` & `ReconstituteShipments` ID reservation & rollback).

## Durable decisions and remediation

* Verified Windows directory handle identity via `VolumeSerialNumber`/`FileIndex` and `finalPathByHandle` in `atomicfile_windows.go` and `id_history_windows.go` so `SUBST` and mapped drives pass while symlink/junction swaps fail closed.
* Kept `lockArtifactMutation` outside `id_allocator.lock` (`BeforeRename` records journal adoptions only; `applyReconstitution` acquires the new-ID artifact lock after `AdoptItemWithOptions` returns) to preserve `stash -> lifecycle-global -> A -> C -> B -> id_allocator.lock`.
* Captured out-of-scope review findings under P-021 C2 as `39F868B1`, `E0A6FDAF`, `7C26BE01`, and `DE5D1EA5` in `.backlogit/stash.jsonl`.

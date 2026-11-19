---
title: "218-S / 217-F post-merge closure"
description: "Post-merge operational closure and releasability evidence for Shipment 218-S (Feature 217-F)."
doc_type: closure
source: docs/closure/218-S-217-F-post-merge-closure.md
ingested_at: "2026-11-19T16:28:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
docline:
  shipment_id: 218-S
  feature_id: 217-F
  merge_commit: 37966bf4b2af72ca8c822bfd29f186180a27cb08
  close_path: cascade
  close_evidence: docs/closure/evidence/218-S-217-F-close-evidence.json
  closure_status: READY
  compaction_status: done
---

## Release and governed scope

PR `#662` (`https://github.com/softwaresalt/backlogit/pull/662`, `headRefOid: 4989ceaf23800f4c36f3df22344d34469319cd6d`) passed current-HEAD local multi-persona reviews (`READY_WITH_FOLLOWUPS`, `P0=0, P1=0`), 8/8 CI checks, P-018 `gsd gate copilot-review 662` (`PASS`, `0` unresolved blocking, `0` advisory, `2` resolved), and P-009 merge-commit-only settings, merging via commit `37966bf4b2af72ca8c822bfd29f186180a27cb08` at `2026-11-19T16:11:49Z`.

Local `main` was synchronized via fast-forward-only sync (`HEAD == origin/main == 37966bf4b2af72ca8c822bfd29f186180a27cb08`) before creating `post-merge/218-s-closure`. Lifecycle topology gate returned `CARRY_FORWARD_ELIGIBLE` (`exit_code: 0`).

Flat explicit membership is `M = [217-F, 217.001-T, 217.004-T, 217.002-T, 217.003-T]` plus shipment control `218-S`. Pre-close, safe-close, and post-close reconciliation passed under one held `.backlogit/queue/.218-S.md.agent-lock`, which was released at post-mode completion. `218-S` is archived with `archived_status: shipped`; `217-F` and `217.001-T`..`217.004-T` are archived with `archived_status: done` and merge commit `37966bf4b2af72ca8c822bfd29f186180a27cb08`. Zero non-member changes and zero tracked archive deletions occurred (`P-007 PASS`).

| Governing report | Result |
| --- | --- |
| `.backlogit/reconcile/218-S-pre-20261119T162000Z.md` | `PROCEED` |
| `.backlogit/reconcile/218-S-safe-close-20261119T162100Z.md` | `CLOSED` |
| `.backlogit/reconcile/218-S-post-20261119T162200Z.md` | `PROCEED` |

Canonical governed evidence: [218-S close evidence](evidence/218-S-217-F-close-evidence.json) (`shipment_status: shipped`, `returned_ids: []`, `phase: post_close`, `postcondition_verdict: pass`, `failures: []`).

## Invariants and validator evidence

* `internal/toolchain/{types.go,verify.go}` (`217.001-T`): Supports `.bin/backlogit[.exe]`, `bin/backlogit[.exe]`, and configured `tools_dir` (including mixed inventories via `!isBacklogitTool` early return) while rejecting `.git`/`.backlog`/`.backlogit`, traversal, drive/UNC prefixes, and symlinks/junctions.
* `internal/core/id_history.go` (`217.004-T`): Queries all `refs/heads` root tree OIDs via `git for-each-ref`, deduplicates root/subtree OIDs, batches `git cat-file --batch` (with `64 MiB` cap, regular-file mode/`path.Ext` filter, and `allowMissing = false` on referenced subtrees) alongside `branchIDScanCache`, preserving merged-then-reverted branch reservations.
* `schemas/workspace-toolchain.schema.json` & `internal/toolchain/launcher.go` (`217.002-T`): Declares optional `tools_dir`, validates provisioning manifests, and emits manifest-aware remediation paths (defaulting to `.bin` while preserving `./bin/...` for legacy `bin` manifests).
* `internal/config/{schema.go,loader.go}` & `internal/cli/mcp_daemon.go` (`217.003-T`): Resolves `.mcp.json` -> `toolchain.yaml` -> `config.yaml` `tools_dir` -> `.bin` -> `bin` via step-by-step pinned child `*os.Root` handle traversal (`openLocalFileInRoot` / `verifyPinnedDaemonBinaryRel`), rejecting drive/UNC prefixes, duplicate JSON/YAML keys, non-pinned `containment_class`, and reserved storage dirs.

See [post-merge runtime verification](218-S-post-merge-runtime-verification.md) for full probe output (`verdict: PASS`).

## Pre-deploy audit and rollout

| Check | State |
| --- | --- |
| Deployment / rollout path | Merge-only CLI/library release; no schema migration required (`tools_dir` is optional `omitempty`) |
| Pre-deploy audit | All Go packages (`-race` on modified packages and full CI `test` + `test-windows`), `go vet`, `golangci-lint`, `gofmt`, `md-lint`, `docs lint`, and `wave-scheduler-sim` passed |
| Post-deploy checks | `go build ./cmd/backlogit` on synchronized `main` and 3 isolated CLI runtime probes (`.bin` prewarm/status/stop, `custom-bin` `tools_dir` strict toolchain + prewarm/stop, 120-branch `add` in `443ms`) passed |
| Risky action: governed cascade close | `ActionRisk: destructive`; authorized via autopilot closure; `ActionResult: applied` (`shipment_status: shipped`, `returned_ids: []`) |

## Monitoring plan, owner, and rollback

* **Owner**: `_Ship` agent (closure verification); operator (rollback authorization and host `.bin\backlogit.exe` refresh); `_Stage` agent (deferred follow-up triage).
* **Validation window**: Immediate post-merge checks (passed) plus 7-day observation window (`2026-11-19` through `2026-11-26`).
* **Healthy signals**: Zero `ErrDaemonBinaryMissing` or `ErrDaemonVersionMismatch` errors on `.bin/` and `.mcp.json` workspaces; `go test -timeout=30m ./...` and `make docs-lint` green.
* **Failure signals / rollback trigger**: Any containment regression, daemon startup failure, or ID allocation regression on `main`.
* **Rollback procedure**: Preserve archived backlog/event history and submit an operator-approved `git revert -m 1 37966bf4b2af72ca8c822bfd29f186180a27cb08` PR (excluding `.backlogit/archive/` history).

## Source artifact provenance

Feature `217-F` records `source_stash_id: 2CCDFDED` and `source_deliberation_id: 102-DL` (`docs/decisions/2026-11-19-mcp-daemon-configurable-tools-dir-deliberation.md`, archived `.backlogit/archive/102-DL.md`). Referenced source stash and deliberation artifacts are retained untouched as provenance only.

## Tracked residual risks and P-021 C2 deferrals

* Three out-of-scope stash entries remain active in `.backlogit/stash.jsonl` for Stage triage:
  * **`85442104`**: Companion PowerShell helper scripts (`tools/mcp/install.ps1`, `scripts/build-dev.ps1`) deferred during Stage intake (`102-DL`).
  * **`F1D193CC`**: P-021 C2 deferred scope expansion — align `AuditRepositoryToolchain` / `internal/toolchain` with `<storage>/config.yaml` `tools_dir`, share `.mcp.json`/`toolchain.yaml` parsing and pinned child `*os.Root` traversal, and relocate `WorkspaceRootCandidates` below `internal/core`.
  * **`618AA1C0`**: P-021 C2 deferred scope expansion — bound pre-existing `io.ReadAll` calls with `io.LimitReader` in `internal/toolchain/verify.go`, `internal/toolchain/launcher.go`, and `internal/core/id_history.go` (`readIDSequenceLedger`).
* Host `.bin\backlogit.exe` was locked by running MCP daemon processes (`PID 7816`, `PID 25480`) during this run and should be refreshed once active MCP sessions restart (stashed as follow-up `82DAA5FC`).

## Knowledge refresh, P-020 compaction, and follow-ups

* Invoked `compound-refresh` (`scope: recent`, `mode: propose`; [218-S compound refresh](218-S-compound-refresh.md)): 5 compound entries reviewed and kept (`keep: 5`).
* Invoked `compact-context` (`target: all`; [218-S compaction report](2026-11-19-218-s-compaction-report.md)): created [218-S compacted memory](../memory/compacted/2026-11-19-218-s-217-f-compacted.md) and finalized `compaction_status: done`.

## Structured releasability and compaction

| Evidence | Profile requirement | State |
| --- | --- | --- |
| Validator evidence | `required: false`, `minimum_verdict: PASS` | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Mandatory post-merge closure duty | `done` |

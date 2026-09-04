---
title: "Compacted Memory — 2026-08-10 Ship Sessions (120-S, 121-S, 122-S)"
doc_type: memory
schema_version: "1.0"
ingested_at: "2026-08-20T04:45:00Z"
---

Source files (archived to `docs/archive/memory/2026-08-10/`):

* `120-s-f5-ship-session.md`
* `121-s-ship-session-memory.md`
* `122-s-dark-mode-visibility.md`
* `122-s-ship-session.md`
* `default-workspace-dir-rename-memory.md`

## 120-S — F5 Idempotent Multi-Mutation Envelope (COMPLETE, shipped/archived)

* Tasks 106.025-T…106.031-T (U1-U7) + `106-F` covering feature archived.
  PR #340 merged at `1f046895e8058e97c5eec1fe5784fba758df7ce2`.
* `MutationEnvelope`/`MutationPartialError` (new, `internal/errors`,
  `internal/core/mutation_envelope.go`) wraps `AssociateCommit`,
  `CreateArtifact`, `AddDependency`, `AddItemToShipment` for idempotent
  recovery. `doctor.CheckPartialMutations` + CLI/MCP surface added.
  Design doc: `docs/design-docs/governed-mutation-recovery-contract.md`.
* **Issue encountered**: gate evidence missing for pre-gate tasks
  `106.001-T`/`106.002-T` (shipped earlier in F2/F3, before formal gating
  existed) blocked `ShipShipment`; fixed by writing `EventGateForced` events
  with `head_sha=1f046895` before re-running ship. Same class of issue as
  the 118-S MCP-timeout repair — gate evidence gaps on older/pre-gate tasks
  are a recurring closure friction point worth checking early in future
  `shipment ship` attempts.

## 121-S — Default Workspace Directory Rename to `.backlog` (COMPLETE, shipped/archived)

* Tasks 135.001-T…135.009-T (U1-U9) + `135-F` archived. PR #345 merged at
  `99e8ecc8`. 12 Copilot review threads resolved across 5 CI push cycles.
* Implemented dual-root support: `.backlog/` default for new workspaces,
  `.backlogit/` legacy-supported. `Workspace.StorageRoot` now stored
  separately from `RootPath`; `BACKLOGIT_WORKSPACE_DIR` is a closed-set
  override (`.backlog`|`.backlogit`); ambiguous dual-root workspaces fail
  closed with a typed error.
* Symlink/reparse-point guards added to archive and queue candidate scan
  loops (`canonical_scan.go`, `shipment_verify.go`); exported
  `IsSymlinkOrReparsePoint` helper.
* **Deferred (tracked as follow-up, not done)**: full `WorkspaceStorageRoot`
  fallback change (`.backlogit`→`.backlog` default) requires updating every
  `WorkspaceStorageRoot(ws.RootPath)` call site to `workspaceStorageRoot(ws)`
  instead — attempting this directly broke
  `TestDoctor_FixOrphansArchivesOrphanedTask` because
  `probeWorkspaceCandidate` requires `config.yaml`, which many tests don't
  create. Partial safe fix applied instead (`artifactSearchDirs` uses
  `workspaceStorageRoot(ws)`). Also deferred: removing `t.Parallel()` from
  `workspace_dualroot_test.go` tests that mutate `BACKLOGIT_WORKSPACE_DIR`
  (env var pollution risk under parallelism).
* AGENTS.md noted as larger than architecture-doc guidance recommends —
  only storage-root wording was updated this session, not a full rewrite.

## 122-S — Administrative Checkpoint Disposition (COMPLETE, shipped/archived)

* Tasks 136.001-T…136.013-T + `136-F` archived (final commit `04d14fbe`,
  the post-fix-forward main tip — not the intermediate pre-fix merge).
  Feature PR #342 merged (`5b963353`).
* New: `CheckpointAbandon`/`CheckpointQuarantine` CLI+MCP surfaces
  (`internal/core/checkpoint_disposition.go`, `checkpoint_target.go`,
  `checkpoint_audit.go`); disjoint by design (abandon=valid, quarantine=
  malformed-only). Design doc:
  `docs/design-docs/checkpoint-administrative-disposition.md`.
* **Review-fix cycle limit exceeded (5 rounds vs. the documented 3)** — same
  category of deviation as 117-S. Findings decreased in severity each round;
  rounds 4-5 (TOCTOU classify-move race, unwind-error discard,
  `durable_writes` config gap) were explicitly accepted as backlog
  (`136.014-T`, `136.015-T`, `136.016-T`) with decline-with-rationale replies
  posted before resolving threads, once the limit was clearly exceeded.
* **Post-merge runtime verification caught a genuine regression** not
  covered by any unit test: `ResolveDispositionTarget` joined a relative
  `--cwd` candidate path directly (not via `filepath.Abs`) before
  `confineToStorageRoot`, corrupting containment for relative workspace
  roots. All unit tests used `t.TempDir()` (always absolute), so the bug was
  invisible to them — only caught by exercising a real compiled binary with
  a relative `--cwd`. Treated as release-blocking (not deferred to backlog)
  and fixed immediately via a dedicated same-session PR #343 (`04d14fbe`)
  with full TDD (confirmed RED before fix). See compound learning
  `2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`.
  **Lesson reinforced**: unit tests using `t.TempDir()` cannot catch
  relative-path confinement bugs — runtime verification with a real relative
  `--cwd` invocation is load-bearing, not redundant with unit tests.
* Chose `os.Link`+`os.Remove` (atomic no-replace move) over `os.Stat`+
  `os.Rename` for the quarantine clobber-refuse guard (closes a TOCTOU
  race); used the existing `atomicfile.WriteFileAtomic` primitive instead of
  a hand-rolled temp-then-rename helper (fixes a Windows rename-cannot-
  replace-existing bug).
* Residual exposure noted: this repo's own dogfooded pinned binary
  (`C:\Tools\backlogit.exe`) did not yet include these new commands at the
  time — consistent with the existing compound learning on self-hosted CLI
  version skew (`2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`).

## Status

All three shipments (120-S, 121-S, 122-S) confirmed merged/archived via
`gh pr list` cross-check (closure PRs #341, #346, #344). Follow-up tasks
136.014-T/136.015-T/136.016-T remain queued in the live backlog for future
pickup — not stale, do not archive/compact those queue entries themselves.

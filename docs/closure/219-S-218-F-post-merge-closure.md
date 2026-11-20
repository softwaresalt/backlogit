---
title: "219-S / 218-F post-merge closure"
description: "Post-merge operational closure and releasability evidence for Shipment 219-S (Feature 218-F)."
doc_type: closure
source: docs/closure/219-S-218-F-post-merge-closure.md
ingested_at: "2026-11-20T19:56:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
docline:
  shipment_id: 219-S
  feature_id: 218-F
  merge_commit: 7229def0c1308955db257e4eec69d842b93f0256
  close_path: cascade
  close_evidence: docs/closure/evidence/219-S-218-F-close-evidence.json
  closure_status: READY
  compaction_status: done
---

## Release and governed scope

PR `#665` (`https://github.com/softwaresalt/backlogit/pull/665`, `headRefOid: 8d773ef16c92f77a2a7c14087bb5825f5d3ccf8b`) passed current-HEAD local multi-persona reviews (`READY_WITH_FOLLOWUPS`, `P0=0, P1=0`), `9/9` CI checks, P-018 `gsd gate copilot-review 665` (`PASS`, `0` unresolved blocking, `0` advisory, `5` resolved), and P-009 merge-commit-only settings, merging via commit `7229def0c1308955db257e4eec69d842b93f0256` at `2026-11-20T19:35:06Z`.

Local `main` was synchronized via fast-forward-only sync (`HEAD == origin/main == 7229def0c1308955db257e4eec69d842b93f0256`) before creating `post-merge/219-s-closure`. Lifecycle topology gate returned `CARRY_FORWARD_ELIGIBLE` (`exit_code: 0`).

Flat explicit membership is `M = [218-F, 218.001-T, 218.003-T, 218.006-T, 218.008-T, 218.002-T, 218.007-T, 218.004-T, 218.005-T]` plus shipment control `219-S`. Pre-close, safe-close, and post-close reconciliation passed under one held `.backlogit/queue/.219-S.md.agent-lock`, which was released at post-mode completion. `219-S` is archived with `archived_status: shipped`; `218-F` and `218.001-T`..`218.008-T` are archived with `archived_status: done` and merge commit `7229def0c1308955db257e4eec69d842b93f0256`. Zero non-member changes and zero tracked archive deletions occurred (`P-007 PASS`).

| Governing report | Result |
| --- | --- |
| `.backlogit/reconcile/219-S-pre-20261120T194000Z.md` | `PROCEED` |
| `.backlogit/reconcile/219-S-safe-close-20261120T194600Z.md` | `CLOSED` |
| `.backlogit/reconcile/219-S-post-20261120T194700Z.md` | `PROCEED` |

Canonical governed evidence: [219-S close evidence](evidence/219-S-218-F-close-evidence.json) (`shipment_status: shipped`, `returned_ids: []`, `phase: post_close`, `postcondition_verdict: pass`, `failures: []`).

## Invariants and validator evidence

* `internal/cli/self_install.go` (`218.001-T`, `218.002-T`): Handle-anchored workspace root creation/opening (`openOrCreateValidatedWorkspaceRoot`), target tool directory creation (`ensureAndOpenTargetToolsDirWithCreated`), non-short-circuiting `os.SameFile` self-install, literal-escaped `.gitignore`/`.git/info/exclude` rules, canonical `.mcp.json` (`mcpServers.backlogit`) provisioning, launcher migration (`start.ps1`/`start.sh`), `toolchain.yaml` SHA-256 reconciliation preserving raw `workspace_root`, and LIFO rollback.
* `internal/cli/mcp_daemon.go` (`218.003-T`): Unconditional non-short-circuiting 5-source preflight (`defaultTargetToolsDir`, `resolveConfiguredDaemonBinaryRel`), control-character rejection, and `BACKLOGIT_WORKSPACE_DIR=""` unset parity.
* `internal/toolchain/verify.go` & `internal/toolchain/launcher.go` (`218.006-T`, `218.008-T`): Relative `workspace_root` resolution against `abs`, `BACKLOGIT_WORKSPACE_DIR` override on dual-root workspaces, and `VerifyProposedLauncherScripts` in-memory pre-install validation.
* `internal/config/defaults.go` & `internal/cli/root.go` (`218.007-T`, `218.004-T`): Pure in-memory bootstrap file rendering (`RenderDefaultBootstrapFiles`), handle-confined `writeWorkspaceBootstrapDefaults`, `--tools-dir` flag, `--jsonrpc` non-interactive path, and TTY/piped stdin prompting.
* `docs/cli-reference/backlogit_init.md` & `cmd/gen-docs/main_test.go` (`218.005-T`): Regenerated CLI reference with zero drift.

See [post-merge runtime verification](219-S-post-merge-runtime-verification.md) for full probe output (`verdict: PASS`).

## Pre-deploy audit and rollout

| Check | State |
| --- | --- |
| Deployment / rollout path | Merge-only CLI/library release; additive `--tools-dir` flag and workspace-local `.mcp.json` provisioning |
| Pre-deploy audit | All Go packages (`-race` on modified packages and full CI `9/9` including `test`, `Windows handle/lock tests`, `CLI Reference Drift`, `Docline frontmatter gate`), `go vet`, `gofmt`, `md-lint`, `docs lint`, and `pipeline-topology` passed |
| Post-deploy checks | `go build -o logs/backlogit-probe.exe ./cmd/backlogit` on synchronized `main` and 4 isolated CLI runtime probes (`init --tools-dir`, piped stdin prompt, fail-closed negative preflight, and `install` reconciliation + `mcp status --toolchain --strict` + daemon start) passed |
| Risky action: governed cascade close | `ActionRisk: destructive`; authorized via dark-mode closure; `ActionResult: applied` (`shipment_status: shipped`, `returned_ids: []`) |

## Monitoring plan, owner, and rollback

* **Owner**: `_Ship` agent (closure verification); operator (rollback authorization and post-closure `Auto-Tune` via `BE9AD4F1`); `_Stage` agent (deferred follow-up triage).
* **Validation window**: Immediate post-merge checks (passed) plus 7-day observation window (`2026-11-20` through `2026-11-27`).
* **Healthy signals**: Zero `ErrValidation` containment regressions on `init`/`install`; `mcp status --toolchain --strict` and `go test -timeout=30m ./...` green.
* **Failure signals / rollback trigger**: Any target-directory path escape, `.mcp.json` / `toolchain.yaml` corruption, or `init`/`install` prompt regression on `main`.
* **Rollback procedure**: Preserve archived backlog/event history and submit an operator-approved `git revert -m 1 7229def0c1308955db257e4eec69d842b93f0256` PR (excluding `.backlogit/archive/` history).

## Source artifact provenance

Feature `218-F` records `source_stash_id: 269D054D` and `source_deliberation_id: 103-DL` (`docs/decisions/2026-11-19-init-install-target-tools-dir-mcp-config-deliberation.md`, archived `.backlogit/archive/103-DL.md`). Referenced source stash and deliberation artifacts are retained untouched as provenance only.

## Tracked residual risks, P-021 C2 deferrals, and follow-up stash entries

Eight active stash entries in `.backlogit/stash.jsonl` remain queued for Stage triage (not mutated by Ship):

* **`159B162F`** (`high`, `bug` — Operator Directive #2): Broaden upstream `gsd` `schemas/carry-forward/1.0.0.schema.json`, loader, and `carry-forward.yaml` template beyond `^\.gsd/` to allow safe workspace-relative paths outside explicit denylisted paths (`.git/`, `.gsd/gates/`, `.gsd/backlog-registry.yaml`, `.gsd/stage-worktree-marker.yaml`).
* **`EF372865`**: Update `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` for `tools_dir` and `.bin` default (`P-021 C2`).
* **`CC4F1741`**: Extend `validateMCPConfigPinning` in `internal/toolchain/verify.go` to audit `.mcp.json` `servers` blocks (`P-021 C2`).
* **`6470AD6F`**: Cross-package consolidation of `os.Root` traversal, JSON/YAML scanning, and ancestor reparse policy (`P-021 C2`).
* **`63E74B09`**: Unify `BACKLOGIT_WORKSPACE_DIR=""` handling and `.backlog`/`.backlogit` defaults across `init`, `install`, and `core` (`P-021 C2`).
* **`BE9AD4F1`**: Reconcile carried-forward `.gsd/config.yaml` `model_routing` edits with `.gsd/harness-manifest.yaml` (`config_hash` / `gsd tune` / `TestU12_HarnessManifestNamespaceReconciliation`), retained for post-closure `Auto-Tune` (`P-021 C2`).
* **`B5A55936`**: Support splitting shared tool-directory variables in `migrateLauncherBytes` when relocating only `backlogit` in multi-tool `start.ps1`/`start.sh` launchers (`P-021 C2`).
* **`5A7EACC6`**: Add an explicit non-interactive flag or bounded stdin timeout for `resolveInitTargetToolsDir` under open idle non-TTY pipes (`P-021 C2`).

## Knowledge refresh, P-020 compaction, and follow-ups

* Invoked `compound-refresh` (`scope: recent`, `mode: propose`; [219-S compound refresh](219-S-compound-refresh.md)): 7 compound entries reviewed and kept (`keep: 7`).
* Invoked `compact-context` (`target: all`; [219-S compaction report](2026-11-20-219-s-compaction-report.md)): created [219-S compacted memory](../memory/compacted/2026-11-20-219-s-218-f-compacted.md) and finalized `compaction_status: done`.

## Dark-mode closure summary (`DARK_MODE_ACTIVE`, `P-017`)

* **Degraded remote visibility**: `agent-intercom` is not installed; all dark-mode events (`LOCAL_REVIEW_READY`, `DARK_MODE_MERGE_AUTHORIZED`, `DARK_MODE_COMPLETE`) recorded in local session output and PR readiness/closure records.
* **Reviewed HEADs**: Feature PR `#665` reviewed at `8d773ef16c92f77a2a7c14087bb5825f5d3ccf8b` (`READY_WITH_FOLLOWUPS`, `P0=0, P1=0`); merged via merge commit `7229def0c1308955db257e4eec69d842b93f0256` (normal merge, no admin fallback).
* **P-020 compaction status**: `done`.
* **Closure status**: `READY`.

## Structured releasability and compaction

| Evidence | Profile requirement | State |
| --- | --- | --- |
| Validator evidence | `required: false`, `minimum_verdict: PASS` | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Mandatory post-merge closure duty | `done` |

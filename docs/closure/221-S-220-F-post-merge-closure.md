---
title: "221-S / 220-F post-merge closure"
description: "Post-merge operational closure and releasability evidence for Shipment 221-S (Feature 220-F)."
doc_type: closure
source: docs/closure/221-S-220-F-post-merge-closure.md
ingested_at: "2026-11-22T03:02:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
docline:
  shipment_id: 221-S
  feature_id: 220-F
  merge_commit: a079f4527a9243413ea992ad0701225fac722cc5
  close_path: safe-close
  close_evidence: docs/closure/evidence/221-S-220-F-close-evidence.json
  closure_status: READY
  compaction_status: done
---

## Release and governed scope

PR `#670` (`https://github.com/softwaresalt/backlogit/pull/670`, `headRefOid: 9c750ba2833abb58444e4f896c36d78d56c4520c`) passed current-HEAD local `Adversarial Review` (`mode: report-only` -> `READY`, `P0=0`, `0` HIGH/MEDIUM P0/P1) + `Correctness Reviewer` (`READY`, `0` findings), `8/8` CI checks, P-018 `gsd gate copilot-review 670` (`PASS`, `0` unresolved blocking, `0` advisory, `0` resolved, Copilot review body `6009796844` approval recommended with zero issues), and P-009 merge-commit-only settings, merging via commit `a079f4527a9243413ea992ad0701225fac722cc5` at `2026-11-22T02:50:59Z`.

Local `main` was synchronized via fast-forward-only sync (`HEAD == origin/main == a079f4527a9243413ea992ad0701225fac722cc5`) before creating `post-merge/221-s-closure`. Lifecycle topology gate returned `CARRY_FORWARD_ELIGIBLE` (`exit_code: 0`).

Flat explicit membership is `M = [220-F, 220.001-T, 220.002-T]` plus shipment control `221-S`. Pre-close, safe-close, and post-close reconciliation passed under one held `.backlogit/queue/.221-S.md.agent-lock`, which was released at post-mode completion. `221-S` is archived with `archived_status: shipped`; `220-F`, `220.001-T`, and `220.002-T` are archived with `archived_status: done` and merge commit `a079f4527a9243413ea992ad0701225fac722cc5`. Zero non-member changes and zero tracked archive deletions occurred (`P-007 PASS`).

| Governing report | Result |
| --- | --- |
| `.backlogit/reconcile/221-S-pre-20261122T025740Z.md` | `PROCEED` |
| `.backlogit/reconcile/221-S-safe-close-20261122T025829Z.md` | `CLOSED` |
| `.backlogit/reconcile/221-S-post-20261122T025829Z.md` | `PROCEED` |

Canonical governed evidence: [221-S close evidence](evidence/221-S-220-F-close-evidence.json) (`shipment_status: shipped`, `returned_ids: []`, `phase: post_close`, `postcondition_verdict: pass`, `failures: []`).

## Invariants and validator evidence

* **MCP `validateSessionPath` separator normalization (`220.001-T`)**: `(*Server).validateSessionPath` (`internal/mcp/session_mux.go`) delegates to `(*Server).validateSessionPathForOS(path, goos string)`, normalizing both `/` and `\` via package-level `strings.NewReplacer` before `isLocalSessionPathForOS`, `..` traversal rejection, and component-wise `root.Lstat` + `core.IsSymlinkOrReparsePoint` inspection. `isLocalSessionPathForOS` unconditionally rejects empty `goos`, `""`, `\x00`, leading `/` or `\`, and lexical `..`, and when `goos == "windows"` additionally rejects any `:` (drive prefixes and NTFS ADS), trailing `.`/` ` segments, and Windows reserved device names (`CON`, `PRN`, `AUX`, `NUL`, `CONIN$`, `CONOUT$`, `COM1`–`COM9`/`COM¹`–`COM³`, `LPT1`–`LPT9`/`LPT¹`–`LPT³`), guarded by `TestU1_ValidateSessionPathMixedAndForwardSeparators` (`internal/mcp/session_mux_test.go`).
* **Toolchain `tools_dir` documentation (`220.002-T`)**: `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` document optional `tools_dir` in `config.yaml` and `toolchain.yaml`, 5-source discovery precedence (`.mcp.json` -> `toolchain.yaml` -> `config.yaml` -> `.bin` -> `bin` fallback), `VerifyProposedLauncherScripts`, and `backlogit init`/`install [--tools-dir <dir>]` provisioning (`--jsonrpc` vs `backlogit_get_metadata_catalog`) with zero edits to `docs/cli-reference/*.md`.

See [post-merge runtime verification](221-S-post-merge-runtime-verification.md) for full probe output (`verdict: PASS`).

## Pre-deploy audit and rollout

| Check | State |
| --- | --- |
| Deployment / rollout path | Merge-only MCP path validation fix, cross-OS unit tests, and configuration/design documentation |
| Pre-deploy audit | All `8/8` CI checks on PR `#670`, `go vet ./...`, CI-pinned `golangci-lint@v1.64.8`, governed `079-DL`/`105-DL`/`106-DL` `gofmt -l cmd internal tests scripts`, `EXEMPT_VERIFY_OK:220.002-T`, `markdownlint-cli2`, `backlogit_docs_lint`, and `GOTOOLCHAIN=go1.24.0 go test -timeout=30m -p 1 ./...` passed |
| Post-deploy checks | `go build ./cmd/backlogit`, `TestU1_ValidateSessionPathMixedAndForwardSeparators` (`-race`), `mcp status --format json`, and `docs lint` (`authoring` + `ingestion`) passed on synchronized `main` |
| Risky action: governed safe-close | `ActionRisk: destructive`; authorized via dark-mode closure; `ActionResult: applied` (`shipment_status: shipped`, `returned_ids: []`) |

## Monitoring plan, owner, and rollback

* **Owner**: `_Ship` agent (closure verification and next queued shipment `222-S` execution via `_Orchestrator`); operator (rollback authorization); `_Stage` agent (follow-up stash triage).
* **Validation window**: Immediate post-merge checks (passed) plus 7-day observation window (`2026-11-22` through `2026-11-29`).
* **Healthy signals**: `TestU1_ValidateSessionPathMixedAndForwardSeparators` green on both Windows and Ubuntu CI; `backlogit docs lint` reports `0` violations; `ServeSessionStream` rejects mixed-separator and junction traversal attempts with `validation_failed`.
* **Failure signals / rollback trigger**: Any `validateSessionPath` regression breaking valid workspace-relative MCP paths or failing containment on Windows/POSIX hosts.
* **Rollback procedure**: Preserve archived backlog/event history and submit an operator-approved `git revert -m 1 a079f4527a9243413ea992ad0701225fac722cc5` PR (excluding `.backlogit/archive/` history).

## Source artifact provenance

Feature `220-F` records `source_stash_id: 39F868B1` and `source_deliberation_id: 105-DL` (archived `.backlogit/archive/105-DL.md`); Task `220.002-T` records `source_stash_id: EF372865` and `source_deliberation_id: 106-DL` (archived `.backlogit/archive/106-DL.md`). Source stash entries `39F868B1` and `EF372865` and deliberations `105-DL` and `106-DL` were left untouched by `221-S` closure as provenance only.

## Tracked residual risks, P-021 C2 deferrals, and follow-up stash entries

Five pre-existing out-of-scope `P-021 C2` deferred scope expansions captured during `221-S` remain active in `.backlogit/stash.jsonl` for Stage triage (not mutated by Ship per P-021 C5 single-write capture invariant):

* **`DCE8FD71`**: Reject leading/trailing whitespace in `(*Server).validateSessionPathForOS` (`internal/mcp/session_mux.go`) instead of applying `strings.TrimSpace(path)`.
* **`272A805F`**: Enforce `validateSessionPath` (or reject non-notification requests with missing/null `envelope.ID` before `s.mcp.HandleMessage`) in `(*Server).dispatchSessionFrame` (`internal/mcp/session_mux.go:207`).
* **`5C70ECD0`**: Fail closed with `corerrors.ErrValidation` in `(*Server).validateSessionPathForOS` when `strings.TrimSpace(s.RootPath)` is empty instead of letting `filepath.Abs("")` resolve to CWD.
* **`8827BED4`**: Update `u6BuildBinary` in `internal/cli/mcp_session_shim_test.go:404` on Windows to invoke `go build` directly instead of requiring gitignored `bin/tmp/212-go-env.ps1`.
* **`A748BF57`**: Reconcile pre-existing `kbdb serve --read-only` instruction in `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md:138` with `ValidateMCPConfigPinning` (`["serve"]` only) and `.backlogit/` workspace default wording in `docs/configuration.md:48`.

## Knowledge refresh, P-020 compaction, and follow-ups

* Invoked `compound-refresh` (`scope: recent`, `mode: propose`; [221-S compound refresh](221-S-compound-refresh.md)): 4 compound entries reviewed and kept (`keep: 4`).
* Invoked `compact-context` (`target: all`; [221-S compaction report](2026-11-22-221-s-compaction-report.md)): created [221-S compacted memory](../memory/compacted/2026-11-22-221-s-220-f-compacted.md) and finalized `compaction_status: done`.

## Dark-mode closure summary (`DARK_MODE_ACTIVE`, `P-017`)

* **Degraded visibility / routing**: `agent-intercom` is not installed; all dark-mode events (`LOCAL_REVIEW_READY`, `DARK_MODE_MERGE_AUTHORIZED`, and final `DARK_MODE_COMPLETE` emitted once this closure PR merges to `main`) are recorded in local session output and PR readiness/closure records. `ROUTING_DEGRADED: context_tier` (`P-013.5`, non-blocking) and `ENGRAM_DEGRADED` (`[PACK-ROUTING]`) recorded per contract.
* **Reviewed HEADs**: Feature PR `#670` reviewed at `9c750ba2833abb58444e4f896c36d78d56c4520c` (`READY_WITH_FOLLOWUPS`, `P0=0, P1=0` blocking); merged via merge commit `a079f4527a9243413ea992ad0701225fac722cc5` (normal merge, no admin fallback). Closure PR reviewed at current HEAD prior to merge.
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

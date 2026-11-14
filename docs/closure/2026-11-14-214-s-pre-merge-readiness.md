---
title: "214-S pre-merge readiness"
doc_type: closure
source: docs/closure/2026-11-14-214-s-pre-merge-readiness.md
ingested_at: "2026-11-14T08:15:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-14"
  mode: pre-merge
  shipment_id: 214-S
  feature_id: 213-F
  evidence_head: f72525f4c6f6715652b966c97353bdb48735b7d4
  closure_status: READY
  compaction_status: pending
---

## Readiness summary

Shipment `214-S` (`213-F`: Cross-Tool Workspace Pinning Verifier, Launcher/Companion MCP Alignment, and Multi-Workspace Negative Verification) has completed all four waves (`213.004-T`, `213.001-T`, `213.002-T`, `213.003-T`), passed unfiltered Wave 4 convergence across all 50 packages, passed full local build (`go build ./cmd/backlogit`), and passed isolated CLI runtime verification (`PASS`).

Pre-merge readiness is `READY` (subject to current-HEAD whole-shipment local review, CI checks, and P-018 Copilot review before merge execution).

## Invariants and pre-deploy audits

* Level 1 workspace-local pinning does not provide an OS or container sandbox (`os_container_sandbox_deferred: true` remains explicit in `.backlogit/toolchain.yaml` and `ToolchainVerificationReport`).
* `VerifyLauncherScripts` and `AuditRepositoryToolchain` inspect `.mcp.json`, `start.ps1`, and `start.sh` read-only and reject ambient `PATH` lookups or unpinned binaries.
* `WriteToolchainManifest` and `RenderPinnedMCPConfig` reject path traversal, symlink/reparse escapes, duplicate keys, and `unsupported` or `non-conformant` classes with wrapped `ErrValidation`.
* No database schema migration or destructive state rewrite is required; `.backlogit/toolchain.yaml` is additive.

## Validator evidence

See [214-S pre-merge runtime verification](2026-11-14-214-s-pre-merge-runtime-verification.md):

* Verdict: `PASS` (`cli` command adapter in isolated `--cwd <tempDir>` workspaces plus Go race/integration suites).
* Pinned temp workspace (`mcp status --toolchain --strict --format json`): exit `0`, `valid: true`, `violations: 0`.
* Unpinned temp workspace (`mcp status --toolchain --strict --format json`): exit `1`, `valid: false`, `violations: 12`, wrapped `ErrValidation`.
* Full local build (`go build ./cmd/backlogit`), `go vet ./...`, `golangci-lint run --concurrency=1 --timeout=15m`, `gofmt -l .`, `scripts/md-lint.ps1` (`0` issues across 4162 files), `backlogit docs lint` (`0` violations), and unfiltered `go test -timeout=30m ./...` (50/50 packages) all passed.

## Risky action record

| ProposedAction | Risk | Approval and result |
| --- | --- | --- |
| Isolated `--cwd <tempDir>` CLI runtime verification | Low, read-only against repo | Approved under Plan Invariant I8; executed and temp dirs removed |
| Merge PR for shipment `214-S` | Moderate, release handoff | Pending CI, current-HEAD local review, P-018, and P-014 merge approval |

## Monitoring plan, owner, and validation window

* **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
* **Validation window**: Immediate post-merge verification on `main` plus 7-day multi-session observation window (`_Orchestrator`, `_Stage`, `_Ship`).
* **Healthy signals**:
  * `backlogit mcp status --toolchain --format json` reports expected `containment_class` per tool (`violations: 0` when operator-provisioned `.mcp.json` and `./bin/*` pins align).
  * `backlogit mcp status --format json` reports healthy `running`, `pid`, `active_sessions`, and `endpoint`.
  * `backlogit doctor` reports `0` errors and CI `CLI Reference Drift Check` / `Docline` / `P-008` checks stay green.
* **Failure signals**:
  * Unexpected `violations > 0` on a properly provisioned workspace or `ErrDaemonWorkspaceMismatch` false positives.
  * Failing `CLI Reference Drift Check`, `docs lint`, or multi-workspace containment tests on `main`.

## Rollback trigger and procedure

* **Rollback trigger**: Any CI drift failure on `main`, unexpected `AuditRepositoryToolchain` regression blocking operator workflows, or multi-workspace cross-attach failure after merge.
* **Rollback procedure**: Stop any active workspace MCP daemon in affected test/operator sessions (`backlogit mcp stop`), revert the `214-S` merge commit on a reviewed PR via `git revert -m 1 <merge_sha>`, rebuild `./bin/backlogit.exe`, and run `backlogit sync`. `.backlogit/toolchain.yaml` is additive, so rollback requires no database migration.

## Source artifact provenance

Feature `213-F` declares `source_stash_id: none` and `source_deliberation_id: none` (`.backlogit/archive/082-DL.md` is referenced as provenance only and remains untouched).

## Residual risks and deferred follow-ups

Seven out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` stash entries were captured during `214-S` (active in `.backlogit/stash.jsonl`, awaiting Stage triage):

* `9CFC88E5`: Investigate existing custody Git-subprocess timeout (`TestExternal_LiveAcceptAndVerifyUseExternalStore`)
* `0CD5F543`: Enforce non-root and directory-or-nonexistent checks for `COPILOT_HOME` / `ENGRAM_DATA_DIR` in `internal/toolchain/verify.go`
* `E231EA13`: Forward structured `ToolchainVerificationReport` `error.data` from `Execute()` under `--jsonrpc` `--strict`
* `D8EE8389`: Resolve relative `workspace_root: "."` against verified root `abs` rather than `os.Getwd()` in `verify.go` / `launcher.go`
* `040826D4`: Investigate 10-second `reconCtx` timeout in `TestU1f_ExecuteReconstitutionPreliminaryPlanReadsUnderCanonicalLocks`
* `3466D3CB`: Update `docs/ARCHITECTURE.md` dependency diagram (`cli -> toolchain`) and current implementation status
* `08A5E6F7`: Update `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` shipped-status notes

## Structured releasability and compaction

| Evidence | Requirement | State |
| --- | --- | --- |
| Runtime verification | Optional (`required: false`) | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Post-merge closure duty | `pending` |

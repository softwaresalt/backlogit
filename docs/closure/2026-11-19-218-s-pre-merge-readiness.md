---
title: "218-S pre-merge readiness"
description: "Pre-merge operational closure and releasability evidence for Shipment 218-S (Feature 217-F)."
doc_type: closure
source: docs/closure/2026-11-19-218-s-pre-merge-readiness.md
ingested_at: "2026-11-19T14:15:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-19"
  mode: pre-merge
  shipment_id: 218-S
  feature_id: 217-F
  evidence_head: c8523d6e
  closure_status: READY
  compaction_status: pending
---

## Readiness summary

Shipment `218-S` (`217-F`: Honor `.mcp.json`, configurable `tools_dir`, and `.bin` default in MCP daemon startup and toolchain verification) completed all 4 tasks (`217.001-T`..`217.004-T`) across Waves 1–3, passed the unfiltered Go test suite (`GOTOOLCHAIN=go1.26.5 go test -timeout=30m -p 1 ./...`), passed `gofmt`, `go vet`, `golangci-lint`, full local build (`go build ./cmd/backlogit`), and passed pre-merge runtime verification (`PASS`).

## Invariants to preserve

* **I1 (Pinned child-handle traversal in `pinnedMCPDaemonBinary`)**: Every binary and config candidate descends segment-by-segment through pinned child `*os.Root` directory handles (`current.Lstat` -> `rejectLink` -> `current.OpenRoot` -> `next.Stat(".")` + `os.SameFile`) and opens the regular file leaf on the innermost pinned parent handle.
* **I2 (Fail-closed refusal on invalid configured paths)**: Invalid, escaping, redirected, swapped, or missing configured targets in `.mcp.json`, `toolchain.yaml`, or `config.yaml` fail closed immediately without falling back to `.bin/`, `bin/`, or `PATH`.
* **I3 (Reserved directory exclusion)**: `.git`, `.backlog`, and `.backlogit` are excluded from valid tool directories in `pinnedMCPDaemonBinary`, `ToolchainManifest`/`verify.go`, and `WorkspaceConfig` (`Load`/`Validate`), using `core.WorkspaceRootCandidates()`.
* **I4 (Manifest-aware remediation)**: `effectiveManifestToolsDir` and `backlogitRemediationPath` preserve `./bin/...` for legacy `bin` manifests while emitting `./.bin/...` or `./<tools_dir>/...` otherwise.
* **I5 (Cross-branch ID reservation completeness)**: `scanLocalBranchArtifactIDsContext` inspects all `refs/heads` branch tips via `git for-each-ref` + `git cat-file --batch` with tree/subtree OID deduplication and `branchIDScanCache`, preserving merged-then-reverted branch reservations.

## Validator evidence

See [218-S pre-merge runtime verification](2026-11-19-218-s-pre-merge-runtime-verification.md):

* Verdict: `PASS` (`cli` adapter against `.\backlogit.exe`: Probe 1 `.bin` prewarm/status/stop, Probe 2 `custom-bin` `tools_dir` strict toolchain audit + prewarm/stop, Probe 3 120-branch `backlogit add` in `431ms`, and full Go test suite at `c8523d6e`).

## Risky action record

| ProposedAction | Risk | Approval and result |
| --- | --- | --- |
| Expand pinned `backlogit` path resolution and `tools_dir` config/verification in `internal/{cli,config,toolchain}` and batch git tree scanning in `internal/core` | Moderate (daemon execution boundary, containment, ID allocator) | Approved in `docs/plans/2026-11-19-mcp-daemon-configurable-tools-dir-plan.md` (`102-DL`); verified across full test suite and 3 CLI runtime probes |
| Refresh locked dogfood binary `.bin\backlogit.exe` | Low (host runtime binary locked by active MCP server PIDs `7816`, `25480`) | Deferred until active MCP daemon processes stop; `go build ./cmd/backlogit` succeeded for `.\backlogit.exe` |

## Monitoring plan, owner, and validation window

* **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
* **Validation window**: Immediate post-merge verification on `main` and external GSD `.bin` workspaces.
* **Healthy signals**: Zero `ErrDaemonBinaryMissing` or `ErrDaemonVersionMismatch` failures on `.bin/` and `.mcp.json`-configured workspaces; `make docs-lint` and `go test -timeout=30m ./...` green.
* **Failure signals**: Containment refusal regression on symlink/junction workspaces, `TestU6`/`TestU11`/`TestU12` failures, or `CreateArtifact` branch-scan timeouts.

## Rollback trigger and procedure

* **Rollback trigger**: Any containment regression, daemon startup failure, or ID allocation regression on `main`.
* **Rollback procedure**: Submit an operator-approved `git revert -m 1 <merge_sha>` PR; `tools_dir` fields are optional (`omitempty`) and require no state migration.

## Source artifact provenance

Feature `217-F` records `source_stash_id: 2CCDFDED` and `source_deliberation_id: 102-DL` (`docs/decisions/2026-11-19-mcp-daemon-configurable-tools-dir-deliberation.md`). Companion deferred stash entry `85442104` remains queued for Stage disposition.

## Structured releasability and compaction

| Evidence | Requirement | State |
| --- | --- | --- |
| Runtime verification | Optional (`required: false`) | Satisfied (`PASS`) |
| Monitoring plan | Recorded | Recorded |
| Rollback procedure | Recorded | Recorded |
| Owner and validation window | Recorded | Recorded (`_Ship`, immediate + post-merge) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Post-merge closure duty | `pending` |

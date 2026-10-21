---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: degraded
description: "Post-merge closure for 185-S (governed queued-shipment disposition docs, Stage-only, no activation)."
doc_type: closure
docline:
  date: 2026-10-21T07:40:00Z
  status: final
  tags:
    - closure
    - 185-S
schema_version: "1.0"
shipment_id: 185-S
source: docs/closure/185-S-185-F-post-merge-closure.md
title: "185-S closure"
---

# 185-S closure

Context: feature 185-F (tasks 185.021-T, 185.025-T, 185.026-T), PR #564, merge `be7280e2b20849c534a5484d659aa7619651e597`. Documents the governed queued-shipment disposition in `docs/workflow.md` and the CLI/MCP parity matrix (confirmed-only CLI fallback), records the `6434A4D7` role-contract drift on the `_stage`, `_orchestrator`, and `_ship` manifest entries, and verifies the capability through in-process tests. No runtime surface changed, so runtime-verification is not applicable.

## Releasability

`closure_status: READY`. Monitoring: none beyond CI. Rollback: revert PR #564. Owner: operator. Validation window: next Stage session that exercises a disposition (operator-run or Stage-run, never Ship). No conditions.

## Evidence

* Multi-persona adversarial review before the PR (reviewed HEAD `404c3c57`, report-only, three reviewers, two with shell access): P0 0, P1 0, P2 0, P3 3, outcome READY_WITH_FOLLOWUPS. The one reviewer without a shell read the source directly. The third reviewer returned no findings and no per-claim evidence.
* Copilot raised one valid finding (the confirmation checklist omitted the Stage authorization safeguards). Fixed in `6ca27869` within P-021 C1, replied on the thread, resolved via GraphQL, review re-requested. The re-review raised nothing new. `autoharness gate copilot-review` was SATISFIED before and immediately before merge. The delta `404c3c57..6ca27869` was re-reviewed locally against `_stage.agent.md` lines 54 to 66.
* CI green on `6ca27869` (8 checks, merge state CLEAN). The merge used the merge-commit strategy (P-009), authorized by the dark-mode activation record. No admin fallback was used.
* Local gates: `go build ./cmd/backlogit`, `go vet ./...`, `gofmt -l cmd internal tests`, and `go test -timeout=30m -count=1 ./...` passed. `go run ./cmd/backlogit docs lint` reports violations only in untracked `docs/scratch/` files, none in the edited files. `golangci-lint run` could not run (export-data version mismatch with the installed toolchain), so CI was authoritative.
* In-process disposition evidence (all PASS): `TestShipmentDispose_MCPSuccessReturnsAbandonedShipment`, `TestShipmentDispose_MCPErrorCodes`, `TestShipmentDispose_MCPStateRefusal`, `TestShipmentDispose_MCPWriteIndeterminate`, `TestShipmentDispose_SuccessPrintsAbandonedShipment`, `TestShipmentDispose_MissingConfirmRequiresConfirmation`, `TestShipmentDispose_ValidationFailures`, `TestShipmentDispose_StateRefusal`, `TestDisposeQueuedShipment`, `TestDisposeQueuedShipmentState`, `TestDisposeJournalRecovery`, `TestDisposeJournalUnnormalizedPreimage`, `TestShipmentDispositionProvenance`, `TestShipmentDispositionDeclarations`, `TestGovernedShipmentDispositionEdge`, `TestRegistryParity_DisposeConfirmationFailClosed`.
* `backlogit shipment ship 185-S --sha be7280e2...` returned `shipped` and archived `185.021-T`, `185.025-T`, `185.026-T`, `185-F`, and `185-S`. The archive records are committed in this closure PR after the governed ship rewrote them.
* Ship ran no queued-shipment disposition in any form against any workspace (G-B).
* Deviation recorded: the three tasks carried no harness label or exemption contract, so the baseline `git grep` RED checks in their acceptance criteria stood in for a harness.
* Deviation recorded: the first branch name was rejected by the pipeline-topology gate (`BRANCH_MISMATCH`), so the branch was renamed to a gate-expected name before any task work.

## R13 release-specific items (185-F, source plan Per-Release Closure Rule)

* Binary version: `backlogit` built from the repository at the merged tree with the local toolchain; the CLI reports commit and build date as `unknown` for this local build.
* Unchanged shipments: `156-S` and every other existing shipment record are unchanged. No live disposition happened. The diff touched only the 185 records, the hooks queue, the stash, docs, the manifest, and memory.
* Manual policy: the manual (P)+(C) 074-DL policy stays in force until AF1E5075 lands and the `154-S` attestation exists. `READY` does not relax it.
* Read-only doctor on the live workspace was not run in this session (MCP and CLI calls took minutes each under load); CI passed and no code changed.

## Compaction status (P-020)

`compaction_status: degraded`. No files were moved: retained docs and checkpoints still reference existing `docs/memory/` paths. The compact-context skill was not run to completion; this is non-blocking per P-020.

## Follow-ups

* P3 (stash `1E1F3C8F`, DEFERRED SCOPE EXPANSION): refresh the parity-matrix totals (30 parity, 57 tools) and add rows for tools added since the audit.
* P3: `golangci-lint` needs a rebuild against the current toolchain.
* P3: Stage-only enforcement is by agent contract only. Neither the core nor the MCP handler checks the caller role.

## Source artifact provenance (untouched)

Source stash `6434A4D7` and its deliberation are provenance only and were not mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated.

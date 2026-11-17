---
title: "217-S pre-merge readiness"
description: "Pre-merge operational closure and releasability evidence for Shipment 217-S (Feature 216-F / CAP-3)."
doc_type: closure
source: docs/closure/2026-11-17-217-s-pre-merge-readiness.md
ingested_at: "2026-11-17T22:55:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-17"
  mode: pre-merge
  shipment_id: 217-S
  feature_id: 216-F
  evidence_head: cc59f7b6
  closure_status: READY
  compaction_status: pending
---

## Readiness summary

Shipment `217-S` (`216-F`: Zero-Flake Concurrency and Fast Ship-Closure Engine,
CAP-3) completed all 8 tasks (`216.001-T`..`216.008-T`) across Waves 1–6,
resolved all in-scope review findings (`cc59f7b6`), passed all 50 Go packages,
passed `gofmt`, `go vet`, `golangci-lint`, full local build (`go build ./cmd/backlogit`),
and passed pre-merge runtime verification (`PASS`).

## Invariants to preserve

* **Canonical lock order (`I1`)**: `AssociateCommit` and `LinkCommit` acquire `lifecycle-global -> C -> B` for shipments and shipment members and fail closed with `*ShipmentLifecycleLockOrderError` on preheld `C` without verified `lifecycle-global` ownership.
* **Ship dual-gate & containment (`I2`)**: `findArtifactWithPath` runs `ensureArtifactLookupContained` before parsing direct `{id}.md` candidates; `ShipShipment` retains both `gateShipmentCompletion` passes and Markdown reloads.
* **Memoization safety (`I3`)**: `isAncestor` enforces `--no-replace-objects`, `--`, symlink/junction root resolution (`resolveAncestryRootPath`), and `.git/shallow` / `.git/info/grafts` absence both before and after `cmd.Run()`; `custodyBlobDigest` validates `isGitObjectName(oid)` and canonicalizes `root`.
* **SQLite WAL Page 1 & Ordinal 17 (`I4`)**: `db.Open` orders `_pragma=busy_timeout(30000)` first, initializes Page 1 WAL header `[2, 2]` with bounded concurrent retry, and `custExecNewRepo` retains `PRAGMA synchronous = FULL`.

## Validator evidence

See [217-S pre-merge runtime verification](2026-11-17-217-s-pre-merge-runtime-verification.md):

* Verdict: `PASS` (`cli` adapter against `.\bin\backlogit.exe`, isolated `init`/`add`/`update --commit`/`sync` WAL Page 1 `[2, 2]` probe, `docs lint`, and 50/50 Go packages at `cc59f7b6`).

## Risky action record

| ProposedAction | Risk | Approval and result |
| --- | --- | --- |
| Enforce lock ordering, WAL Page 1 init, bounded `custodyGit`, and lookup/ancestry caching in `internal/{core,db,events}` | Moderate (concurrency, SQLite pool, custody git) | Approved in `docs/plans/2026-11-15-cap3-zero-flake-concurrency-ship-closure-plan.md` (`096-DL`..`101-DL`); verified across all 50 Go packages |
| Merge PR for shipment `217-S` | Moderate (release handoff) | Pending final current-HEAD local review, CI, P-018 Copilot review, and P-014 merge approval |

## Monitoring plan, owner, and validation window

* **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
* **Validation window**: Immediate post-merge verification on `main` plus 7-day observation window.
* **Healthy signals**: All 50 Go packages, `golangci-lint`, `go vet`, and `go build ./cmd/backlogit` pass on `main`; `backlogit_ship_shipment` completes without MCP `-32001` timeouts; zero `no such table: commit_links` or lock-order inversion errors.
* **Failure signals**: Any lock-order regression, custody verification regression, WAL initialization failure, or shipment gate discrepancy.

## Rollback trigger and procedure

* **Rollback trigger**: Any regression in commit-writer lock ordering, `ShipShipment` closure gate integrity, SQLite WAL pool initialization, or custody verification on `main`.
* **Rollback procedure**: Submit an operator-approved `git revert -m 1 <merge_sha>` PR (preserving `.backlogit/archive/` history); no database schema migration is involved.

## Source artifact provenance

Feature `216-F` and tasks `216.001-T`..`216.008-T` record source stash IDs `703835DC`, `76553D8D`, `F448AA52`, `9CFC88E5`, `B25BF131`, and `63909DAE`, and deliberations `096-DL`..`101-DL` (`docs/decisions/2026-11-15-cap3-zero-flake-concurrency-ship-closure-deliberation.md`). Source stash and deliberation artifacts are retained untouched as provenance only.

## Tracked residual risks and P-021 C2 deferrals

* Unnamed `df4a2df9` `internal/core` panic (`63909DAE` / `101-DL`) remains explicitly unresolved as a tracked diagnostic risk (unproven origin).
* Out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` stash entries captured in `.backlogit/stash.jsonl` for Stage triage: `451486E7`, `FDFC52FF`, `DA6809A0`, and `D1CA631D`.

## Structured releasability and compaction

| Evidence | Requirement | State |
| --- | --- | --- |
| Runtime verification | Optional (`required: false`) | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Post-merge closure duty | `pending` |

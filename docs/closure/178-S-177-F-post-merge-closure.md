---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 178-S and covering feature 177-F: governed 30m full-suite budget pinned in the pre-push Test gates and the Makefile test recipe, with manifest checksum reconciliation."
doc_type: closure
schema_version: "1.0"
shipment_id: 178-S
feature_id: 177-F
source: docs/closure/178-S-177-F-post-merge-closure.md
title: "178-S / 177-F Post-Merge Closure"
---

# 178-S / 177-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `178-S` (073-DL ensemble E2, stash `CB8887AF`) |
| Covering feature | `177-F` |
| Tasks | `177.001-T`, `177.002-T`, `177.003-T`, `177.004-T` |
| Implementation PR | [#600](https://github.com/softwaresalt/backlogit/pull/600) (merge commit) |
| Merge commit | `8581659120391eee00fe90d4e60e4d77b72a39dc` |
| Reviewed implementation HEAD | `7946082a462c88498f5d4a5b0c54ccb1ce79453a` |
| Closure branch | `post-merge/178-s-governed-test-budget-e2` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `target: all` invoked) |

## Delivered

* `177.001-T` (RED deliverable): `tests/integration/governed_budget_local_gates_test.go`, `TestGovernedFullSuiteBudget_LocalGates`. Observed RED on assertions naming both pre-push scripts and the Makefile, with `go vet ./tests/integration` passing. Script lines are quoted, so the test uses its own detector rather than the workflow one.
* `177.002-T` (green maker): both pre-push Test gates run `go test -timeout=30m ./...`; the Build gate `-run=^$` is unchanged.
* `177.003-T` (green maker): `make test` runs `go test -race -timeout=30m -coverprofile=coverage.out ./...`; `make.ps1` is untouched.
* `177.004-T` (harness-required): `TestU177_004_ManifestPrePushChecksumReconciliation` in `tests/integration/harness_manifest_pre_push_reconciliation_test.go` was observed RED, then the two manifest entries were rotated to the LF-normalized checksums of the new scripts and the 073-DL E2 sentence was appended to each `drift_reason`. Both entries keep `drift_allowed: true` and the PR #400 reasons.

## Gates

* Local: `go build ./cmd/backlogit` and `go vet ./...` passed. Unfiltered `go test -timeout=30m ./...` passed except an `internal/events` stale-lock timing test that failed once under load and passed in isolation; this change does not touch it.
* Review: local code review `READY` at the reviewed HEAD (P0 0, P1 0, P2 0, P3 0).
* CI: every required check passed for the merged HEAD, including `test`, Docline frontmatter gate, and Windows handle/lock tests.
* P-018: `autoharness gate copilot-review 600 --enforcement auto` returned `SATISFIED`, including the last-mile re-run.
* Merge: merge commit (P-009), `MERGE_CONFIRMED`, and local `main` equal to `origin/main` at `8581659`.
* Dark mode events: `LOCAL_REVIEW_READY`, `DARK_MODE_MERGE_AUTHORIZED`; no admin fallback was used.

## Shipment closure

* Closed with `backlogit shipment ship 178-S --sha 8581659120391eee00fe90d4e60e4d77b72a39dc` (CLI; the MCP claim call timed out, so the claim and ship used the registry CLI fallback after Served-Root Attestation).
* Archived IDs: `177.001-T`, `177.002-T`, `177.003-T`, `177.004-T`, `177-F`, `178-S`; no returned IDs.

## Deviations and residual risk

* Local `golangci-lint` cannot import the installed toolchain export data (environment limitation); CI is authoritative for lint.
* The post-claim topology gate first reported `CLAIM_NOT_OBSERVED` because the MCP claim call timed out before the status change was visible; the claim completed and the gate then passed.
* Pre-closure baseline: the worktree was clean before closure writes.
* Pre-existing note: `make.ps1` Windows `-race` budget remains measurement-gated under stash `5A1C4D3F`.
* No follow-up stash entries were created. Provenance stash `CB8887AF` was not touched.

## Compaction and knowledge graduation

* compact-context (`target: all`): nine ship session checkpoints (141-S to 194-S) were consolidated into `docs/memory/compacted/2026-10-20-141s-to-194s-ship-sessions-compacted.md`, with originals moved to `docs/archive/memory/2026-10-20/`.
* compound-refresh: no `docs/compound` entry is invalidated by this shipment.
* No documentation graduation was required beyond this record.

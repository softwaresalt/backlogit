---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 180-S and covering feature 179-F: governed 30m full-suite budget pinned in agent and skill procedures, repo-wide test-run prohibitions reworded, manifest checksums reconciled."
doc_type: closure
schema_version: "1.0"
shipment_id: 180-S
feature_id: 179-F
source: docs/closure/180-S-179-F-post-merge-closure.md
title: "180-S / 179-F Post-Merge Closure"
---

# 180-S / 179-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `180-S` (073-DL ensemble E3, stash `CB8887AF`) |
| Covering feature | `179-F` |
| Tasks | `179.001-T`, `179.002-T`, `179.003-T`, `179.004-T`, `179.005-T` |
| Implementation PR | [#605](https://github.com/softwaresalt/backlogit/pull/605) (merge commit) |
| Merge commit | `fd6ac3dd47a8ad426c5a0512bd4ff6e2b532e136` |
| Reviewed implementation HEAD | `61c3e43c3d905c3b59cfac6fbb8988a362a0471f` |
| Closure branch | `post-merge/180-s-governed-test-budget-e3` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `target: all` invoked; no candidates met the thresholds) |

## Delivered

* `179.001-T` (RED deliverable): `tests/integration/governed_budget_procedures_test.go`, `TestGovernedFullSuiteBudget_Procedures`. Landed RED on assertions for every anchor, with `go vet ./tests/integration` passing; the prohibition shape is matched structurally, not by a literal allowlist.
* `179.002-T`: build-feature non-wave path and the four fix-ci anchors now name `go test -timeout=30m ./...`.
* `179.003-T`: github-pr-automation fix loop 1.4 and go-engineer now name the governed command; plan deviation D6 closes.
* `179.004-T`: the three prohibitions (build-feature, two in harness-architect) forbid any repo-wide test-running `go test ... ./...` form and keep `-run=^$` allowed.
* `179.005-T`: manifest checksums for github-pr-automation, go-engineer, and harness-architect rotated to LF-normalized SHA-256 values with 073-DL notes.

## Gates

* Local: `go build ./cmd/backlogit`, `go vet ./...` passed; `TestGovernedFullSuiteBudget_` ensemble passed. The unfiltered full suite showed only `TestU175_*_LintFix` failures from a local golangci-lint export-data/toolchain mismatch, unrelated to this change.
* Review: local review `READY` (P0 0, P1 0, P2 0, P3 0).
* CI: every required check passed for the merged HEAD.
* P-018: `autoharness gate copilot-review 605 --enforcement auto` returned `SATISFIED`, including the last-mile re-run.
* Merge: merge commit (P-009), `MERGE_CONFIRMED`, local `main` equal to `origin/main` at `fd6ac3d`. No admin fallback was used.

## Shipment closure

* Closed with `backlogit shipment ship 180-S --sha fd6ac3dd47a8ad426c5a0512bd4ff6e2b532e136`.
* Archived IDs: `179.001-T` through `179.005-T`, `179-F`, `180-S`; `180-S` is `archived` with `archived_status: shipped`.

## Deviations and residual risk

* The MCP claim call and several task moves timed out; each still landed. The post-claim gate reported `CLAIM_NOT_OBSERVED` once and passed on re-verification.
* Local `golangci-lint` cannot import the toolchain export data; CI is authoritative for lint.
* The ignored `.copilot/` tree (~120k files) makes cached `go test ./...` runs extremely slow locally; `-count=1` avoids the cache hashing.
* No follow-up stash entries were created. Provenance stash `CB8887AF` was not touched.

## Compaction and knowledge graduation

* compact-context (`target: all`): `docs/memory/` holds 33 files, below the 40-file threshold, so no candidates were compacted.
* compound-refresh: no `docs/compound` entry is invalidated by this shipment.

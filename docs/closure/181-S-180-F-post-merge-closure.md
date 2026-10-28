---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 181-S and covering feature 180-F: governed 30m full-suite budget pinned in the constitution (MINOR 1.1.0) and top-level guidance, manifest checksums reconciled."
doc_type: closure
schema_version: "1.0"
shipment_id: 181-S
feature_id: 180-F
source: docs/closure/181-S-180-F-post-merge-closure.md
title: "181-S / 180-F Post-Merge Closure"
---

# 181-S / 180-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `181-S` (073-DL ensemble E4, stash `CB8887AF`) |
| Covering feature | `180-F` |
| Tasks | `180.001-T`, `180.002-T`, `180.003-T`, `180.004-T`, `180.005-T`, `180.006-T` |
| Implementation PR | [#607](https://github.com/softwaresalt/backlogit/pull/607) (merge commit) |
| Merge commit | `a72030a023d81556f00e2b0106dcc44a8dc7da0a` |
| Reviewed implementation HEAD | `a75624a3a99b07d2fb0a48b6e78a1889a5dc3cec` (PR head `5e0f6465` adds metadata and one behavior-neutral lint literal fix) |
| Closure branch | `post-merge/181-s-governed-test-budget-e4` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `target: all` invoked; 35 memory files, below the 40-file threshold, no candidates) |

## Delivered

* `180.001-T` (RED deliverable): `tests/integration/governed_budget_guidance_test.go`, `TestGovernedFullSuiteBudget_Guidance`, 12 anchored subtests, all RED on assertions before the migration.
* `180.002-T`: constitution Principle II, Technical Constraints and Quality Gates name `go test -timeout=30m ./...`; MINOR bump 1.0.0 to 1.1.0 with rationale and sync impact report (plan deviation D4 closed).
* `180.003-T`: AGENTS.md and README.md migrated.
* `180.004-T`: copilot-instructions.md and go.instructions.md (including the coverage variant) migrated.
* `180.005-T`: copilot-review-instructions.md validation bar migrated.
* `180.006-T`: manifest checksums for the four managed files rotated (LF-normalized); `TestU180006_ManifestChecksumsCurrent` added.

## Gates

* Local: `go build ./cmd/backlogit`, `go vet ./...` passed; the `TestGovernedFullSuiteBudget_` and `TestU180006_` ensemble passed. The full suite showed only intermittent `TestU175_*_LintFix` failures under local package-parallel load (each passes in isolation; local golangci-lint toolchain mismatch).
* CI on PR #607: all required checks passed at HEAD `5e0f6465` after an S1007 lint fix.
* P-018 Copilot-review gate: SATISFIED before merge. Merge used the merge-commit strategy (P-009).

## Provenance and follow-ups

* Source stash `CB8887AF` and deliberation 073-DL are untouched provenance; Stage owns any later triage.
* Follow-up (not captured as a stash): `.github/copilot-instructions.md` has a pre-existing doubled code fence under `## Project Structure` that hides `## Commands`; the E4 test anchors around it.
* Runtime verification and monitoring: not applicable (documentation and test-only change). Rollback: revert the merge commit.

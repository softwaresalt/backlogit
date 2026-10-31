---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 209-S and covering feature 208-F: adopt rename propagation, history-safe ID allocation, stash correct --force, and doctor shipment manifest checks."
doc_type: closure
schema_version: "1.0"
shipment_id: 209-S
feature_id: 208-F
source: docs/closure/209-S-208-F-post-merge-closure.md
title: "209-S / 208-F Post-Merge Closure"
---

# 209-S / 208-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `209-S` |
| Covering feature | `208-F` |
| Tasks | `208.001-T` through `208.003-T` (3 tasks, 3 waves, all test-first) |
| Implementation PR | [#635](https://github.com/softwaresalt/backlogit/pull/635) (merge commit) |
| Merge commit | `11fc9edf65ac3a47f5bfd9b3f680f781c2dd6ae3` |
| Reviewed implementation HEAD | `c5ca624678b7cae58ab5304263d5451522a6aac6` |
| Closure branch | `post-merge/209-s-adopt-propagation-id-safety-doctor-manifest` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `compact-context` invoked; nothing eligible to archive) |

## Delivered

* `AdoptItem` rewrites shipment manifest members and the stash archive (`harvested_artifact_id`, correlated provenance-correction successor), with `--keep-id` / `keep_id` to retain the ID
* ID allocation skips every ID retained in SQLite or on disk history (queue, archive, item logs including `adopted` old IDs, stash archive, correction ledger)
* `stash correct --force` operator repoint with event-first audit, rollback, crash-resume, and rehydration refresh
* Doctor shipment manifest findings: dangling member, duplicate membership, split feature, forward dependency, unsequenced queued shipment (advisory)
* CLI/MCP/registry parity and regenerated CLI reference

## Gates

| Gate | Result |
|---|---|
| Local review (current HEAD) | `READY`, P0-P3 all 0 |
| CI (`test`, windows handle/lock, drift, markdown) | pass for HEAD `c5ca6246` |
| P-018 Copilot-review gate | `SATISFIED`; 9 findings across rounds fixed or deferred and all threads resolved |
| Local lint | `golangci-lint` unusable locally (export-data mismatch); CI lint green |

Review-driven fixes: unused symbols, unique temp file, rollback defer capture, slug standalone ID history parsing, forced-correction crash resume, doctor archive detection relative to storage root, transactional stash upsert+link, shared active-stash lock taken outermost (stash, lifecycle-global, item-log, artifact), cross-reference rollback on adopt failure.

## Follow-ups

* Stash `CF11DA5C` (deferred scope expansion): cache-aware ID-history high-water marks for batch `CreateArtifact`; deferred per P-021 C1
* Pre-existing drift in `docs/cli-reference/` shipment and update pages (unrelated generator output) left untouched
* Provenance IDs untouched: `7BE3AF92` (cross-branch ID reservation)

## Rollback and monitoring

Revert merge commit `11fc9edf`; no data migration. Doctor now surfaces shipment manifest findings; monitor `backlogit doctor` output for new error-severity findings.

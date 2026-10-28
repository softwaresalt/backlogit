---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 179-S and covering feature 178-F: governed 30m full-suite budget pinned in the harness render inputs, workspace profile, and code-review focus instruction, with manifest checksum reconciliation."
doc_type: closure
schema_version: "1.0"
shipment_id: 179-S
feature_id: 178-F
source: docs/closure/179-S-178-F-post-merge-closure.md
title: "179-S / 178-F Post-Merge Closure"
---

# 179-S / 178-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `179-S` (073-DL ensemble E5, stash `CB8887AF`) |
| Covering feature | `178-F` |
| Tasks | `178.001-T`, `178.002-T`, `178.003-T`, `178.004-T` |
| Implementation PR | [#603](https://github.com/softwaresalt/backlogit/pull/603) (merge commit) |
| Merge commit | `12ed94d771c9e9240f0b1d2136a96236db67eccf` |
| Reviewed implementation HEAD | `fd71d0dad15f7f6d958eafb52ad0d235d484dad6` |
| Closure branch | `post-merge/179-s-governed-test-budget-e5` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `target: all` invoked; no candidates met the thresholds) |

## Delivered

* `178.001-T` (RED deliverable): `tests/integration/governed_budget_render_inputs_test.go`, `TestGovernedFullSuiteBudget_RenderInputs`. Observed RED on assertions for all six anchors, with `go vet ./tests/integration` passing. Manifest values are parsed as YAML keys.
* `178.002-T` (green maker): `TEST_COMMAND`, the `HARNESS_ENFORCED_SUMMARY` bullet, and the `TESTING_RULES` bullet in `.autoharness/harness-manifest.yaml`, plus `test.command` and `ci.quality_gates` in `.autoharness/workspace-profile.yaml`, now name `go test -timeout=30m ./...`.
* `178.003-T` (green maker): the matching bullet in `.github/instructions/copilot-code-review.instructions.md`.
* `178.004-T`: the manifest checksum for that instruction was rotated to the LF-normalized SHA-256 `0171d541f30cf41f2424cdce24825428a58ed920c0def427406be96d44b5ea71` with a 073-DL note. The pre-push `drift_allowed` reasons still hold because the scripts are untouched.

## Gates

* Local: `go build ./cmd/backlogit`, `go vet ./...`, and `gofmt -l .` passed. The `TestGovernedFullSuiteBudget_` ensemble passed. The unfiltered `go test -timeout=30m ./...` showed load-induced `TestU175_*_LintFix` failures and one EOL failure in the new test file; the EOL was fixed and every failing test passed on rerun.
* Review: local review `READY` at the reviewed HEAD (P0 0, P1 0, P2 0, P3 0).
* CI: every required check passed for the merged HEAD.
* P-018: `autoharness gate copilot-review 603 --enforcement auto` returned `SATISFIED`, including the last-mile re-run.
* Merge: merge commit (P-009), `MERGE_CONFIRMED`, and local `main` equal to `origin/main` at `12ed94d`.
* Dark mode events: `LOCAL_REVIEW_READY`, `DARK_MODE_MERGE_AUTHORIZED`; no admin fallback was used.

## Shipment closure

* Closed with `backlogit shipment ship 179-S --sha 12ed94d771c9e9240f0b1d2136a96236db67eccf`.
* Archived IDs: `178.001-T`, `178.002-T`, `178.003-T`, `178.004-T`, `178-F`, `179-S`; `179-S` is `archived` with `archived_status: shipped`.

## Deviations and residual risk

* The MCP claim call timed out; the claim still landed, the post-claim gate reported `CLAIM_NOT_OBSERVED` once, and it passed on re-verification.
* Local `golangci-lint` cannot import the installed toolchain export data (environment limitation); CI is authoritative for lint.
* The task moves for `178.003-T` and `178.004-T` timed out over MCP while the full suite ran; both landed.
* Accepted drift window: E3/E4 consumers (blocked by this shipment) still render the budget-less form until migrated.
* No follow-up stash entries were created. Provenance stash `CB8887AF` was not touched.

## Compaction and knowledge graduation

* compact-context (`target: all`): `docs/memory/` holds 31 files, below the 40-file threshold, so no candidates were compacted.
* compound-refresh: no `docs/compound` entry is invalidated by this shipment.
* No documentation graduation was required beyond this record.

---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 147-S and covering feature 165-F: harness and documentation hygiene."
doc_type: closure
schema_version: "1.0"
shipment_id: 147-S
feature_id: 165-F
source: docs/closure/147-S-165-F-post-merge-closure.md
title: "147-S / 165-F Post-Merge Closure"
---

# 147-S / 165-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `147-S` |
| Covering feature | `165-F` |
| Tasks | `165.001-T`, `165.003-T` (2 tasks, 1 wave, both `harness-exempt` docs-only) |
| Implementation PR | [#618](https://github.com/softwaresalt/backlogit/pull/618) (merge commit) |
| Merge commit | `bb0756fabdad7c24e3785fb64d6e0f09cf0f0c71` |
| Reviewed implementation HEAD | `db30339b63deefff243d8a1a3f7b2af99d5ee3fa` |
| Closure branch | `post-merge/165-f-harness-and-documentation-hygiene` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `compact-context` invoked) |

## Delivered

* `.github/instructions/commit-message.instructions.md`: scope vocabulary now includes `events`, `docline`, `memory`, `compound`, `cli-reference`; body guidance drops the 300-byte and 0-5 item limits and allows multi-bullet or structured bodies for complex changes (`165.001-T`)
* `docs/decisions/2026-07-13-plugin-bundle-structural-verification-decision.md`: new `## Scope boundary: P-002 harness-satisfied contract` section recording that the P-002 / P-002.1-P-002.5 contract is intentionally not propagated to `plugin/` (`101-F`, `633818E1`) (`165.003-T`)
* No `plugin/` content and no production code changed
* `165.002-T` (cross-repo upstream template) was descoped from this shipment before execution

## Gates

| Gate | Result |
|---|---|
| Exempt gate | Both probes failed before the edit; `EXEMPT_VERIFY_OK:165.001-T` and `EXEMPT_VERIFY_OK:165.003-T` afterwards |
| Local review (P-014) | `READY` (P0 0, P1 0, P2 0, P3 0); full build not applicable (docs-only) |
| Copilot review (P-018) | `SATISFIED` for the merged HEAD |
| CI | All checks passed except `pipeline-topology (ambient)` (see note); not a required check |
| Merge strategy (P-009) | Merge commit |
| Governed closure | `backlogit_ship_shipment` archived `147-S`, `165-F`, `165.001-T`, `165.003-T` with the merge SHA; the client call timed out and completion was confirmed from the archive file and the `shipped` and `archived` log events |

## Runtime verification and releasability

Releasability: `READY`. The change is documentation-only and touches no runtime surface, so runtime verification is not applicable.

* Monitoring: the docs lint and Markdown lint CI jobs cover the edited files
* Rollback: revert the merge commit
* Owner: backlogit maintainers
* Validation window: none required

## Compaction (P-020)

`compact-context` thresholds were exceeded (file count and size). Of the 38 memory files older than 14 days, 26 are cited by path from durable records and the remaining 12 are already compacted summaries or session memos, so nothing was moved and no reference was broken.

## Environment notes

* CI `pipeline-topology (ambient)` failed on the pull request with `PREDECESSOR_NOT_SHIPPED: predecessor 146-S` because CI pins `autoharness==1.5.0`, which derives the numeric predecessor `146-S` (archived as `queued`) instead of the declared `154-S` dependency that the newer local gate honors. This is the known numeric-predecessor derivation defect (`A592FC1C`); the check is not branch-protected.
* The local engram daemon timed out (`ENGRAM_DEGRADED`).
* Seven untracked, locally excluded `docs/scratch` files lack frontmatter and fail the repo-wide docs lint locally; they were moved aside only while the `165.001-T` probe ran and then restored.

## Follow-ups

* Consider raising the CI `autoharness` pin so `pipeline-topology (ambient)` honors explicit shipment dependencies (not stashed here; capture is an operator or Stage decision)

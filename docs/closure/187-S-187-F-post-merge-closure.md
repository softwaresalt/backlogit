---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: done
conditions:
  - id: feature-pr-531-merged
    description: "PR #531 (187-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "Merge commit 04e91160e822e17cb189ad278e1df5db87f34428 (P-009 merge commit)."
  - id: governed-closure-archived
    description: "Shipment 187-S, feature 187-F and tasks 187.001-T..187.004-T are archived with merge provenance; post-mode checks pass."
    satisfied: true
    evidence: ".backlogit/reconcile/187-S-post-20261018T0655.md PROCEED; 187-S archived_status shipped; item-level commit association 04e91160e822e17cb189ad278e1df5db87f34428 recorded on 187-S, 187-F and 187.001-T..187.004-T via backlogit_track_commit and verified in .backlogit/archive/."
description: "187-S (187-F readiness A1 predicate and predecessor sentinel) post-merge closure for PR #531, merge 04e91160e822e17cb189ad278e1df5db87f34428. READY_WITH_CONDITIONS: additive internal API, not yet consumed by queue or claim."
doc_type: closure
docline:
  date: 2026-10-18T06:55:00Z
  status: final
  tags:
    - operational-closure
    - post-merge
    - 187-S
    - 187-F
feature_id: 187-F
schema_version: "1.0"
shipment_id: 187-S
source: docs/closure/187-S-187-F-post-merge-closure.md
title: "187-S post-merge closure"
---

# 187-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `187-S` |
| Feature | `187-F` |
| Implementation PR | [#531](https://github.com/softwaresalt/backlogit/pull/531) |
| Merge commit | `04e91160e822e17cb189ad278e1df5db87f34428` (merge commit, P-009) |
| Closure branch | `post-merge/187-readiness-a1-predicate` |
| Closure status | `READY_WITH_CONDITIONS` |
| compaction status | `done` |

## Summary

Adds `core.ShipmentPredecessorShipped(ctx, ws, id) (bool, string, error)` and the `ErrShipmentPredecessorNotShipped` sentinel. The predicate reads Markdown provenance through `findArtifact` (never the index) and accepts only a live `shipped` shipment or an `archived` shipment with `archived_status: shipped`. Missing or unparsable provenance returns `provenance_missing`, a non-shipment returns `not_shipment`, every other state returns `not_shipped:<status>` (`not_shipped:archived/<archived_status|unknown>` for archived), and a cancelled context returns an error. `status_taxonomy.go` is unchanged.

## R13 closure items

* Binary version: unchanged release line; the predicate is an unexported-consumer internal API with no CLI or MCP surface.
* The guard covers (P) only and is not wired into queue filtering or `ClaimShipment`; those land in later A-U5..A-U8 shipments.
* `156-S` and every other existing shipment are unchanged.
* The manual (P)+(C) 074-DL policy stays in force until `AF1E5075` lands; the `154-S` attestation exists (`CONDITION_B_ATTESTED`, 2026-10-06).

## Gates

* Local multi-persona adversarial review (3 reviewers): `READY_WITH_FOLLOWUPS`, P0 0, P1 0, P2 1, P3 4. P3-3 refuted by experiment; P3-2 partly addressed.
* Copilot P-018 gate: PASS (review completed, no comments).
* Required CI green: `test`, Markdown lint, Docline gate, CLI Reference Drift, Windows handle/lock tests.
* Non-required `pipeline-topology (ambient)` failed (known predecessor-inference defect, stash `95379DBA` / task `183.004-T`). Disclosure only.

## Runtime verification

Not applicable: no runtime surface changed (internal Go API and tests only).

## Monitoring, rollback and owner

* Monitoring: none needed; no production consumer yet.
* Rollback: revert merge `04e91160`; nothing depends on the predicate.
* Owner: backlogit maintainers. Validation window: until the first consumer (A-U6) lands.

## Follow-ups

* Stash `1FE847BC`: make shared `findArtifact` honor context and detect duplicate IDs; consider exporting reason-code constants (DEFERRED SCOPE EXPANSION, P-021 C2).
* Provenance recorded, untouched: source stash `6434A4D7` stays on `184-F`.

## Compaction (P-020)

compact-context ran with `target: memory`, bounded: 21 memory files compacted into 4 summaries under `docs/memory/compacted/`, originals moved to `docs/archive/memory/`. `compound-refresh` was not needed: no existing learning was superseded.

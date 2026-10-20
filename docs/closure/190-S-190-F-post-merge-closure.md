---
chunk_strategy: h1-h2-h3
# gate-required: closure_status and compaction_status MUST remain at the top level
# of this frontmatter (pipeline-topology gate reads them). Do NOT run
# `backlogit docs migrate --apply` against this file.
closure_status: READY
compaction_status: degraded
description: "Post-merge closure for 190-S / 190-F generic-path queued-abandon guard and governed edge (PR #548)."
doc_type: closure
schema_version: "1.0"
source: docs/closure/190-S-190-F-post-merge-closure.md
shipment_id: 190-S
title: "190-S / 190-F Post-Merge Closure"
---

# 190-S / 190-F Post-Merge Closure

* Shipment: 190-S (190-F, 190.001-T through 190.005-T), shipped via governed `backlogit shipment ship --sha`; every explicit member and the shipment record archived with `archived_status: shipped` semantics for the shipment; no archive deletions (P-007 clean).
* PR: #548, merge commit `1d75be28298c2f1da5a1ac7eed131fd7715b3e22` (merge-commit strategy, P-009). Binary: built from `go build ./cmd/backlogit` at merge.
* Behavior: generic shipment write paths (`MoveShipmentStatus` without marker, `UpdateArtifact`, `UpdateArtifactWithGate`, `BulkUpdateStatus`) refuse `queued` to `abandoned`. The edge opens only under the unexported governed-disposition context marker with non-empty Reason/By/AuthorizationRef, persisting status and `disposition_*`/`superseded_by` in one write. `isValidShipmentTransition` is unchanged. The marker is not set by any production caller yet (interim state I-2). The guard covers (P) only. 156-S and every other existing shipment are unchanged. The manual (P)+(C) 074-DL policy stays in force until AF1E5075 lands; the 154-S attestation exists (see the 188-S closure).
* B-U4 audit: no production caller moves a queued shipment to abandoned and no fixture needed a change; recorded as a no-op.
* Local review: READY_WITH_FOLLOWUPS at 997c51ae (P0 0, P1 0, P2 0, P3 3, all fixed in b6325aca). Two shell-capable reviewer personas (bypass and correctness/tests) ran the targeted tests, `go vet` and the broader suite; an advisory three-reviewer pass had no shell and produced no blocking finding. CI all green; Copilot review of #548: zero threads, copilot-review gate SATISFIED.
* Local note: `golangci-lint` could not run locally (toolchain export-data mismatch) and `make docs-lint`/`md-lint` do not run on this Windows host; CI lint, docs and markdown gates passed.
* Follow-up stash (P-021 C2): 9CE8D292 (write-boundary guard against `disposition_*` keys on non-abandoned shipments; AC3 of 190.005-T holds for the status edge only).
* Source artifacts untouched: stash 6434A4D7 and its provenance; 185-F and 185-S.
* Next in the dark-mode order: 191-S (its predecessor 190-S is now shipped).

## Releasability

READY. Internal core guard with no runtime surface; rollback is a revert of the merge commit; owner: Ship. No runtime verification was required.

## Compaction (P-020)

Degraded. compact-context was deliberately not run to completion this session: its candidate selection would move `docs/memory/` notes that retained decision, plan and checkpoint documents still reference (the same hazard that forced a revert in the 189-S closure), and the operator constraint forbids that. No files were moved. A later session should run compact-context after repointing those references.

---
chunk_strategy: h1-h2-h3
# Keep both gate fields at the top-level frontmatter; pipeline-topology reads them via fm.get().
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
description: "Post-merge closure for shipment 154-S and covering feature 173-F; successor routing remains gated by external scheduler marker-consumption attestation."
doc_type: closure
schema_version: "1.0"
shipment_id: 154-S
feature_id: 173-F
source: docs/closure/2026-09-30-154-s-runtime-verification.md
title: "154-S / 173-F Post-Merge Closure"
---

# 154-S / 173-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `154-S` — Shipment-claim scheduler-baseline marker (enabling precondition) |
| Covering feature | `173-F` — Shipment-claim scheduler-baseline marker (enabling precondition) |
| Implementation PR | [#466](https://github.com/softwaresalt/backlogit/pull/466) |
| Merge commit | `6d233d21162a072ddbdfecb52ec62a8fb8a63793` |
| Reviewed implementation HEAD | `c43689cf` |
| Closure branch | `post-merge/154-s-closure` |
| Closure PR | Pending creation |
| Closure status | `READY_WITH_CONDITIONS` |
| Context compaction | `degraded` — P-020 `target: all` was invoked; one completed memory record was compacted, but the full candidate set was preserved for a later verified pass. See `2026-09-30-154-s-compaction-report.md`. |

## Shipment reconciliation

- Fresh pre-close report:
  `.backlogit/reconcile/154-S-pre-20260930T154233Z.md`.
  `PROCEED`: all eight explicit members were uniquely found in the archive
  with `status: done`; shipment `154-S` was active and unblocked.
- Governed `backlogit_ship_shipment` was invoked with merge SHA
  `6d233d21162a072ddbdfecb52ec62a8fb8a63793`. The MCP request timed out
  after the shipment status and commit events persisted. Ship did not retry
  the mutation. After a subsequent registered CLI workspace-open with
  automatic shipment-operation recovery enabled, a read-only reinspection
  observed the completed archive transition; no second ship mutation was sent.
- Fresh post-close report:
  `.backlogit/reconcile/154-S-post-20260930T155230Z.md`.
  `CLOSED`: the eight explicit members and shipment control record each have
  one provenance-bearing archive record, no queue copy, and commit traceability
  to the requested SHA.
- Stage reconciled the explicit feature member `173-F` from `active` to
  `done`; backlogit relocated `.backlogit/queue/173-F.md` to
  `.backlogit/archive/173-F.md` and recorded hook event sequence `3352`.
  This occurred before the fresh pre-close report and remains within the
  explicit manifest. Ship did not move the feature status.
- P-007 archive check: `git status -- .backlogit/archive/` contained no
  deletions. The post-ship shipped-event audit no longer reports a 154-S
  residue. It reports 18 advisory legacy `missing_shipped_event` findings
  for unrelated shipment IDs; no unrelated record was changed and no event
  was synthesized.
- The `shipment-reconcile` harness file lock was not acquired, per the
  operator's single-agent/single-worktree guidance. No `.lock` file was read,
  created, modified, or removed.

## Shipped scope and provenance

The exact flat manifest was `173-F`, `173.006-T`, `173.007-T`, `173.008-T`,
`173.009-T`, `173.001-T`, `173.003-T`, and `173.005-T`; no descendants or
unlisted artifacts were inferred. The seven task records were already
pre-archived as `done`; the governed close associated the merge SHA with
their archive records. The explicit feature and shipment control record were
archived by the close.

Source-artifact provenance for `173-F`:

- `source_stash_id`: `CC0EBB59` (provenance only; untouched)
- `source_deliberation_id`: `none`

## Runtime verification and operational readiness

See [`2026-09-30-154-s-runtime-verification.md`](2026-09-30-154-s-runtime-verification.md)
for the structured validator evidence. The local build and CLI/MCP read probes
passed. The workspace profile has no configured runtime-validator surfaces
and marks runtime validation optional. These local probes do **not** establish
that the external autoharness scheduler consumed a 154-S marker; the archived
member response did not expose a `scheduler_baseline_claim` field.

### Invariants to preserve

1. An item is claim-activated only when it is `active`, its
   `scheduler_baseline_claim` equals the currently active shipment ID, and
   its ID is present in that shipment's manifest.
2. Missing or ambiguous reads, more than one active shipment, CLI/MCP
   disagreement, a shipment-lifecycle doctor finding, or manifest drift are
   indeterminate. Fail closed and defer to the backlogit claim gate.
3. The marker is advisory and does not replace backlogit claim authorization.
4. Do not route any successor behind `154-S` until the external scheduler
   owner attests that the deployed consumer uses the marker contract.

### Rollout, monitoring, and rollback

- **Deployment path:** merge-only for backlogit. External scheduler rollout
  and consumption are outside this repository and are not represented as
  complete.
- **Pre-route audit:** verify the running external scheduler reads the active
  shipment, its exact `custom_fields.items` manifest, and item `status` plus
  `custom_fields.scheduler_baseline_claim`; verify all binaries opening the
  workspace include the shipped recovery compatibility.
- **Healthy signals:** CLI/MCP reads agree; at most one shipment is active;
  the consumer applies the three-part predicate; successor routing remains
  withheld until the attestation is recorded.
- **Failure signals:** absent or mismatched marker, more than one active
  shipment, a non-zero/erroring recipe read, CLI/MCP divergence, journal
  finding, or a changing manifest.
- **Rollback trigger:** any indeterminate or mismatched consumer result
  before successor routing. Stop routing immediately; do not mutate archived
  marker data or synthesize shipment events.
- **Rollback / mitigation:** defer to the backlogit claim gate and operator.
  A code rollback, if required, must be a separately reviewed PR; do not
  revert or rewrite this shipped archive as an operational shortcut.
- **Owner:** repository operator and the external autoharness scheduler
  owner.
- **Validation window:** until the attestation is recorded and the first
  successor shipment has been observed applying the contract.

## Residual risks and follow-up

The following existing stash entries remain untouched and are carried as
residual-risk references:

| Stash ID | Residual |
|---|---|
| `AD5AECAF` | Pin the Ship lint gate to the CI version to avoid local-toolchain drift. |
| `4DB1DFF1` | Deferred pre-existing lint/staticcheck and formatting debt outside the authorized surface. |
| `5A1C4D3F` | Measurement-gated Windows `-race` test-budget calibration. |
| `D116AF58` | Profile per-member shipment validation latency for large shipments. |
| `67F17B6B` | Existing harness/backlogit advisory-lock sidecar naming collision. |
| `E52607F5` | Deferred non-queued member-parent cascade / marker recovery edge case. |
| `8B52A5F1` | Capture-only deferred scope entry for the harness/backlogit lock-name collision. |

The successor-routing attestation and enforcement condition is already
tracked by active stash `AF1E5075`; no duplicate was created. The incomplete
target-all context compaction pass is captured as follow-up stash `A17EE897`.
The related compound learning was reviewed and classified `keep` in
`2026-09-30-154-s-compound-refresh.md`.

## Context compaction (P-020)

`compact-context` was invoked with `target: all`. The run is recorded as
`degraded`, not complete: it compacted the completed 139-S memory record into
`docs/memory/compacted/2026-09-30-139s-post-merge-closure-compacted.md` and
archived its verbose original at
`docs/archive/memory/2026-09-09/2026-09-09-139s-post-merge-closure-complete.md`
(507 bytes recovered), but did not complete classification and compaction of
the remaining target set. The current 154-S session checkpoint was preserved
because its closure PR is still pending. See
`2026-09-30-154-s-compaction-report.md` for the bounded run result and
remaining work.

## Releasability

**`READY_WITH_CONDITIONS`.** The local shipment and archive closure is
verified. The closure PR may proceed through its review and CI gates, but
successor routing remains blocked until the external scheduler-consumption
attestation is recorded. No runtime deployment or external validation is
claimed here.

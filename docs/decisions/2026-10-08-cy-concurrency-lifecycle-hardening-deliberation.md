---
title: 'Deliberation: CY concurrency and lifecycle hardening (three waves)'
doc_type: decision
source: docs/decisions/2026-10-08-cy-concurrency-lifecycle-hardening-deliberation.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: 'Stage deliberation for the CY group of 13 deferred-scope-expansion stash entries covering lifecycle error classification, lock and event ordering, declared-postcondition accuracy, and filesystem TOCTOU hardening. Splits CY into three wave-aligned shipments CY-A, CY-B, and CY-C.'
topic: concurrency-lifecycle-hardening
depth: standard
decision_status: accepted
promoted_to: docs/exec-plans/2026-10-08-cy-concurrency-lifecycle-hardening-plan.md
linked_artifacts:
  - docs/decisions/2026-10-08-cx-s-ship-closure-gate-correctness-deliberation.md
  - docs/design-docs/artifact-mutation-lock-order.md
---

# Deliberation: CY concurrency and lifecycle hardening

## Subject

The operator approved CY as one group of 13 stash entries, planned after CX:
5247D4BC, E4908F30, BACD94FC, A0C733C6, E45E6D65 (supersedes 37699341),
DBF89C9C, 0FFBF819, 45BD3B36, 7D8717B1, 8AF55264, FA6AE139, AB31C9C5, and
010F437C. Every member carries the `DEFERRED SCOPE EXPANSION` marker, so P-021
C6 forces this deliberation.

## Framing

The members cover four contract surfaces:

1. Lifecycle error classification and the blocked-envelope write boundary
   (7D8717B1, 8AF55264, FA6AE139). These live in `internal/core/shipment.go`,
   `internal/core/artifacts.go`, `internal/core/shipment_blocked_envelope.go`,
   `internal/events`, and `internal/mcp/errors.go`.
2. Lock and event ordering across governed mutations (5247D4BC, E4908F30,
   BACD94FC, DBF89C9C). These follow `docs/design-docs/artifact-mutation-lock-order.md`.
3. Accuracy of the faultline mutation declarations (AB31C9C5, 010F437C), whose
   EventsJSONL postconditions do not match the actual core behavior.
4. Filesystem TOCTOU and parser hardening (A0C733C6, E45E6D65, 0FFBF819,
   45BD3B36) in reconcile preconditions, Windows reconcile writers, and the
   blocked-shipment snapshot read.

The group is too large for one release unit: about 20 two-hour tasks across
four surfaces with separate risk profiles.

## Options

### Option 1: one CY shipment

One feature and one shipment. This is simpler to stage, but the PR would be
very large and mix error classification, lock-order, and filesystem security
review. It fails the reviewability goal.

### Option 2: three wave-aligned shipments (recommended)

* CY-A: error classification and the blocked-envelope boundary (surface 1).
  This is the smallest and most contract-adjacent wave. It touches
  `internal/mcp/errors.go` and the BlockShipment/ClaimShipment area that 189-S
  also edits.
* CY-B: lock and event ordering plus declaration accuracy (surfaces 2 and 3).
  Declaration accuracy depends on the event-writing behavior that the
  ordering work settles, so they ship together.
* CY-C: filesystem TOCTOU and parser hardening (surface 4). This wave needs
  security review and a Windows spike.

Chain them `CY-B blocks-on CY-A` and `CY-C blocks-on CY-B`, and `CY-A
blocks-on CX`.

### Option 3: four shipments, one per surface

This adds a fourth hand-off for two small declaration tasks. It isn't worth
the overhead.

## Decisions

* D1: adopt Option 2. CY-A, CY-B, and CY-C each get a covering feature.
* D2: `189-S blocks-on CY-A`. The planning shows a real overlap: CY-A changes
  `domainError` ordering in `internal/mcp/errors.go` and BlockShipment member
  write accounting in `internal/core/shipment.go`, and 189-S (E52607F5 fold-in
  and its marker work) edits the claim recovery path in the same files. No
  overlap was found with CY-B or CY-C.
* D3: 5247D4BC (bind reconciliation to the membership lock domain) is a
  cross-layer locking decision. Its unit is characterization-first: pin the
  current interleaving with a test, then decide in a design note inside the
  same task whether reconciliation takes the membership lock domain. The
  implementation follows in a separate unit.
* D4: E45E6D65 supersedes 37699341. The newer entry proves that real payload
  bytes, not just an empty placeholder, can leak. Record `E45E6D65 supersedes
  37699341` and archive 37699341. The CY-C unit starts with a spike that evaluates
  NT-native directory-relative creation (`NtCreateFile` with a root directory
  handle) against a stage-then-verify write to an in-workspace temp file.
* D5: AB31C9C5 and 010F437C use one rule: a declaration lists EventsJSONL only
  when the core path makes the event write fatal. Otherwise it is marked
  conditional. Do not change the core event-writing semantics in CY.
* D6: FA6AE139 classifies `appendFast` failures (`ErrWriteIndeterminate` after
  bytes, `ErrWriteNotApplied` before) instead of special-casing compensate.
* D7: 0FFBF819 and 45BD3B36 share the blocked-shipment snapshot read path.
  0FFBF819 is a handle-relative no-follow open. 45BD3B36 is duplicate-member
  rejection with a token-level pre-scan before decode.

## Scope Boundaries

* In scope: the 13 entries above.
* Out of scope: openat2 for checkpoint reads (3F06493B), snapshot_ref
  provenance (09D06A75), and the 2692E24C declaration audit. These overlap by
  keyword only and are separate expansions.

## P-021 C5/C6 Records

### Duplicate Scan (A)

| Entry | Outcome |
|---|---|
| 37699341 | Duplicate merge. Surviving entry E45E6D65 (newer, sharper finding that explicitly supersedes 37699341). Disposition: supersedes link, then 37699341 archived. The survivor is the later capture because it carries the stronger evidence and states the supersession itself, so the earliest-captured rule is overridden by the explicit supersession. |
| 0FFBF819 | Clean scan. DISCOVERY-STATUS AMBIGUOUS candidate 09D06A75 reviewed: provenance contract, not handle-relative open. Not the same expansion. |
| 45BD3B36 | Clean scan. 09D06A75 is not the same contract. |
| AB31C9C5 | Clean scan. 2692E24C reviewed: different declaration. |
| All others | Clean scan. No duplicate found. |

### Late-Identifier Reconciliation (B)

| Entry | Outcome |
|---|---|
| 7D8717B1, 8AF55264, FA6AE139 | PR #450 (155-S) recovered from the 155-S closure record. |
| E4908F30, BACD94FC, DBF89C9C | PR #485 (153-S) recovered from the 153-S closure record. |
| 5247D4BC | No late identifier found. PR N/A and thread N/A stand. |
| Others | Source refs already concrete. No reconciliation needed. |

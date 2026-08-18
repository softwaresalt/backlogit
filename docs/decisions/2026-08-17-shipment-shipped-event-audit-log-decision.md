---
title: "Decision: durable shipped-event append with class-aware rollback and report-only doctor reconciliation"
description: "Durable decision record for stash 0115F71F, distinguishing committed clean-main behavior from planned behavior and from the deferred prevention follow-up 47B48DB0"
source: ".backlogit/queue/059-DL.md"
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Status

Decided. Harvested into feature `143-F` on branch `chore/restage-143-shipment-audit-log`, based on
`origin/main` `3ec95ee3e7c6787762beb15c0e4b226746e50a89`.

This record supersedes the staging attempt on
`origin/chore/stage-143-shipment-audit-log-reconciled` (`2188bab2`), which asserted a baseline the
committed tree does not contain. That branch is retained as reference history only.

## Context

Stash `0115F71F` reports that a shipment can persist `archived_status: shipped` while its
append-only JSONL log omits `shipment_status_changed: shipped`. Observed in the field from
autoharness stash `84D8E6AB` / shipment `114-S`.

## Three-state separation

The single most important property of this record is that it keeps three things apart. The prior
staging attempt failed review three times largely because it blurred them.

### A. What is committed on clean `origin/main` today

| Fact | Evidence |
|---|---|
| The shipped-status path emits its audit event through the best-effort, fire-and-forget `appendItemEvent` | `internal/core/shipment.go:205`, implementation at `:660` / `:677`, warning-and-return at `:690` and `:696` |
| The status is already persisted before that emission | `internal/core/shipment.go:200` (`persistArtifactWithGuard`) |
| Archival proceeds regardless of the append outcome | `internal/core/shipment_lifecycle.go:509` returns the locked closure; `:515-527` archives unconditionally |
| An error-returning appender exists, but only for gate evidence | `appendItemEventErr` at `internal/core/gate_evidence.go:40`; seam `gateEvidenceAppend` at `internal/core/workspace.go:55-59`; dispatcher at `internal/core/gate_evidence.go:81-86` |
| There is **no** shipment-scoped `ws.shipmentEventAppend` seam | ripgrep across `internal/`, `cmd/`, `tests/` returns zero matches |
| There is **no** `shipmentEventAppendError` classification type | ripgrep returns zero matches |
| No transition out of `shipped` is permitted | `isValidShipmentTransition` at `internal/core/shipment.go:713-722` |
| `restoreShipArtifacts` silently skips an item whose log lock it cannot re-acquire | `internal/core/shipment_lifecycle.go:184-188` |
| LIFO defer order runs the non-member covering-feature fallback **after** the artifact locks are released | registration at `:365` and `:375` |
| `EventWriter.AppendEvent` returns only `error`, and the default non-durable `appendFast` path discards the `fmt.Fprintf` byte count | `internal/events/stream.go:243`, `:280-292` - a caller cannot tell a pre-write `open` failure from a short write |
| The durable append path, when enabled, does tag both durability classes | `appendDurable` contract at `internal/events/stream.go:294-304` |
| The in-process item-log lock is uncancellable; only the cross-process sidecar lock is bounded | `LockItemLog` `mutex.Lock()` at `internal/events/stream.go:125`; bounded path at `:40`, `:177` |

### B. What this decision plans to change

* Introduce a per-workspace `shipmentEventAppend` seam and a shipment-scoped
  `appendShipmentEventErr` that owns the item-log lock and tags **only what it can prove**: a
  pre-write lock failure is tagged `ErrWriteNotApplied`; any writer error is wrapped with `%w` and
  given no class of its own, so a durability sentinel the writer already attached survives and an
  untagged error stays untagged.
* Route **only** the `active -> shipped` transition through it fail-closed, and only on the governed
  `ShipShipment` path: the gate is `newStatus == ShipmentShipped && !topLevel`, because
  `ShipShipment` is the sole caller passing `topLevel=false` for that status
  (`internal/core/shipment_lifecycle.go:509`) while the exported `MoveShipmentStatus` passes `true`
  (`internal/core/shipment.go:115-117`). Gating on the status alone would make the exported entry
  point fail closed with no compensating half. Claim and abandon keep best-effort semantics. Generic
  update and archive producers are **not** gated; they are covered report-only by the audit.
* Classify conservatively, indeterminate-first: tagged `ErrWriteIndeterminate` **and every
  unclassified append error** suppress rollback and return a `MutationPartialError`; only a
  **proven** not-applied outcome compensates, and an error carrying both sentinels classifies
  indeterminate. Every failure branch returns a `MutationPartialError`, including the fully
  compensated one, so the outcome is measurable rather than an opaque internal error. This deliberately inverts the untagged default in `MutationEnvelope`,
  because the envelope's persist step routes through a primitive that tags both classes
  (`internal/atomicfile/atomicfile.go:82-129`) while the default non-durable append path tags
  nothing. The writer API is not expanded; `internal/events/` is out of scope.
* Make compensation honest: an item that cannot be restored surfaces as
  `CompensationState: "partially-compensated"` naming the un-restored IDs, never a silent skip, and
  the `CompensationState` doc comment is amended to admit that fourth value.
* Correct the adjacent defer ordering so the non-member covering-feature fallback runs with the
  artifact locks genuinely held.
* Add a report-only doctor audit emitting two distinct findings, `missing_shipped_event` and
  `shipped_unarchived_residue`, that never writes or rewrites JSONL.
* Surface the audit on the CLI and on MCP, and make the MCP recovery guidance producer-scoped so it
  names a check that can actually detect the residue.
* Update the governed-recovery contract doc, workflow policy P-007, the `shipment-reconcile` skill,
  and the Ship agent, all of which currently encode a "ship always archives" postcondition that this
  change deliberately breaks.

### C. What is deferred and to where

| Deferred item | Owner |
|---|---|
| Prevention: closing non-`ShipShipment` paths that can also produce `archived_status: shipped`, including the deliberation's minimum floor that `UpdateArtifactWithGate` must not drive a shipment to `shipped` bypassing the durable event | active stash `47B48DB0` |
| A supported reconciliation transition out of `shipped` | named closure follow-up |
| Durability of the item-level events inside `ShipShipment` (`status_changed`, `returned_to_backlog`, parent cascades) | named closure follow-up |
| A richer append outcome from `internal/events` (byte count or explicit applied/not-applied result) that would let the ship path narrow unproven failures below `indeterminate` | named closure follow-up |
| Reconciling the two pre-existing drifted registry doctor params and adding a repo-wide `params`-to-`InputSchema` parity assertion | named closure follow-up |

The guarantee this decision buys is therefore **path-scoped**: on the governed `ShipShipment`
archival path a shipment cannot reach `archived_status: shipped` unless the shipped event was
durably appended, or archival was halted with a `MutationPartialError`. It is **not** a universal
prevention claim, and no artifact derived from this record may state one.

Detection ships now because historical residue already exists and the forward fix cannot repair it.
Prevention waits because closing those paths is a different concern with a different blast radius,
and the report-only audit already flags residue regardless of which producer created it.

## Options considered

| Option | Verdict |
|---|---|
| A. Wrap the whole status-to-archival path in `core.MutationEnvelope` | Rejected: very high blast radius across the hardened `ShipShipment` locked closure, its membership lock, the head-drift guard, and the 133-F covering-feature restore |
| B. Shipment-scoped error-returning append inside the existing envelope, classified per the two-class durable-write contract, plus a report-only doctor audit | **Chosen** |
| C. Swap to an error-returning appender and always roll back | Rejected: rolling back an indeterminate write can destroy an applied append, violating the contract in `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` |
| D. Hoist the shipment item-log lock to the top of the locked closure | Rejected at review cycle 2: the ownership marker in `ctx` would outlive the lock, exposing the rollback log rewrite and the archival appends to lock-free execution, inverting the lock order against `lockArtifactMutations`, and holding a starving lock across two gate-broker evaluations |
| E. Retry the append before classifying | Rejected at review cycle 2: an append is explicitly not safe to blindly retry, and retrying risks a duplicate shipped event in the log this change exists to protect |
| F. Keep the append best-effort and only warn more loudly | Rejected at adversarial review: this is the status quo the bug report is about and it contradicts the stash's explicit "archival must not continue without that durable event" |
| G. Expand `EventWriter.AppendEvent` to report a write byte count or an explicit applied/not-applied outcome, so the ship path could compensate on a proven-not-applied non-durable failure | Rejected at PR review cycle 1: it changes a shared primitive used by every event producer and both append paths to recover a distinction the fail-closed branch does not need. The conservative classification achieves the safety property with no writer change; narrowing the class is a named closure follow-up |
| H. Keep the earlier compromise - classify untagged append errors `not-applied` and promise to tag short writes `ErrWriteIndeterminate` | Rejected at PR review cycle 1 as unimplementable: `AppendEvent` returns only `error` and `appendFast` discards the byte count (`internal/events/stream.go:243`, `:290-291`), so nothing in `internal/core` can observe a short write. The promise would have shipped as an untested, unenforceable claim |

## Chosen classification contract

| Observed append outcome | Pre-write status | Class | Rollback |
|---|---|---|---|
| Lock acquisition fails inside `appendShipmentEventErr`, before any writer call | proven not-applied | `not-applied` | compensate |
| Writer error explicitly tagged `blerrors.ErrWriteNotApplied` | proven not-applied | `not-applied` | compensate |
| Writer error explicitly tagged `blerrors.ErrWriteIndeterminate` | proven unknown | `indeterminate` | suppress |
| Any other append error, including every untagged error from the default non-durable path | not proven | `indeterminate` | suppress |

## Accepted consequences

* A ship that previously succeeded can now refuse. That is the intent, scoped to the shipped
  transition on the governed `ShipShipment` path only.
* Because pre-write status is unobservable on the default non-durable append path, a genuinely
  not-applied `open` failure is classified `indeterminate`. The shipment is then left `shipped`
  and unarchived and needs the documented manual reconciliation rather than an automatic revert.
  This is the accepted cost of never compensating over a possibly-applied append; enabling
  `Config.DurableWrites` narrows it, because that path tags `not-applied` explicitly.
* A new end state exists - `shipped` and unarchived - with **no automated forward transition**. This
  is declared, monitored by SLI 2 and SLI 3, and has a documented manual recovery procedure.
* The guarantee is path-scoped. Generic update and archive producers can still create the residue
  until stash `47B48DB0` lands; they are detected, not prevented.
* Historical residue is reported, never repaired. The audit never synthesizes a missing event.
* Twelve tasks, roughly twenty-four hours. The decomposition was independently judged proportionate
  by the Architecture Strategist and the Scope Boundary Auditor after two prior review cycles.

## Gate record

| Gate | Outcome |
|---|---|
| Plan review, cycle 1 | FAIL - P0=3, P1=16 across five personas |
| Plan review, cycle 2 | FAIL - P0=2, P1=12 |
| Plan review, cycle 3 | **PASS** - P0=0, P1=0 |
| Adversarial multi-model review, 3 reviewers on 3 model families | **PASS** - HIGH-confidence P0/P1 = 0; 2 MEDIUM findings fixed; of 15 LOW findings, 13 fixed and 2 rejected with rationale |
| PR #366 Copilot review, cycle 1 | 9 comments, all accepted; revision 4 replaced the short-write promise with the conservative classification contract, moved both core tracks to harness-first ownership, corrected the lock-bound and review-count claims, reconciled `059-DL`, and narrowed the feature guarantee to the governed path |
| Plan review, cycle 4 (revision 4) | FAIL - P0=0, P1=11 across five personas; all remediated in revision 5: locked-context propagation, governed-path gating, the Unit 4 split into `143.004-T` and `143.012-T`, the no-early-return rollback rule, a sanctioned partial-compensation injection mechanism, all-red harness scenarios, colocated appender tests, and the contract-doc carve-out |
| Adversarial review, panel 2 (revision 4) | **PASS** - HIGH-confidence P0/P1 = 0 across three model families (A PASS, B FAIL with 8 P1s, C PASS); 5 MEDIUM findings fixed, 2 LOW findings accepted as declared risk |

## Superseded lesson

The prior staging attempt tripped a three-cycle circuit breaker. The cause was not reviewer
disagreement; it was that the plan asserted a baseline the committed tree did not contain - most
importantly a shipment-scoped append seam and an error-returning shipped path that do not exist on
`origin/main`. Four durable lessons carried into this record:

1. Baseline claims must be re-derived in a clean worktree at a named SHA, and the anchors must be
   regenerated mechanically rather than transcribed. Both cycle-1 and cycle-2 reviews found
   hand-written anchors drifting by a few lines even when the substance was right.
2. Red-before-green must be expressed as a dependency edge, not as prose. The prior plan gave the
   production seam to the first task and the failing tests to the third, which inverted the
   constitutional order while reading as if it complied.
3. A "purely additive scaffold" task ahead of a harness is still production before test. PR review
   cycle 1 caught the second-order version of lesson 2: the scaffold units were inert, but they
   changed production files - including a call site - before any failing test existed. The correct
   shape is a harness task that carries the declarations its own failing test needs to compile, which
   is what `.github/skills/harness-architect/SKILL.md` Step 4.3 already prescribes.
4. A plan may not promise a distinction the code cannot observe. The earlier revision required
   classifying short and partial writes as indeterminate, but `AppendEvent` returns only `error` and
   `appendFast` discards its byte count. The rule that replaced it - only a proven pre-write failure
   may compensate - is enforceable at the boundary the plan actually controls.

## References

* Plan: `docs/exec-plans/2026-08-17-shipment-shipped-event-audit-log-plan.md`
* Adversarial review: `docs/reviews/2026-08-17-shipment-shipped-event-audit-log-adversarial-review.md`
* Deliberation: `.backlogit/queue/059-DL.md`
* Prior art: `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`,
  `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md`,
  `docs/compound/2026-07-20-manual-feature-harvest-provenance-backfill.md`

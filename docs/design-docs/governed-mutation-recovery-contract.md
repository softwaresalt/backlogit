---
chunk_strategy: h1-h2-h3
description: Contract for idempotent multi-mutation envelopes, partial-mutation classification, and recovery surfaces across governed backlogit write paths
doc_type: design
docline:
    date: 2026-08-10T00:00:00Z
    status: accepted
    tags:
        - mutation-envelope
        - recovery
        - durability
        - doctor
        - mcp
        - idempotency
ingested_at: "2026-08-10T00:00:00Z"
schema_version: "1.0"
source: docs/design-docs/governed-mutation-recovery-contract.md
title: Governed mutation recovery contract
---

# Governed Mutation Recovery Contract

## Scope

This contract defines how backlogit reports and recovers from failures in
ordered multi-store mutations where one logical write spans more than one
surface:

* artifact frontmatter
* SQLite cache rows
* append-only JSONL event logs

The current governed paths are:

* commit association
* artifact creation with eager dependency-edge indexing
* dependency mutation
* shipment membership mutation
* shipment ship (the governed `ShipShipment` active-to-shipped audit append)

## Envelope contract

`internal/core.MutationEnvelope` executes an ordered slice of named mutation
steps. Each step provides:

* `Name` — a stable machine-readable identifier
* `Apply(ctx)` — the forward mutation
* `Compensate(ctx)` — the rollback for already-applied earlier steps

On success, the envelope returns `nil`.

On failure after one or more steps were applied, the envelope returns
`*errors.MutationPartialError`. The typed payload carries:

* `Completed` — applied step names in order
* `FailedStep` — the step that classified the failure
* `CompensationState` — `compensated`, `not-compensated`, `partially-compensated`,
  or `unknown`. `partially-compensated` reports a compensation that ran but could
  not restore every affected item; the producer names the un-restored IDs.
* `Class` — `not-applied`, `indeterminate`, or `double-fault`
* `Cause` — the wrapped original error, or a joined error set

Callers inspect the payload with `errors.As` rather than parsing text.

## Classification precedence

The invariant is strict:

* if any step returns `ErrWriteIndeterminate`, the envelope class is
  `indeterminate`
* `indeterminate` dominates any plain or `ErrWriteNotApplied` errors in the same
  joined set
* once the envelope enters the `indeterminate` branch, it never compensates

This is the governed-mutation recovery rule: when a durable append may already
be present, backlogit prefers convergence over rollback.

### Producer carve-out: the shipped-event append

The rule above describes `MutationEnvelope`, whose persist step routes through a
write primitive that tags BOTH durability classes explicitly. The governed
`ShipShipment` shipped-event append does not have that guarantee and therefore
diverges deliberately:

* `events.EventWriter.AppendEvent` exposes only `error`, and its default
  non-durable append discards the write byte count, so a core-side wrapper cannot
  distinguish a pre-write open failure from a short or partial write.
* Only a **proven** pre-write outcome may compensate: the item-log lock failure
  raised inside `appendShipmentEventErr` before any writer call, or a writer error
  explicitly tagged `ErrWriteNotApplied`.
* **Every other append error, including every untagged error, is `indeterminate`**
  and suppresses rollback. This is the inverse of the envelope's untagged default,
  and it is intentional: compensating over a possibly-applied append to an
  append-only log is the one outcome the `AppendEvent` contract forbids.
* Short-write detection is not promised, tested, or relied upon anywhere.

## Failure branches

### Not applied

When the failing step returns a plain error or `ErrWriteNotApplied`, the
envelope compensates the already-applied earlier steps in reverse order. If all
compensations succeed, the returned class is `not-applied`.

**Shipped-event carve-out**: on that producer a plain (untagged) error does NOT
compensate — it classifies `indeterminate`. Only the two proven pre-write outcomes
listed above reach this branch. When compensation runs but cannot restore every
release-scope item, the result is `not-applied` with
`CompensationState: partially-compensated`, naming the un-restored IDs, rather
than a silently skipped item.

### Indeterminate

When the failing step returns `ErrWriteIndeterminate`, the envelope does not
compensate. It continues running later steps so the stores can converge on the
intended end state, then returns `Class: indeterminate`.

**Shipped-event carve-out**: that producer **halts** instead of continuing.
Archival is deliberately not attempted, leaving a detectable, documented
`shipped`-and-unarchived residue rather than a permanently missing audit record.
There is no supported forward transition out of `shipped`; recovery is the
operator procedure documented in P-007.

### Double fault

When a non-indeterminate failure triggers compensation and one of the
compensations also fails, the envelope returns `Class: double-fault` and joins
the forward-failure error with the compensation error set.

## Idempotent rerun rule

The envelope does not provide idempotency by itself. Idempotency stays with the
caller and the underlying mutation seams.

Governed callers must therefore use idempotent or convergence-safe steps:

* upserts instead of blind inserts where possible
* state restoration for frontmatter compensation
* append-only event writes that tolerate a retry by surfacing a typed partial
  result rather than pretending rollback occurred

The operational rule is simple: after a partial failure, a caller may safely
re-run the same logical mutation, provided each step implementation is itself
designed for rerun safety.

## Recovery discovery

Doctor exposes read-only recovery detection through
`DoctorOptions.CheckPartialMutations` and
`DoctorOptions.CheckShippedEventCompleteness`.

The current checks are:

* `partial_commit_association` — a `commit_links` row exists without a matching
  `commit_tracked` JSONL event for the same SHA
* `inconsistent_dependency_edge` — frontmatter dependency edges and `item_deps`
  cache rows disagree
* `missing_shipped_event` — an archived shipment whose `archived_status` is
  `shipped` has no `shipment_status_changed: shipped` event in its item JSONL, or
  its log could not be read at all (reported explicitly as "event presence
  unknown"). Enabled by `CheckShippedEventCompleteness`
* `shipped_unarchived_residue` — a shipment left `shipped` but unarchived, with
  shipped-event presence recorded and the stranded archive candidates enumerated.
  Enabled by `CheckShippedEventCompleteness`

`FailedStep: shipped-event-append` (the exported constant
`errors.StepShippedEventAppend`) is the discriminator that routes recovery guidance
to the shipped-event audit rather than to `check_partial_mutations`, which
provably cannot detect this residue. Recovery guidance is therefore keyed on the
PRODUCER first and the class second, and it differs by `CompensationState`:
`compensated` is safe to retry, `partially-compensated` requires reconciling the
named un-restored IDs first (the audit may report clean while they remain torn),
and `not-compensated` means the shipment is shipped-and-unarchived and the P-007
named-limitation procedure applies. `Retryable` is `Class == "not-applied" &&
CompensationState == "compensated"`.

These findings are advisory. They never change doctor exit behavior on their
own.

## MCP result shape

The MCP layer maps `*errors.MutationPartialError` to a structured domain error
payload instead of flattening the failure into a string. The result includes:

* `error: mutation_partial`
* `classification`
* `completed_steps`
* `failed_step`
* `compensation_state`
* `retryable`
* `recovery`

This lets agent callers branch on machine-readable recovery guidance while still
preserving the wrapped cause chain.

## Honest boundary

The envelope is an in-process coordination primitive. It improves honesty and
recovery after step-level failures inside one running process. It does not
survive:

* process termination
* OS crash
* power loss between steps
* external mutation by another process after a step completed

Doctor and idempotent rerun behavior are the follow-up recovery surfaces for
those boundaries.

## Atomic-write premise correction

This contract does not change `internal/atomicfile`.

The relevant write primitive remains:

* Windows: `MoveFileEx` via Go's `os.Rename` implementation
* other platforms: plain `os.Rename`

Both paths provide atomic rename semantics for the final replace step under the
same filesystem assumptions already relied on elsewhere in backlogit. F5 governs
multi-mutation coordination above that primitive; it does not redefine the
atomic-file contract.

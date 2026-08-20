---
title: "Stage session memory: PR #366 review-fix cycle 1 for 143-F / 127-S"
description: "Session memory for the Stage review-fix cycle that answered nine Copilot comments on PR #366, replaced the unimplementable short-write promise with a conservative append classification contract, moved both core tracks to harness-first ownership, split the compensation unit out of Unit 4, and re-ran plan review plus a second adversarial panel"
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Session frame

| Field | Value |
|---|---|
| Agent | Stage |
| Date | 2026-08-17 |
| Mode | `DARK_MODE_ACTIVE`, review-fix cycle 1 on PR #366 |
| Scope | plan, decision, review, memory, deliberation `059-DL`, feature `143-F`, tasks `143.001-T` .. `143.012-T`, shipment `127-S` |
| Reviewed HEAD | `df11660f` (the exact commit Copilot reviewed) |
| Branch | `chore/restage-143-shipment-audit-log` |
| Worktree | `.worktrees/143-restage`, no other worktree touched |
| Intercom | unavailable; visibility persisted locally in this file and in the plan gate records |
| Role boundary | Stage only: no source, test, or config edits; no Go build; no PR replies; no merge |

## The nine Copilot comments and their disposition

| # | Comment | Disposition |
|---|---|---|
| 1 | Plan line ~205: short/partial-write classification is not implementable because `AppendEvent` returns only `error` and `appendFast` discards the `fmt.Fprintf` byte count | **Accepted.** The writer API is not expanded. New classification contract: only a proven pre-write failure compensates; every other append error, untagged included, is indeterminate. `internal/events/` is now explicitly outside the freeze scope. |
| 2 | `143.001-T` promised the same unimplementable classification | **Accepted.** The task no longer promises short-write detection; the appender tags only its own lock failure and passes writer errors through unmodified. |
| 3 | TDD violation: Units 1 and 5 changed production before their RED harnesses | **Accepted.** `143.001-T` and `143.005-T` are now the RED harness tasks, carrying only the declarations their failing tests need to compile. No scaffold task, no scaffold exception. |
| 4 | The 3-second lock bound is false for in-process contention | **Accepted.** The Baseline Verification row now separates the bounded cross-process sidecar lock from the uncancellable in-process `mutex.Lock()` at `internal/events/stream.go:125`. |
| 5 | Review report LOW counts are wrong | **Accepted.** 15 LOW findings: L1-L13 fixed, R1-R2 rejected. Corrected in the report, plan, decision, and memory. |
| 6 | Session memory carried the stale count | **Accepted.** Corrected. |
| 7 | Decision record carried the stale count | **Accepted.** Corrected. |
| 8 | `059-DL` chosen option is stale: names `appendItemEventErr` and compensates untagged errors | **Accepted.** Both occurrences now name `appendShipmentEventErr` and record the conservative classification; the open-questions section is reconciled too. |
| 9 | `143-F` goal claims universal prevention while generic paths stay deferred | **Accepted.** The guarantee is now path-scoped to the governed `ShipShipment` archival path everywhere; prevention for other producers stays deferred to stash `47B48DB0` and is covered report-only by the audit. |

## Corrected classification contract

| Observed append outcome | Pre-write status | Class | Rollback |
|---|---|---|---|
| Lock failure raised inside `appendShipmentEventErr` | proven not-applied | `not-applied` | compensate |
| Writer error tagged `blerrors.ErrWriteNotApplied` | proven not-applied | `not-applied` | compensate |
| Writer error tagged `blerrors.ErrWriteIndeterminate` | proven unknown | `indeterminate` | suppress |
| Any other append error, untagged included | not proven | `indeterminate` | suppress |

Precedence is indeterminate-first, so an error carrying both sentinels can never be compensated.
The accepted cost is that a genuinely pre-write `open` failure in the default non-durable
configuration strands the shipment as `shipped`-and-unarchived instead of reverting it; that residue
is detected by `shipped_unarchived_residue`, measured by SLI 2 and SLI 3, and resolved by the
documented manual procedure.

## Corrected task graph and TDD order

```text
Durability: 143.001-T (RED harness + seam declaration)
          -> 143.002-T (appender, dispatcher, StepShippedEventAppend, colocated appender tests)
          -> 143.003-T (governed-path fail-closed routing + boundary type)
          -> 143.004-T (classification + indeterminate branch + slog)
          -> 143.012-T (honest compensation + defer swap + ordering regression)
Detection:  143.005-T (RED harness + finding-type and option declarations)
          -> 143.006-T (missing_shipped_event + archived_status plumbing + non-mutation guard)
          -> 143.007-T (shipped_unarchived_residue + stranded archive-candidate enumeration)
Surfaces:   143.008-T (CLI) -> 143.009-T (MCP) -> 143.010-T (recovery guidance)
Coherence:  143.011-T (contract doc, P-007, reconcile skill, Ship agent, drift-ignore)
```

Seventeen `blocks` edges, acyclic, two roots (`143.001-T`, `143.005-T`), one sink (`143.011-T`).
`143.012-T` is new: the Constitution Reviewer found the original Unit 4 changed more than five
functions, breaching the 2-hour rule on the riskiest change in the feature.

## Gate record for this cycle

| Gate | Result |
|---|---|
| Plan review cycle 4, five personas | FAIL - P0=0, P1=11 (Constitution 3, Concurrency 3, Architecture 2, Coupling 2, Scope 1) |
| Adversarial panel 2, three model families | A PASS (0/0), B FAIL (0 P0, 8 P1), C PASS (0/0); HIGH-confidence P0/P1 = 0 |
| After remediation | All P0/P1 resolved in revision 5; 2 LOW findings accepted as declared risk and recorded in the Risks table |

## The eleven P1 findings and what changed

1. **Locked context** - `appendShipmentEventErr` must pass the context returned by
   `LockItemLogCrossProcess` to `AppendEvent`; otherwise the non-reentrant uncancellable
   `mutex.Lock()` at `internal/events/stream.go:125` deadlocks the ship goroutine while it holds the
   membership lock and every artifact lock. Now a unit change, a guardrail, and a stop condition.
2. **Governed-path gating** - the gate is `newStatus == ShipmentShipped && !topLevel`, because
   `ShipShipment` is the only caller passing `topLevel=false` for that status. Gating on the status
   alone would have made the exported `MoveShipmentStatus` fail closed with no compensating half.
3. **Unit 4 too large** - split into `143.004-T` (classify and halt) and `143.012-T` (compensation,
   defer swap, flag truth table).
4. **Non-discriminating defer regression** - the old assertion passed with or without the swap; the
   new one observes the ordering directly and must be red before the swap.
5. **Untested appender contract** - colocated tests in `143.002-T`, written first and observed
   failing, cover lock tagging, sentinel passthrough, and untagged passthrough.
6. **Harness tasks shipped a passing scenario** - P-002, P-004, and harness-architect Step 5.2
   require all harness tests to fail, so both always-green scenarios moved into the first
   implementation unit of their track.
7. **Early return in the rollback loop** - forbidden; `unlockItemLog()` is a plain statement and the
   cross-process helper returns a nil unlock on error, so the promotion uses a per-item closure with
   a nil-guarded defer.
8. **Partial-compensation injection** - the seam cannot reach the compensation path; the sanctioned
   mechanism is a directory planted at the lock sidecar path from inside the seam callback, with the
   harness forbidden from taking the lock and every scenario carrying a watchdog.
9. **`143.003-T` without its classifier** - between them the existing unconditional rollback would
   compensate over an unproven append; a stop condition and both rollback triggers now bind
   `143.003-T`, `143.004-T`, and `143.012-T` together.
10. **Contract-doc carve-out** - Unit 11 now amends the classification-precedence and failure-branch
    sections and the closed `CompensationState` enumeration in
    `docs/design-docs/governed-mutation-recovery-contract.md`.
11. **Error-model coherence** - indeterminate-first precedence, a `MutationPartialError` on every
    failure branch including the compensated one, the joined restore failure inside `Cause`, and
    `Retryable` gated on `not-applied` plus `compensated`.

## Accepted risks, recorded rather than engineered away

* The retry budget bounds the loop, not a single acquisition, because the in-process lock has no
  deadline and `internal/events` stays frozen. Widening that contract is a named closure follow-up.
* SLI 4 and SLI 5 will correlate: compensation re-acquires the same item-log locks whose failure
  produced the not-applied class.

## Validation performed

* `backlogit sync` - 1107 artifacts indexed, zero parse failures
* `backlogit docs lint` - `violation_count: 0`
* `backlogit doctor` - 23 findings, all pre-existing `106.0xx-T` and `016.001-R` orphans; zero
  mention `143`, `127-S`, or `059-DL`
* Dependency SQL - 17 `blocks` edges, exactly as designed
* Shipment manifest - 13 members: `143-F` plus twelve tasks
* Stash SQL - `0115F71F` is `state=harvested`; `47B48DB0` is `state=active`
* Section and frontmatter integrity - `begin=3 end=3 empty=0` on every task, `begin=5 end=5` on
  `059-DL`, two frontmatter delimiters per file
* `git diff --check` clean; no `.go`, `go.mod`, `go.sum`, workflow, Makefile, or linter config touched

## Open questions and next steps

* Orchestrator replies to the nine PR #366 comments after this push; Stage did not reply and did not
  resolve any thread.
* Ship claims `127-S` and starts at `143.001-T` or `143.005-T`; the two tracks are independent roots.
* Six source anchors were corrected this cycle. The mechanical anchor sweep claimed in cycle 3 did
  not in fact cover every row - regenerate anchors from the worktree before the next revision rather
  than trusting the claim.

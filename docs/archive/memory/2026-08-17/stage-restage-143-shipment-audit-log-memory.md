---
title: "Stage session memory: restage 143-F / 127-S onto clean origin/main"
description: "Session memory for the operator-authorized Stage replan that rebuilt deliberation 059-DL, feature 143-F, eleven tasks, and shipment 127-S on a clean worktree from origin/main 3ec95ee3, superseding the halted three-cycle attempt"
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Session frame

| Field | Value |
|---|---|
| Agent | Stage |
| Date | 2026-08-17 |
| Mode | `DARK_MODE_ACTIVE`, operator-authorized fresh replan |
| Scope | stash `0115F71F` -> feature `143-F` -> shipment `127-S` |
| Base SHA | `3ec95ee3e7c6787762beb15c0e4b226746e50a89` (`origin/main`) |
| Branch | `chore/restage-143-shipment-audit-log` |
| Worktree | `.worktrees/143-restage`, created fresh from the base SHA |
| Intercom | unavailable; visibility persisted locally in this file and in the plan gate records |
| `merge_approval_pre_authorized` | true |
| `admin_fallback_pre_authorized` | false |

## Why this session existed

The prior staging attempt on `chore/stage-143-shipment-audit-log-reconciled` (`2188bab2`) tripped a
three-cycle circuit breaker. Root cause: the plan asserted a baseline the committed tree does not
contain, specifically a shipment-scoped append seam (`ws.shipmentEventAppend`) and an
error-returning shipped path, neither of which exists on `origin/main`. The operator authorized a
fresh replan from a clean baseline with corrected test-first ownership.

## Ground truth re-derived on `3ec95ee3`

* `appendItemEventErr` exists (`internal/core/gate_evidence.go:40`) but is gate-evidence scoped.
* `ws.gateEvidenceAppend` is the only per-workspace append seam (`internal/core/workspace.go:55-59`).
* `shipmentEventAppend` and `shipmentEventAppendError`: **zero matches** across `internal/`, `cmd/`,
  and `tests/`.
* The shipped path still calls best-effort `appendItemEvent` (`internal/core/shipment.go:205`) after
  the status is already persisted (`:200`), and `ShipShipment` archives unconditionally
  (`internal/core/shipment_lifecycle.go:515-527`).

## Artifacts produced

| Artifact | Path or ID |
|---|---|
| Deliberation | `059-DL` (`.backlogit/queue/059-DL.md`), created via `backlogit deliberate 0115F71F` |
| Feature | `143-F`, created via `backlogit stash harvest 0115F71F --type feature` |
| Tasks | `143.001-T` through `143.011-T`, all `parent_id: 143-F`, priority medium |
| Shipment | `127-S`, explicit membership of the feature plus all eleven tasks |
| Plan | `docs/exec-plans/2026-08-17-shipment-shipped-event-audit-log-plan.md` |
| Decision | `docs/decisions/2026-08-17-shipment-shipped-event-audit-log-decision.md` |
| Adversarial review | `docs/reviews/2026-08-17-shipment-shipped-event-audit-log-adversarial-review.md` |

## Corrected test-first ownership

The operator's central correction was that the prior decomposition inverted the constitutional
red-before-green order. PR review cycle 1 on PR #366 found the first fix was still inverted at
second order: each track opened with a "purely additive scaffold" task that changed production files
before any failing test existed, and one of those scaffolds also changed the shipped call site. The
corrected graph makes red-before-green structural on **both** tracks with no scaffold task at all -
each track opens with its RED harness, and the harness task carries only the declarations its own
failing test needs to compile:

```text
Durability: 143.001-T (RED harness + seam declaration) -> 143.002-T (appender, dispatcher, boundary type, constant) -> 143.003-T (fail-closed routing) -> 143.004-T (class-aware rollback)
Detection:  143.005-T (RED harness + finding-type and option declarations) -> 143.006-T (missing_shipped_event + archived_status plumbing) -> 143.007-T (shipped_unarchived_residue)
Surfaces:   143.008-T (CLI) -> 143.009-T (MCP) -> 143.010-T (recovery guidance)
Coherence:  143.011-T (contract doc, P-007, reconcile skill, Ship agent, drift-ignore)
```

Fifteen `blocks` edges, acyclic, two roots (`143.001-T`, `143.005-T`), one sink (`143.011-T`). The
edge set did not change between the original harvest and the cycle-1 correction; only the content
each task owns did.

## Decisions and rationale

* **Two distinct doctor findings**, not one: `missing_shipped_event` for an archived shipment with
  `archived_status: shipped` and no shipped event, and `shipped_unarchived_residue` for a shipment
  that is `shipped` but unarchived. Different causes, different operator responses, and the second is
  residue this plan itself creates.
* **Untagged append errors are indeterminate, not compensatable.** Corrected at PR review cycle 1.
  The original harvest matched the untagged default in `MutationEnvelope`
  (`internal/core/mutation_envelope.go:25-30`), but that default is only safe where the write
  primitive tags both classes. `EventWriter.AppendEvent` returns just `error` and `appendFast`
  discards the `fmt.Fprintf` byte count (`internal/events/stream.go:243`, `:290-291`), so pre-write
  status is unobservable and only a proven not-applied outcome may compensate.
* **Detection ships now, prevention is deferred** to active stash `47B48DB0`, which also absorbs the
  deliberation's `UpdateArtifactWithGate` minimum floor. The guarantee is therefore path-scoped to
  the governed `ShipShipment` archival path and must not be written as universal prevention.

## Designs proposed then rejected during review

These are the most valuable artifacts of the session: each was proposed by me and killed by evidence.

1. **Hoisting the item-log lock to the top of the `ShipShipment` locked closure.** Rejected in cycle
   2: assigning the locked context in place makes the ownership marker outlive the lock, so
   `restoreShipArtifacts` would rewrite an append-only log and archival would append with no
   exclusion; it also inverts the lock order against `lockArtifactMutations` (`:468`) and would hold a
   three-second-starving lock across two gate-broker evaluations whose timeout defaults to 600 s.
2. **Retrying the append before classifying.** Rejected: `AppendEvent` is explicitly not safe to
   blindly retry; retrying risks a duplicate shipped event in the log the plan protects.
3. **Nilling `releaseArtifactLocks` in its own defer.** Rejected: LIFO order already runs the release
   before the non-member-feature fallback, so refusing after release would make the fallback
   unconditionally dead and silently delete the 133.004-T guarantee. Replaced by swapping the two
   defer registrations.
4. **Real-filesystem injection in the RED harness.** Rejected: a directory planted at the log or lock
   path aborts in `snapshotShipArtifacts` (`:156-164`, called at `:478`) before the append at `:509`,
   so the scenarios could never fail for the right reason. Replaced by seam injection.
5. **An `internal/core` test proving CLI-versus-MCP parity.** Rejected: the package cannot reach
   either surface, so the claim would restate its premise.

## Gate record

| Gate | Result |
|---|---|
| Plan review cycle 1 | FAIL - P0=3, P1=16 across five personas |
| Plan review cycle 2 | FAIL - P0=2, P1=12 |
| Plan review cycle 3 | **PASS** - Concurrency P0=0 P1=0; Coupling P0=0 P1=0 |
| Adversarial review, 3 models on 3 families | **PASS** - HIGH-confidence P0/P1 = 0; 2 MEDIUM fixed; of 15 LOW findings, 13 fixed and 2 rejected with rationale |
| PR #366 Copilot review, cycle 1 | 9 comments, all accepted; see `docs/memory/2026-08-17/stage-pr366-review-cycle1-memory.md` |

## Validation performed

* `backlogit sync` - 1106 artifacts indexed, zero parse failures
* `backlogit docs lint` - `violation_count: 0`
* `backlogit doctor` - 23 findings, all pre-existing `106.0xx-T` orphans; **zero** findings mention
  `143`, `127-S`, or `059-DL`
* Dependency SQL - 15 `blocks` edges, exactly as designed
* Shipment manifest SQL - 12 members, covering feature first
* Stash SQL - `0115F71F` is `state=harvested` linked to `143-F`; `47B48DB0` is `state=active`
* JSONL integrity - `.backlogit/archive/stash.jsonl` grew by exactly one line, the harvested record;
  `7F0A6E89` and `6FA0829B` are still present and untouched
* `git diff --check` clean; no `.go`, `go.mod`, `go.sum`, workflow, Makefile, or linter config touched

## Incident during the session

A PowerShell array-splatting bug in a body-normalization helper emptied the template sections of the
eleven task files and of `127-S` after they were created. Recovery: the section text was still in the
SQLite index because no `sync` had run since creation, so descriptions were recovered from
`items.description` and the acceptance criteria and implementation notes were re-authored inline. All
thirteen artifacts were re-verified afterwards with `begin=3 end=3 empty=0` for every file. The
operator also declined two script executions on transparency grounds; the remaining writes were
performed with fully inline commands whose content is visible in the command itself.

## Open questions and next steps

* Ship claims shipment `127-S` and executes `143.001-T` first, or `143.005-T` - the two tracks are
  independent roots and may run in parallel.
* The freeze-scope path list is declared in the plan; any path outside it must return to Stage.
* Named closure follow-ups: a supported reconciliation transition out of `shipped`; durability of the
  item-level events inside `ShipShipment`; reconciliation of the two pre-existing drifted registry
  doctor params plus a repo-wide parity assertion; re-apply the P-007 and Ship-agent third branch
  after any autoharness template adoption.
* Stash `47B48DB0` stays active as the prevention complement.

## Preserved state

* `D:\Source\GitHub\backlogit\.worktrees\127-s-reconcile` was **not** touched. It holds a separate
  uncommitted implementation attempt and was preserved byte-for-byte.
* `origin/chore/stage-143-shipment-audit-log-reconciled` (`2188bab2`) was read as reference history
  only; no branch was created from it and none of its baseline claims were inherited.

---
title: "Shipment shipped-event audit-log durability and doctor reconciliation"
description: "Implementation plan rebaselined onto clean origin/main 3ec95ee3 for a durable shipped-event append on the active-to-shipped transition, class-aware rollback with all-or-nothing compensation, a report-only doctor reconciliation audit, and the surface and policy coherence those changes require"
source: ".backlogit/queue/059-DL.md"
doc_type: plan
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Source

* Deliberation: `.backlogit/queue/059-DL.md`
* Stash intake: `0115F71F` (kind `bug`, priority `medium`)
* Deferred prevention complement: stash `47B48DB0` (kind `task`, priority `low`, kept active)
* Baseline commit: `origin/main` at `3ec95ee3e7c6787762beb15c0e4b226746e50a89`
* Supersedes: the staging attempt on `origin/chore/stage-143-shipment-audit-log-reconciled`
  (`2188bab2`), which asserted a baseline the committed tree does not contain

## Baseline Verification

Every claim was read in a clean worktree at `3ec95ee3`. Line anchors were regenerated
mechanically after two review cycles found hand-written anchors drifting by a few lines.

| Claim | Evidence on `3ec95ee3` |
|---|---|
| An error-returning item-event appender exists | `appendItemEventErr` at `internal/core/gate_evidence.go:40`; `appendItemEventWithActorErr` at `:48` |
| That appender takes the item-log lock itself and returns untagged `fmt.Errorf` for both lock and append failure | `internal/core/gate_evidence.go:53-56` (lock) vs `:66-68` (append); neither carries a durability sentinel, and both mention "gate evidence" |
| A per-workspace append seam exists, but only for gate evidence | field `gateEvidenceAppend` at `internal/core/workspace.go:55-59`; dispatcher `(*Workspace).appendGateEvent` at `internal/core/gate_evidence.go:81-86` |
| No shipment-scoped append seam exists | ripgrep for `shipmentEventAppend` / `ShipmentEventAppend` across `internal/`, `cmd/`, `tests/` returns zero matches |
| No shipment append-error classification type exists | ripgrep for `shipmentEventAppendError` returns zero matches |
| The shipped-status path is still best-effort | `internal/core/shipment.go:205` calls `appendItemEvent`; implementation at `:660` / `:677` warns and returns on lock failure (`:690`) and append failure (`:696`) |
| Archival proceeds regardless of append outcome | `internal/core/shipment_lifecycle.go:509` returns the locked closure; `:515-527` then runs `collectArchiveCandidateIDs`, `attachCommitToItems`, `archiveItems` unconditionally |
| `snapshotShipArtifacts` runs BEFORE the transition, takes the item-log lock, and snapshots the log file | `internal/core/shipment_lifecycle.go:478` (call site), `:156-164` (lock and `snapshotFile`) |
| `restoreShipArtifacts` re-acquires the item-log lock per snapshot and SKIPS that item on lock failure | `internal/core/shipment_lifecycle.go:184-188` (`errs = append(...)`, `continue`) |
| The rollback defer short-circuits when no snapshots were taken | `internal/core/shipment_lifecycle.go:391-393` |
| Defer registration order makes the non-member-feature fallback unwind AFTER the artifact-lock release | fallback registered at `:365`, `releaseArtifactLocks` at `:375`; LIFO means release runs first |
| `ctx` is reassigned in place when artifact locks are taken, so the lock markers persist after release | `:468`; reentrancy short-circuit at `internal/core/shipment.go:372-378` |
| `rollbackIDs` covers shipment, release scope, feature roots, and descendants - but not cascade ancestors | `internal/core/shipment_lifecycle.go:454-466`; `cascadePersistedParentStatuses` recurses above the feature roots at `:983-1010` |
| `lockArtifactMutation` is non-blocking `TryLock` and fails fast with a busy error | `internal/core/shipment.go:747-752`; `internal/core/task_lock.go:67-80` |
| The two-class durability contract exists | `blerrors.ErrWriteNotApplied` / `ErrWriteIndeterminate` produced by `internal/atomicfile/atomicfile.go:82-129` |
| `MutationEnvelope` classifies an untagged plain error as `not-applied`, not indeterminate | `internal/core/mutation_envelope.go:25-30` (contract) and `:83-90` (implementation) |
| An append is NOT safe to blindly retry once it may have partially written | `internal/events/stream.go` `AppendEvent` / `appendDurable` doc contract: partial write or post-write fsync failure surfaces `ErrWriteIndeterminate` |
| Durable writes are opt-in and off by default, so the non-durable append path returns untagged errors | `internal/core/events_writer.go:12-22`; `internal/events/stream.go` `appendFast` |
| The item-log lock wait is bounded at 3 seconds, then fails | `internal/events/stream.go:40` (`itemLogLockWait`), `:177` (deadline) |
| `LockItemLogCrossProcess` short-circuits to a no-op when both markers are already in `ctx` | `internal/events/stream.go:210-220` |
| Multi-item item-log locks are taken in sorted order elsewhere | `lockAdoptionEventLogs` at `internal/core/shipment_lifecycle.go:1041-1058` (`sort.Strings` then nested acquisition) |
| Item logs are flat, one file per item | `internal/events/stream.go:91` `LogPathForItem`; `WorkspaceLogsRoot` at `internal/core/workspace.go:91-93` |
| `internal/core` tests already override package-global write seams | `internal/core/dependencies_indeterminate_test.go`, `archive_durable_write_test.go`, `artifact_size_durable_retry_test.go`; `persistArtifactWriteFn` read at `internal/core/shipment.go:762` |
| Existing ship rollback coverage lives in `shipment_test.go`; `shipment_lifecycle_test.go` does not exist | `TestShipShipment_RollsBackReleaseScopeWhenShipmentPersistFails` at `internal/core/shipment_test.go:397`; `TestShipShipment_RestoresNonMemberFeatureEvenWhenShipFailsAfterRollup` at `:512`; `TestShipShipment_RestoresNonMemberFeatureBeforePostShipHooksObserveIt` at `:892`; `TestRestoreShipArtifactsReadFailureLeavesEventLogUntouched` at `:40` |
| No transition out of `shipped` is permitted | `isValidShipmentTransition` at `internal/core/shipment.go:713-722` |
| `ShipShipment` refuses a non-active shipment | `internal/core/shipment_lifecycle.go:290-292` |
| A report-only advisory doctor finding precedent exists | `FindingOverArchivedCoveringFeature` at `internal/core/doctor.go:94`, option at `:186`, registration at `:522`, `refs`-direct consumption at `:523`, caveat wording at `:545` |
| The doctor check loop ranges over a doctor-local struct, not `artifactRef` | `internal/core/doctor.go:230-236`, `:253-260`; `artifactRef` at `internal/core/canonical_scan.go:21`, `scanCanonicalArtifacts` at `:38`, `parseFile` at `:88` |
| `models.Artifact.ArchivedStatus` already exists | `internal/models/artifact.go:70` |
| The workspace routing registry lists neither `shipped` in its archive nor its queue status set | `.backlogit/registry.yaml`: archive routes `done\|accepted\|rejected\|archived`; queue routes `queued\|active\|blocked\|review` |
| A read-only doctor check IS already MCP-exposed, with contract tests | `check_partial_mutations` at `internal/mcp/tools.go:484`, `check_workspace_root_conflict` at `:485`; tests `internal/mcp/doctor_partial_mutations_test.go:28`, `doctor_workspace_root_conflict_test.go:30` |
| Gate-evidence is the CLI-only doctor check and is NOT a valid MCP precedent | `--check-gate-evidence` var at `internal/cli/doctor.go:33`, wiring at `:129`, registration at `:167`; absent from `internal/mcp/tools.go:480-487` |
| The registry `doctor.params` block is already drifted, omitting two live params | `.autoharness/backlog-registry.yaml` `doctor:` declares only `check_orphans`, `check_duplicates`, `fix_orphans`, `target` |
| MCP recovery guidance is keyed on class ONLY and is shared by three producers | `mutationPartialRecovery(classification string)` at `internal/mcp/errors.go:165-176`; `mutationPartialError` at `:147` (with `FailedStep` already in scope at `:153`,`:156`); reached from `domainError` (`:116-119`), `handleTrackCommit` (`internal/mcp/tools.go:1595`, dispatch `:1613-1616`), `checkpointDispositionError` (`internal/mcp/errors.go:229`, dispatch `:230-233`) |
| An existing test pins the shared guidance text for a different producer | `internal/mcp/error_mapping_test.go:200` asserts `resp.Recovery` contains `check_partial_mutations`, for `FailedStep: "jsonl-append"` |
| `gateErrorResult` runs first in `handleShipShipment` but cannot match `*MutationPartialError` | `internal/mcp/tools.go` `handleShipShipment` calls `gateErrorResult` then `domainError`; `gateErrorResult` matches only gate types (`internal/mcp/gate_errors.go:22-38`) |
| `internal/cli` imports `internal/mcp`, so only a `cli`-package test can compare both surfaces | `internal/cli/registry_parity_test.go` imports `internal/mcp`; the reverse import is forbidden by `docs/ARCHITECTURE.md` |
| CI enforces regenerated CLI reference docs for any `**/*.go` change | `.github/workflows/ci.yml` job `cli-reference-drift` at `:152-182`; the `cli_reference` paths filter is `:73-86` with `'**/*.go'` at `:79`; target page `docs/cli-reference/backlogit_doctor.md` |
| A design doc enumerates the governed recovery contract and doctor recovery checks | `docs/design-docs/governed-mutation-recovery-contract.md` |
| Policy, skill, AND the Ship agent all encode "ship always archives" | `.github/policies/workflow-policies.md` P-007 postcondition and violation action; `.github/skills/shipment-reconcile/SKILL.md` post-mode; `.github/agents/.ship.agent.md:521` (§1.b), `:533` (§1.c), `:537` (§1.d) |
| `workflow-policies.md` and `.ship.agent.md` are deliberately NOT drift-ignored | `.autoharness/drift-ignore` header note; `shipment-reconcile/SKILL.md` and `backlogit.instructions.md` ARE listed |

## Problem Frame

### Committed behavior on the clean baseline

`ShipShipment` completes the `active -> shipped` transition inside its locked closure by
calling `moveShipmentStatusWithHeadGuard` (`internal/core/shipment_lifecycle.go:509`).
That function persists the new status through `persistArtifactWithGuard`
(`internal/core/shipment.go:200`) and only afterwards emits the audit record:

```text
internal/core/shipment.go:205
    appendItemEvent(ctx, ws, shipmentID, "shipment_status_changed", map[string]any{
        "status": string(newStatus),
    })
```

`appendItemEvent` is fire-and-forget. Its implementation (`:660` delegating to `:677`)
warns and returns on lock failure (`:690`) and on append failure (`:696`).

Because the error never reaches the return value, the closure returns `nil`, its rollback
defer (`:390-398`) does not fire, and `ShipShipment` continues into
`collectArchiveCandidateIDs` / `attachCommitToItems` / `archiveItems` (`:515-527`).
`ArchiveItem` then stamps `archived_status` (`internal/core/archive.go:223`).

The end state is a shipment persisted with `archived_status: shipped` whose append-only item
JSONL contains no `shipment_status_changed` event with `status == shipped` - exactly the
field observation from autoharness stash `84D8E6AB` / shipment `114-S`.

### Two distinct residue states

* `missing_shipped_event` - archived, `archived_status: shipped`, no shipped event in the
  item JSONL. Terminal, permanently absent audit record.
* `shipped_unarchived_residue` - `status: shipped` but not archived. This is the residue the
  new indeterminate branch deliberately leaves, and it is anomalous regardless of whether the
  shipped event is present.

`.backlogit/registry.yaml` routes neither status set to `shipped`, so the resolved location of
a shipped-and-unarchived shipment is not assumed by this plan: the audit scans queue **and**
archive, and the harness asserts the actually-resolved path rather than a presumed one.

### Escalation path between the two states

A `shipped`-and-unarchived shipment whose release scope is already `done` is exactly the input
a later generic archive turns into `archived_status: shipped` with no shipped event, converting
the easily-detected residue into the hard-to-detect one. The recovery guidance therefore carries
an explicit "reconcile before archiving" instruction.

### Scope of the durability guarantee

Only the **shipment's own terminal status transition** becomes durability-gated. The same call
also appends best-effort item-level events - `status_changed` via `setArtifactStatus`
(`internal/core/shipment_lifecycle.go:966`), `returned_to_backlog` (`:611`), and parent cascades
(`:1010`). Those stay best-effort by design and are **not** covered by stash `47B48DB0`, which
is scoped to non-`ShipShipment` producers. The gap is named here, restated in the code doc
comment required by Unit 4, and raised as a closure follow-up.

### What this plan does not do

* It does not close non-`ShipShipment` paths that can reach `archived_status: shipped`. That is
  **prevention**, deferred to active stash `47B48DB0`, which also carries the deliberation's
  minimum floor (verify `UpdateArtifactWithGate` cannot drive a shipment to `shipped` bypassing
  the durable event).
* It does not add a forward reconciliation transition out of `shipped`.
* It does not write a `.backlogit/reconcile/` sidecar. See Plan Hardening.

### Named limitation: no forward transition out of `shipped`

`isValidShipmentTransition` (`internal/core/shipment.go:713-722`) permits no transition out of
`shipped`, and `ShipShipment` refuses a non-active shipment (`:290-292`). The
`shipped_unarchived_residue` this plan can create therefore has **no automated forward
recovery**. That is declared, not accidental. Supported operator procedure:

1. Run the doctor audit; the finding reports the shipment ID, its resolved artifact path, and the
   release-scope items left terminal-but-unarchived alongside it.
2. Read `.backlogit/logs/{shipment-id}.jsonl` and determine whether the
   `shipment_status_changed: shipped` event actually landed.
3. If it landed, the residue is archival-only: complete archival with
   `backlogit archive {shipment-id}` **and** `backlogit archive` for each release-scope item the
   finding enumerates, then re-run the audit to confirm the finding clears. The indeterminate branch
   returns before `collectArchiveCandidateIDs` (`internal/core/shipment_lifecycle.go:515`), so the
   whole archive set is stranded, not the shipment alone.
4. If it did not land, the audit-log gap is permanent. Record it. **Never synthesize the event.**

Two failure modes are outside this contract and are declared rather than handled. A process crash
between the status persist (`internal/core/shipment.go:200`) and the append (`:205`) produces the
same residue with no `MutationPartialError` and no `slog` record; only SLI 2 detects it. A
malformed partial JSONL tail is warned-and-skipped by the event reader, so the audit will report
the event as absent even if bytes exist. Both are detected by the doctor audit and reconciled by
the procedure above.

A supported `shipped -> active` reconciliation transition is out of scope and is a named closure
follow-up.

## Requirements Trace

`Source` distinguishes stash-derived requirements from plan-added ones. Every plan-added row
carries an explicit accept/reject decision.

| Requirement | Source | Implementation action | Unit |
|---|---|---|---|
| Integrate the shipped-event append into the shipment mutation/rollback envelope using an error-returning writer; archival must not continue without that durable event | stash `0115F71F` (1) | Shipment-scoped error-returning appender (Unit 1); fail-closed routing gated on `ShipmentShipped` (Unit 3) | 1, 3 |
| Restore active/unarchived shipment and release-scope state on append failure | stash `0115F71F` (2) | `ErrWriteNotApplied` and untagged append errors compensate through the existing snapshot rollback | 4 |
| Or return an explicit indeterminate reconciliation error | stash `0115F71F` (2) | Only `blerrors.IsWriteIndeterminate` suppresses rollback and returns `*blerrors.MutationPartialError{Class:"indeterminate"}` | 4 |
| Test success ordering and injected append failure across the shared core path | stash `0115F71F` (3) | RED harness injecting through the per-workspace seam | 2 |
| Report-only doctor audit for archived shipments missing the shipped event; never synthesize or rewrite historical JSONL | stash `0115F71F` (4) | Declarations (Unit 5), RED harness (Unit 6), canonical queue-plus-archive audit (Unit 7) | 5, 6, 7 |
| Compensation must be all-or-nothing, or must honestly report partial compensation | **plan-added, accepted - correctness of stash requirement (2)** - `restoreShipArtifacts` silently `continue`s past an item whose log lock it cannot re-acquire (`internal/core/shipment_lifecycle.go:184-188`), so "restore state" would otherwise be a promise the code does not keep | Bounded retry on re-acquisition, then `CompensationState: "partially-compensated"` naming un-restored IDs | 4 |
| The non-member covering-feature restore must survive post-closure failures | **plan-added, accepted - pre-existing hazard the plan would otherwise worsen** - LIFO defer order (`:365` vs `:375`) already runs the fallback after the artifact locks are released, and Unit 4 depends on that fallback being live | Swap the two defer registrations so the release unwinds last | 4 |
| Second finding `shipped_unarchived_residue` and a queue-plus-archive scan | **plan-added, accepted** - the indeterminate branch creates exactly this residue; detecting only the archived form would fail to report the plan's own output | Two distinct finding types | 5, 7 |
| CLI surface for the audit | **plan-added, accepted** - requirement (4) asks for an audit an operator can run; the named-limitation procedure, SLI 1, SLI 2, and the pre-deploy checklist are all unexecutable without it | `--check-shipped-event-completeness` plus the CI-required regenerated reference doc | 8 |
| MCP parameter for the audit | **plan-added, accepted** - Unit 4 returns `mutation_partial` to `backlogit_ship_shipment` MCP callers and Unit 10 tells that caller to run the audit; an MCP-only agent cannot invoke a CLI flag | MCP parameter, doctor registry row, MCP contract test | 9 |
| Producer-scoped recovery guidance | **plan-added, accepted** - without it the new residue ships with guidance (`check_partial_mutations`) that provably cannot detect it | Key guidance on `FailedStep`, not on `Class` | 10 |
| Contract, policy, skill, and Ship-agent coherence | **plan-added, accepted - required for correctness** - Unit 4 breaks the P-007 "ship always archives" postcondition, so stale policy, the reconcile skill, and the Ship agent would instruct `git restore` and mask the reconciliation | Update the governed-recovery contract doc, P-007, the reconcile skill, the Ship agent, and the drift-ignore record | 11 |
| Integration/regression unit proving CLI and MCP parity from `internal/core` | **plan-added, REJECTED at review** - an `internal/core` test cannot reach `internal/cli` or `internal/mcp`, so the parity claim would restate its premise | Deleted; surface behavior is proven by surface-local tests | - |
| Retry/backoff on item-log lock contention at the append site | **plan-added, REJECTED at review** - an append is not safe to blindly retry (`internal/events/stream.go` `AppendEvent` contract), and a pre-append lock failure is already correctly classified `not-applied` and compensates | Deleted | - |
| Hoisting the shipment item-log lock to the top of the locked closure | **plan-added, REJECTED at review** - it makes the marker in `ctx` outlive the lock, exposes archival appends and the rollback log rewrite to lock-free execution, inverts the lock order against `lockArtifactMutations`, and extends a 3-second-starving lock across two gate-broker evaluations | Deleted | - |
| Repo-wide registry `params`-to-`InputSchema` parity assertion and reconciliation of two pre-existing drifted params | **plan-added, DEFERRED** - a new global invariant over roughly forty registry operations whose current pass/fail state this plan has not verified; unrelated to this defect | Named closure follow-up | - |

## Implementation Units

Eleven units map one-to-one onto tasks `143.001-T` through `143.011-T`. Both core tracks open
with a purely additive scaffold, then a test-only RED harness, then implementation.

### Unit 1 - Shipment-scoped append seam and error-returning appender (`143.001-T`)

* Posture: **characterization-first**. Additive plus one behavior-preserving call-site change.
* Changes
  1. Add the per-workspace seam field `shipmentEventAppend` to `Workspace`, mirroring
     `gateEvidenceAppend` (`internal/core/workspace.go:55-59`).
  2. Add `appendShipmentEventErr`: a shipment-scoped, error-returning appender that acquires the
     item-log lock itself so a lock failure and an append failure arise in two distinguishable
     statements, and so the message reads "shipment event log", not "gate evidence". Tag the lock
     failure `fmt.Errorf("lock shipment event log %s: %w: %w", itemID, blerrors.ErrWriteNotApplied, lockErr)` -
     nothing was written, so `not-applied` is the honest class and compensation is safe.
     Tag a **short or partial write** `blerrors.ErrWriteIndeterminate`. The default non-durable
     path (`internal/events/stream.go` `appendFast`) writes with `fmt.Fprintf`, which returns an
     untagged error on a short write (for example a full disk) after bytes have already reached
     the file. Classifying that as `not-applied` would compensate over a log that was in fact
     partially written, which the `AppendEvent` contract forbids.
  3. Add the dispatcher `(*Workspace).appendShipmentEvent`, mirroring `appendGateEvent`.
     Also declare the private boundary type `shipmentEventAppendError` here, so the type exists in
     the same package as the call site that wraps it (Unit 3) and the closure that classifies it
     (Unit 4). Declaring it in Unit 4 would be unimplementable: the wrap happens at
     `internal/core/shipment.go:205`, which Unit 4 does not touch.
  4. Route the `shipment_status_changed` call site (`internal/core/shipment.go:205`) through the
     dispatcher while **preserving best-effort semantics**:
     `if appendErr := ws.appendShipmentEvent(...); appendErr != nil { slog.WarnContext(...) }`.
     Behavior is unchanged in this unit.
  5. Declare the failed-step name once as an exported constant beside `MutationPartialError` in
     `internal/errors` (`StepShippedEventAppend = "shipped-event-append"`) so core and MCP share
     one literal.
* Files: `internal/core/workspace.go`, `internal/core/shipment.go`, `internal/errors/` (one const)
* Verification: run the named selectors, not the whole package - the sibling track's harness
  (`143.006-T`) may legitimately be red in `internal/core` at the same time:
  `go test ./internal/core/ -run 'TestShipShipment|TestClaimShipment|TestRestoreShipArtifacts'`
  green with no test edits, specifically
  `TestShipShipment_RollsBackReleaseScopeWhenShipmentPersistFails`,
  `TestShipShipment_RestoresNonMemberFeatureEvenWhenShipFailsAfterRollup`,
  `TestShipShipment_RestoresNonMemberFeatureBeforePostShipHooksObserveIt`, and
  `TestRestoreShipArtifactsReadFailureLeavesEventLogUntouched`.

### Unit 2 - RED harness for shipped-event durability (`143.002-T`)

* Posture: **test-first (red phase)**. Test-only; no production file.
* Injection: through the per-workspace `ws.shipmentEventAppend` seam from Unit 1. Real-filesystem
  injection was evaluated and **rejected**: a directory planted at the item log or lock path aborts
  earlier in `snapshotShipArtifacts` (`internal/core/shipment_lifecycle.go:156-164`, called at
  `:478`, before the transition at `:509`), leaving `shipSnapshots` empty so the rollback defer
  short-circuits at `:391` - the scenarios could never fail for the right reason.
* Files: `internal/core/shipment_shipped_event_durability_test.go` (new, test-only)
* Scenarios (3, each a table-driven `t.Run` group)
  1. **Success ordering** - `active -> shipped -> archived` persists AND the
     `shipment_status_changed` event with `status == shipped` is present in
     `.backlogit/logs/{shipment-id}.jsonl` AND ordered before the archival records.
  2. **NotApplied and untagged compensate, honestly** - subtests: a seam error wrapping
     `blerrors.ErrWriteNotApplied`; a bare untagged `errors.New`. Each returns non-nil, restores the
     shipment to `active`, leaves the release scope not completed, and archives nothing. A third
     subtest makes one release-scope item's log lock unavailable during compensation and asserts the
     result is `*blerrors.MutationPartialError{Class: "not-applied", CompensationState:
     "partially-compensated"}` naming the un-restored ID - never a silently skipped item.
  3. **Indeterminate never rolls back** - a seam error wrapping `blerrors.ErrWriteIndeterminate`,
     using a fixture whose ship would roll up a non-member covering feature: the release scope is NOT
     rolled back, the covering feature is restored to its pre-ship status (re-read through
     `loadArtifact`, not the in-memory value), archival is halted, the shipment is left
     shipped-and-unarchived, and the error satisfies `errors.As(&*blerrors.MutationPartialError)`
     with `Class == "indeterminate"` and `FailedStep == blerrors.StepShippedEventAppend`.
* Constraints
  * MUST NOT call `t.Parallel()`: `ShipShipment` reads the package globals `persistArtifactWriteFn`
    (`internal/core/shipment.go:762`) and `mkdirDirSyncFn`, which other `internal/core` tests override.
  * Every fixture MUST pin `Config.DurableWrites` explicitly so classification never depends
    silently on workspace configuration.
* Verification: all three scenarios compile on the Unit 1 tree. Scenarios 2 and 3 MUST fail there,
  and must fail from the ship path - assert the failure is not `"snapshot release scope"`.
  Scenario 1 characterizes behavior that is already correct on the baseline (the event is appended
  at `internal/core/shipment.go:205` before archival) and is expected to pass; it is present to lock
  the ordering contract against regression, not to establish the red state.

### Unit 3 - Fail-closed shipped routing (`143.003-T`)

* Posture: **implementation**. Depends on Unit 2 being red.
* Changes
  1. In `moveShipmentStatusWithHeadGuard`, gate on `newStatus == ShipmentShipped`: return the append
     error instead of only warning. Every other transition (`claim`, `abandon`) keeps the existing
     best-effort call and semantics.
  2. Wrap the returned append failure in the `shipmentEventAppendError` boundary type declared in
     Unit 1.3, at the call site, so the closure in Unit 4 classifies only that error.
  3. Do **not** retry the append. `AppendEvent`'s contract is that a partial write or post-write
     fsync failure is `ErrWriteIndeterminate` and is not safe to retry blindly; a pre-append lock
     failure is already tagged `not-applied` by Unit 1.2 and compensates. There is no third outcome
     class.
  4. Record in the doc comment that the fail-closed shipped path intentionally suppresses the
     `HookMoveShipmentStatus` post-hook, which today always fires because the append error is
     swallowed. Firing a "status changed to shipped" post-hook for a transition that is about to be
     compensated would misinform external integrations.
* Files: `internal/core/shipment.go`
* Verification: Unit 2 scenario 1 green; scenarios 2 and 3 now fail on classification assertions
  rather than on "no error returned". `TestClaimShipment_ActivatesIncludedScope`
  (`internal/core/shipment_test.go:136`), `TestClaimShipment_RollsBackOnMidFlightActivationFailure`
  (`internal/core/shipment_state_integrity_test.go:28`), and `TestClaimShipment_SuccessActivatesAllItems`
  (`:84`) stay green unchanged, proving non-shipped transitions are untouched.

### Unit 4 - Class-aware rollback, honest compensation, and defer ordering (`143.004-T`)

* Posture: **implementation**. Depends on Unit 3.
* Changes
  1. Classify only the `shipmentEventAppendError` boundary value declared in Unit 1.3 and wrapped
     in Unit 3.2. Every other closure error - including untagged pre-append failures from
     `completeReleaseScope`, `returnUnreleasedFeatureItems`, and the cascades - keeps the existing
     unconditional rollback.
  2. Classification, matching the precedence `MutationEnvelope` already uses
     (`internal/core/mutation_envelope.go:25-30`, `:83-90`): `blerrors.IsWriteIndeterminate`
     suppresses rollback; **everything else, including untagged plain errors, compensates**.
     Treating untagged errors as indeterminate was evaluated and rejected: it contradicts the sibling
     policy in the same package, and because durable writes are off by default a definitively
     not-applied `open` failure would be misclassified and strand the shipment permanently.
  3. Place the classification branch and the `restoreRolledUpNonMemberFeatures` call **before** the
     existing `if closureErr == nil || len(shipSnapshots) == 0 { return }` guard
     (`internal/core/shipment_lifecycle.go:391-393`). Only the `restoreShipArtifacts` call stays
     behind that guard; placing the new branch after it would make it dead code whenever snapshotting
     did not populate the map.
  4. Make compensation honest rather than silently partial. In `restoreShipArtifacts`
     (`:184-188`), apply a **per-call** bounded retry budget to the per-item log-lock
     re-acquisition; if an item still cannot be restored, do not merely `continue` - promote the
     outcome to `*blerrors.MutationPartialError{Class: "not-applied", CompensationState:
     "partially-compensated"}` listing the un-restored IDs. This closes a real torn-state hazard:
     today an item-log lock failure for any release-scope item skips both the file restore and the
     DB upsert while the shipment reverts to `active`. Apply the same promotion to **every**
     rollback stage that can fail per item, not only lock acquisition - the read, the file restore,
     the log restore, the event replay, the reindex, and the DB upsert
     (`internal/core/shipment_lifecycle.go:194`, `:207`, `:211`, `:216`, `:220`, `:225`). The retry
     budget is per call, not per item, because this loop runs inside the closure's rollback defer
     with the membership lock and all artifact locks held.
  5. On the indeterminate branch: suppress `restoreShipArtifacts`, synchronously call
     `restoreRolledUpNonMemberFeatures` while the artifact locks are still held, halt archival, and
     return `*blerrors.MutationPartialError{Class: "indeterminate", FailedStep:
     blerrors.StepShippedEventAppend, CompensationState: "not-compensated", Cause: err}` wrapping with
     `%w` and populating `Completed` with the steps that did land. Join any restore failure onto the
     returned error; indeterminate dominates.
  6. Track `restoreAttempted` and `restoreSucceeded` separately instead of a single `restored` flag,
     and allow the outer fallback exactly one retry when the in-lock attempt failed. A single
     transient busy-lock on a cascade ancestor (which `lockArtifactMutation` reports immediately via
     non-blocking `TryLock`, `internal/core/task_lock.go:67-80`) must not permanently abandon the
     compensation.
  7. **Swap the two defer registrations** at `internal/core/shipment_lifecycle.go:365` and `:375`:
     register `releaseArtifactLocks` **first** so it unwinds **last**, and the non-member-feature
     fallback second so it unwinds first, with the artifact locks genuinely held and the `ctx`
     markers truthful. Today LIFO runs the release before the fallback, so the fallback performs
     status writes and file relocations with a `ctx` that falsely asserts the locks are held
     (`ctx` is reassigned in place at `:468`). Nilling the releaser was evaluated and **rejected**:
     it would make the fallback unconditionally dead and silently delete the 133.004-T guarantee for
     the `collectArchiveCandidateIDs` (`:515`), `attachCommitToItems` (`:519`), and `archiveItems`
     (`:523`) failure paths.
  8. Emit `slog` on both branches with a fixed, greppable shape:
     `slog.ErrorContext(ctx, "shipment shipped-event append failed", "shipment_id", id, "class",
     "indeterminate"|"not-applied", "failed_step", blerrors.StepShippedEventAppend,
     "compensation_state", state, "unrestored_ids", ids, "error", err)`. `compensation_state` and
     `unrestored_ids` are required because `MutationPartialError.Error()` does not render
     `CompensationState`, so the log is the only non-MCP measurement surface for SLI 5.
  9. Add the doc comment recording the invariant: the durability guarantee covers the shipment's own
     terminal transition only; item-level events inside the same call remain best-effort.
* Files: `internal/core/shipment_lifecycle.go`
* Verification: all three Unit 2 scenarios green; the four existing ship tests named in Unit 1 still
  green unchanged; plus a new regression assertion that a post-closure `archiveItems` failure still
  restores the non-member covering feature (the path the defer swap protects).

### Unit 5 - Doctor finding types, option, and inert registration (`143.005-T`)

* Posture: **scaffold**, purely additive. Exists so the Unit 6 harness can compile without importing
  production behavior into a test-only task - the compile-order cycle a two-unit split would create.
* Changes
  * Add `FindingMissingShippedEvent DoctorFindingType = "missing_shipped_event"` and
    `FindingShippedUnarchivedResidue DoctorFindingType = "shipped_unarchived_residue"`.
  * Add `DoctorOptions.CheckShippedEventCompleteness bool`, off by default.
  * Register an inert check beside the `opts.CheckOverArchivedFeatures` block
    (`internal/core/doctor.go:522`) that produces no findings yet and does not touch the exit code.
* Files: `internal/core/doctor.go`
* Verification: run the named selector, not the whole package - the sibling track's harness
  (`143.002-T`) may legitimately be red in `internal/core` at the same time:
  `go test ./internal/core/ -run TestDoctor` green with no behavior change and no test edits.

### Unit 6 - RED harness for the doctor reconciliation audit (`143.006-T`)

* Posture: **test-first (red phase)**. Test-only; no production file. Depends on Unit 5.
* Files: `internal/core/doctor_shipped_event_test.go` (new, test-only)
* Scenarios (3)
  1. An archived shipment with `archived_status: shipped` and no shipped event yields exactly one
     `missing_shipped_event` finding; the same fixture with the shipped event present yields none;
     and an archived shipment whose `archived_status` is **not** `shipped` (for example `abandoned`)
     yields none, proving the check filters on `archived_status` rather than on "archived".
  2. A `status: shipped`, unarchived shipment yields exactly one `shipped_unarchived_residue`
     finding whose detail records whether the shipped event is present and enumerates the
     release-scope items left `done`-and-unarchived alongside it. The scenario asserts the
     **actually resolved** artifact path produced by the routing registry for the `shipped` status
     rather than assuming `queue/`.
  3. Running the audit leaves every file under `.backlogit/logs/` byte-identical (recursive compare,
     `fix_orphans` off). Doctor **exit-code** neutrality is asserted in Unit 8, because `core.Doctor`
     returns a report and an error, not an exit code - the exit mapping lives in
     `internal/cli/doctor.go`.
* Verification: all three compile against Unit 5's declarations and fail because the inert check
  produces no findings.

### Unit 7 - Report-only doctor reconciliation audit (`143.007-T`)

* Posture: **implementation**. Depends on Unit 6.
* Changes
  * Replace the inert registration with the real check. The check MUST read `archived_status`, not
    just `status`: `ArchiveItem` sets `status: archived` and stores the pre-archive status in
    `archived_status` (`internal/core/archive.go:223`), so an archived shipped shipment presents as
    `status: archived`. Neither `artifactRef` (`internal/core/canonical_scan.go:21`) nor the
    doctor-local struct (`internal/core/doctor.go:230-236`, `:253-260`) carries `archived_status`
    today, so **both** must be extended, or the check must re-read the shipment frontmatter from the
    exact path already in `refs`. Without that filter the audit false-positives on shipments
    archived from `abandoned` or any other status. Widening `artifactRef` is not optional; it is
    shared with the create-time uniqueness guard, so the added field must be additive and unused
    elsewhere.
  * Scan queue and archive.
  * For `shipped_unarchived_residue`, also enumerate the shipment's release-scope items that were
    left terminal-but-unarchived by the halted archival, because the indeterminate branch returns
    before `collectArchiveCandidateIDs` (`internal/core/shipment_lifecycle.go:515`) and therefore
    strands the whole archive set, not the shipment alone.
  * Carry the "verify the actual history; this can be transient during an in-flight ship" caveat
    wording from `internal/core/doctor.go:545`.
  * Never write, synthesize, or rewrite JSONL.
* Files: `internal/core/doctor.go`, `internal/core/canonical_scan.go`
* Verification: all three Unit 6 scenarios green.

### Unit 8 - CLI surface and regenerated reference documentation (`143.008-T`)

* Posture: **test-first**. Depends on Unit 7.
* Changes
  * Add `--check-shipped-event-completeness` wired to `DoctorOptions.CheckShippedEventCompleteness`,
    following the field/wiring/registration pattern at `internal/cli/doctor.go:34`, `:130`, `:168`.
  * Regenerate `docs/cli-reference/backlogit_doctor.md` with `make docs`
    (`go run ./cmd/gen-docs docs/cli-reference`) and commit it in the same commit. CI job
    `cli-reference-drift` (`.github/workflows/ci.yml:152-182`) runs `git diff --exit-code
    docs/cli-reference/` and its paths filter (`:73-86`, `'**/*.go'` at `:79`) fires for every unit
    in this plan. Never hand-edit the generated file.
  * No registry change here. A transient `cli_only_flags` entry was evaluated and **rejected**: every
    existing entry carries `human_terminal_only: true` and means deliberate permanent asymmetry, so
    reusing it for a one-task gap would be read as a permanent operator-only exclusion.
* Files: `internal/cli/doctor.go`, `internal/cli/doctor_test.go`,
  `docs/cli-reference/backlogit_doctor.md` (generated)
* Scenarios (2): the flag enables the check and surfaces both finding types for a seeded fixture;
  defaults are unchanged when the flag is absent.
* Verification: `go test ./internal/cli/...` green and `git diff --exit-code docs/cli-reference/`
  clean after `make docs`.

### Unit 9 - MCP surface and doctor registry row (`143.009-T`)

* Posture: **test-first**. Depends on Unit 7 and Unit 8 (so the CLI flag lands first and the
  asymmetry window closes forward, not backward).
* Rationale for MCP exposure: `check_partial_mutations` (`internal/mcp/tools.go:484`) and
  `check_workspace_root_conflict` (`:485`) are the verified precedent - read-only doctor checks with
  dedicated MCP contract tests. Do **not** cite `CheckGateEvidence`; that check is CLI-only
  (`internal/cli/doctor.go:33`, `:129`, `:167`).
* Changes
  * Add the `check_shipped_event_completeness` boolean parameter to `backlogit_doctor` and wire it in
    `handleDoctor`.
  * Add `check_shipped_event_completeness` to the `doctor` operation `params` in
    `.autoharness/backlog-registry.yaml`. Reconciling the two pre-existing drifted params and adding
    a repo-wide parity invariant are **deferred** - see the Requirements Trace and the closure
    follow-ups.
* Files: `internal/mcp/tools.go`, `.autoharness/backlog-registry.yaml`, plus an `internal/mcp`
  contract test mirroring `internal/mcp/doctor_partial_mutations_test.go`, plus the CLI-versus-MCP
  fixture added to `internal/cli/registry_parity_test.go`
* Scenarios (3): the parameter is advertised and enables the check; MCP-side findings render for a
  seeded fixture; a shared fixture yields identical findings through CLI and MCP. The third
  assertion MUST live in `internal/cli/registry_parity_test.go` - the only package that sees both
  surfaces, since `internal/mcp` cannot import `internal/cli`. That file currently asserts registry
  mapping and governed-mutation parity only, so this adds a new fixture rather than extending an
  existing one.

### Unit 10 - Producer-scoped recovery guidance (`143.010-T`)

* Posture: **test-first**. Depends on Unit 4 (the error must exist) and Unit 9 (the MCP parameter
  named in the guidance must exist).
* Changes
  * `mutationPartialRecovery` (`internal/mcp/errors.go:165-176`) is keyed on `Class` **only** and is
    reached from `domainError` (`:116-119`), `handleTrackCommit` (`internal/mcp/tools.go:1595`,
    dispatch `:1613-1616`), and `checkpointDispositionError` (`internal/mcp/errors.go:229`, dispatch
    `:230-233`). Repointing the `"indeterminate"` case would mis-advise all three. Instead pass
    `err.FailedStep` - already in scope at `internal/mcp/errors.go:153`, `:156` - and branch on
    `blerrors.StepShippedEventAppend`, leaving the class-only default text untouched.
  * The shipped-event guidance must be **dual-surface**, naming both
    `check_shipped_event_completeness` (MCP) and `--check-shipped-event-completeness` (CLI), and must
    carry the "reconcile before archiving" instruction.
  * Correct the structured `Retryable` field alongside the text. `internal/mcp/errors.go:155` sets
    `Retryable: err.Class == "not-applied"`, so Unit 4.4's new
    `Class: "not-applied", CompensationState: "partially-compensated"` result would advertise
    itself as safe to retry while release-scope items remain un-restored. Gate `Retryable` on class
    **and** compensation state so a partially-compensated result is never retryable.
  * Keep the single dispatch site in `handleShipShipment`. `gateErrorResult` runs first but cannot
    match `*MutationPartialError` (`internal/mcp/gate_errors.go:22-38`), so no collision exists today;
    add a regression guard asserting that rather than restructuring the dispatch.
* Files: `internal/mcp/errors.go`, plus an `internal/mcp` contract test
* Scenarios (3): the ship-path indeterminate result names both tokens and preserves `Class` and
  `FailedStep`; `internal/mcp/error_mapping_test.go:200`'s existing `track_commit` assertion still
  passes; the indeterminate result is not consumed by `gateErrorResult`.

### Unit 11 - Contract, policy, skill, and Ship-agent coherence (`143.011-T`)

* Posture: **documentation and policy only**. Depends on Units 4, 7, 8, 9, and 10.
* Changes
  * `docs/design-docs/governed-mutation-recovery-contract.md`: add the shipment ship path as a
    governed `MutationPartialError` producer; add `CheckShippedEventCompleteness` and both finding
    names to the recovery-discovery enumeration; document `FailedStep` (via the shared constant),
    `CompensationState: "not-compensated"` and `"partially-compensated"`, and the producer-scoped
    `recovery` semantics.
  * `.github/policies/workflow-policies.md` P-007: add a third branch. When
    `backlogit_ship_shipment` returned `mutation_partial` with `classification: indeterminate` and
    `failed_step: shipped-event-append`, do **not** run `git restore .backlogit/archive/`; run the
    doctor audit and reconcile per the named-limitation procedure.
  * `.github/skills/shipment-reconcile/SKILL.md` post-mode: the same third branch, so the gate does
    not emit `HALT - restore archives` for an intended outcome.
  * `.github/agents/.ship.agent.md` Step 6 §1.b, §1.c, §1.d (`:521`, `:533`, `:537`): the same third
    branch. This is the surface that actually executes the sequence the policy and skill describe;
    leaving it stale would have the Ship agent `git restore` over the intended residue.
  * `.autoharness/drift-ignore`: record `.github/policies/workflow-policies.md` and
    `.github/agents/.ship.agent.md` as intentional local customizations carrying the shipped-event
    indeterminate branch, and amend that file's header NOTE, which currently enumerates both paths
    as "deliberately NOT ignored" for an open autoharness capability decision. Drift-ignore
    suppresses drift *reporting*; it does not by itself prevent a later template adoption from
    overwriting the third branch, so also add "re-apply the P-007 / Ship-agent third branch after
    any autoharness template adoption" to the closure follow-ups.
* Files: the five listed above
* Verification: `backlogit docs lint` clean; markdownlint clean; no code change.

## Dependency Graph

The edge list is **authoritative**. No diagram is provided, deliberately.

| Task | Depends on |
|---|---|
| `143.001-T` | - (root) |
| `143.002-T` | `143.001-T` |
| `143.003-T` | `143.002-T` |
| `143.004-T` | `143.003-T` |
| `143.005-T` | - (root) |
| `143.006-T` | `143.005-T` |
| `143.007-T` | `143.006-T` |
| `143.008-T` | `143.007-T` |
| `143.009-T` | `143.007-T`, `143.008-T` |
| `143.010-T` | `143.004-T`, `143.009-T` |
| `143.011-T` | `143.004-T`, `143.007-T`, `143.008-T`, `143.009-T`, `143.010-T` |

All edges are typed `blocks`. The graph is acyclic with two roots (`143.001-T`, `143.005-T`) and
one sink (`143.011-T`). Red-before-green is structural on both core tracks: `143.003-T` cannot
start before harness `143.002-T`, and `143.007-T` cannot start before harness `143.006-T`. Both
harnesses are preceded by an additive scaffold so neither harness carries production code and
neither pair forms a compile-order cycle. No external dependency blocks Stage harvest or Ship
execution.

## Decisions and Rationale

* **Red before green is enforced by the graph, on both tracks**, with an additive scaffold in front
  of each harness so no harness task carries production code and no pair deadlocks on compilation.
* **Injection goes through the per-workspace seam, not the filesystem.** Filesystem injection cannot
  reach the append because `snapshotShipArtifacts` takes the same lock and snapshots the same log
  file at `internal/core/shipment_lifecycle.go:478`, before the transition at `:509`.
* **The item-log lock is NOT hoisted.** A hoist was designed, reviewed, and rejected: assigning the
  locked context in place makes the ownership markers outlive the lock, so `restoreShipArtifacts`
  would rewrite an append-only log and `archiveItems` / `attachCommitToItems` would append with no
  in-process or cross-process exclusion; it also inverts the lock order against `lockArtifactMutations`
  (`:468`) and would hold a 3-second-starving lock across two `gateShipmentCompletion` evaluations
  whose broker timeout defaults to 600 seconds. The torn-state hazard it was meant to fix is instead
  fixed locally and completely by Unit 4.4.
* **No retry at the append site.** `AppendEvent` is explicitly not safe to blindly retry; retrying
  would risk a duplicate `shipment_status_changed: shipped` line in the very log this plan protects.
  A pre-append lock failure is already `not-applied` and compensates.
* **Compensation is all-or-nothing or it says so.** Silently skipping an item whose lock cannot be
  re-acquired is not a compensation contract.
* **The defer registrations are swapped, not nilled.** Nilling the releaser would make the
  non-member-feature fallback unconditionally dead and delete the 133.004-T guarantee for three
  post-closure failure paths.
* **Untagged append errors compensate; only tagged indeterminate suppresses rollback.** This matches
  `MutationEnvelope`'s precedence and keeps the contract total in the default configuration.
* **Two distinct doctor findings.** Different causes, different operator responses, and one is
  residue this plan itself creates.
* **The integration/regression unit was deleted, not deferred.**
* **MCP exposure is justified by `check_partial_mutations`, not by gate evidence.**
* **Recovery guidance is keyed on producer, not class.**
* **Policy, skill, and the Ship agent are all in scope.** Unit 4 deliberately breaks the P-007
  postcondition; the Ship agent is the surface that actually executes it.
* **Pre-existing registry drift and a repo-wide parity invariant are deferred**, not absorbed. They
  predate this defect and their current pass/fail state across roughly forty operations has not been
  verified by this plan.
* **The deliberation's `UpdateArtifactWithGate` minimum floor is renegotiated into `47B48DB0`.**
  Detection coverage does not regress while prevention waits.

## Safety Mode and Strict-Safety Declaration

Safety modes in force: **freeze-scope** and **careful**.

Freeze-scope path boundary. Ship may modify only: `internal/core/workspace.go`,
`internal/core/shipment.go`, `internal/core/shipment_lifecycle.go`, `internal/core/doctor.go`,
`internal/core/canonical_scan.go`, `internal/errors/` (one exported constant), the new test files
named in Units 2 and 6, `internal/cli/doctor.go`, `internal/cli/doctor_test.go`,
`internal/cli/registry_parity_test.go`, `internal/mcp/tools.go`, `internal/mcp/errors.go`, the new
`internal/mcp` contract tests, `.autoharness/backlog-registry.yaml`, `.autoharness/drift-ignore`,
`docs/cli-reference/backlogit_doctor.md` (generated),
`docs/design-docs/governed-mutation-recovery-contract.md`,
`.github/policies/workflow-policies.md`, `.github/skills/shipment-reconcile/SKILL.md`,
`.github/agents/.ship.agent.md`.

```text
ProposedAction:
  summary: Make the shipment active-to-shipped audit event durability-gated, add class-aware
           rollback with honest compensation that can deliberately leave a shipped-and-unarchived
           residue, correct the ShipShipment defer ordering, and add a report-only doctor audit
           plus its CLI, MCP, guidance, and policy surfaces.
  targets: the freeze-scope path list above
  change_kind: contract change + runtime behavior change + rollout
  rollback: git revert -m 1 of the merge commit (see Rollback procedure)
  approval_required: yes (operator approval at the P-014 merge gate)
ActionRisk: high
ActionResult: planned
```

Rationale for `high` rather than `destructive`: no unit deletes data, rewrites history, or performs
an irreversible action. The audit is read-only by contract and asserted so. The risk is behavioral -
a previously-succeeding ship can now refuse, and a new `shipped_unarchived_residue` state with no
automated forward transition can be created.

## Risks and Caveats

| Risk | Mitigation |
|---|---|
| Fail-closed append refuses a ship that previously succeeded | Intent, and bounded: only `ShipmentShipped` is affected, asserted by the untouched claim/abandon tests |
| Item-log lock contention at the append surfaces as a hard failure | It is classified `not-applied` and compensates; nothing is written, so the shipment reverts cleanly. Note the wait is bounded at 3 seconds only for the cross-process file lock (`internal/events/stream.go:40`, `:177`); the in-process `mutex.Lock()` (`:117-125`) is uncancellable, so the honest statement is "bounded for cross-process contention, serialized for in-process contention" |
| Compensation silently skips an item whose log lock cannot be re-acquired | Unit 4.4: bounded retry, then an explicit `partially-compensated` result naming the un-restored IDs |
| Untagged errors misclassified, stranding a shipment in the default non-durable configuration | Unit 4.2 compensates on untagged, matching `MutationEnvelope` |
| The classification branch becomes dead code behind the `len(shipSnapshots)` guard | Unit 4.3 places it before the guard explicitly |
| The non-member-feature fallback runs without locks, or is deleted outright | Unit 4.7 swaps the defer registrations so the release unwinds last and the fallback runs with locks held |
| A transient busy-lock permanently abandons the covering-feature compensation | Unit 4.6 tracks attempt and success separately and permits one fallback retry |
| The `shipped_unarchived_residue` escalates into `missing_shipped_event` via a later archive | Unit 10 guidance carries "reconcile before archiving"; Unit 7 records event presence in the finding detail |
| No forward transition out of `shipped` | Declared limitation with a documented manual procedure; a supported transition is a named closure follow-up |
| Item-level events inside `ShipShipment` remain best-effort and are owned by no follow-up | Named in the Problem Frame, restated in the Unit 4.9 doc comment, raised at closure |
| A CI `cli-reference-drift` failure blocks every unit, not just Unit 8 | Unit 8 owns the regenerated doc; the pre-deploy checklist re-verifies `make docs` |
| Registry asymmetry during the Unit 8 to Unit 9 window | One task wide, forward-only, and no misleading permanent-asymmetry marker is written |
| Guidance change mis-advises unrelated partial-mutation producers | Unit 10 keys on `FailedStep`; the existing `track_commit` assertion is kept as a regression guard |
| Stale P-007, reconcile skill, or Ship agent instruct `git restore` and mask the residue | Unit 11 adds the third branch to all three and records the files in `.autoharness/drift-ignore` |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Safety-First Go | All new errors wrapped with `%w`; `errors.Join` for combined failures; no `unsafe`; `go vet`, `golangci-lint`, `gofmt` are Ship gates |
| II. Test-First Development (NON-NEGOTIABLE) | Structural on both core tracks: `143.003-T` depends on harness `143.002-T`; `143.007-T` depends on harness `143.006-T`. Each harness is preceded by an additive scaffold (`143.001-T`, `143.005-T`) that changes no behavior. Units 8-10 are surface units whose tests are colocated per repository convention and are gated by the P-002 `harness-ready` label plus P-004 red-phase confirmation. Unit 11 is documentation only. |
| III. Workspace Isolation | All paths resolve inside the workspace root; fixtures use `t.TempDir()` |
| IV. CLI Workspace Containment | No out-of-tree writes |
| V. Structured Observability | Fixed greppable `slog` shape (Unit 4.8); two named finding types; structured MCP `mutation_partial` result |
| VI. Single Responsibility | No new dependency; every primitive reused already exists on the baseline |
| VII. Destructive Command Approval | No destructive action; the audit is read-only by contract, asserted by recursive byte comparison |
| VIII. Explicit Safety Modes | Declared above: freeze-scope plus careful, with `ProposedAction` / `ActionRisk: high` / `ActionResult: planned` |
| IX. Git-Friendly Persistence | No file-format change |
| X. Agent Context Efficiency | The audit consumes the existing canonical scan rather than per-ID lookups |
| XI. Merge Commit History Preservation | Ship-side gate; the rollback procedure specifies `git revert -m 1` and a merge-commit merge |

### Documented deviations

Each entry names the principle, the justification, and the simpler alternative that was rejected.

1. **Width Isolation - Units 7, 8, 9, 10 combine production code with colocated tests.**
   Justification: the repository colocates `*_test.go` beside its source
   (`internal/cli/doctor.go` / `doctor_test.go`; `internal/mcp/doctor_partial_mutations_test.go`),
   and each is a single surface change with a two-to-three assertion contract test. Rejected
   alternative: splitting each into harness plus implementation, producing eighteen tasks for four
   small surface changes and fragmenting single-file contracts across two commits. The units where
   production risk is real - the core durability change and the core doctor change - **are** split
   into scaffold, harness, and implementation.
2. **Width Isolation - Unit 8 also commits a generated documentation file.**
   Justification: CI requires `docs/cli-reference/backlogit_doctor.md` to be regenerated in the same
   commit as any CLI change (`.github/workflows/ci.yml:152-182`), so separating them guarantees a red
   build. The file is generated, never hand-authored. Rejected alternative: a separate docs task.
3. **2-Hour Rule file heuristic - Unit 1 touches three files.**
   Justification: two are one-to-three-line additions (the `Workspace` seam field, one exported
   constant in `internal/errors`); the substantive change is confined to `internal/core/shipment.go`.
   Rejected alternative: leaving the failed-step name as a duplicated string literal in two packages,
   which reintroduces the stringly-typed coupling review flagged.
4. **2-Hour Rule file heuristic - Unit 8 touches three files and Unit 11 touches five.**
   Justification: Unit 8's third file is generated and CI-mandated (see deviation 2). Unit 11 is
   documentation and policy only, with no functions and no tests; each edit is the same three-sentence
   third branch applied to the four surfaces that carry the broken postcondition, plus one
   `.autoharness/drift-ignore` line. Rejected alternative: splitting Unit 11 per file, which would
   allow an intermediate state where policy and the Ship agent disagree about whether to `git restore`.
5. **2-Hour Rule scenario heuristic.** Every unit is at three scenarios or fewer. No deviation.
6. **Task count.** Eleven tasks at roughly two hours each is about twenty-two hours of
   human-equivalent effort, spread across at least two Ship sessions. Each task is individually
   atomic and the graph permits two parallel tracks. The twenty-task session stop condition is a
   per-session limit, not a per-shipment limit.

## Plan Hardening Signals

* public API, schema, or contract change: **yes**
* security, auth, permission, or compliance-sensitive behavior: **no**
* migration, backfill, destructive data/config action, or irreversible step: **no**
* external integration, operator checkpoint, or external dependency: **yes**
* high runtime, rollout, or rollback risk: **yes**

Requires plan hardening: **yes**

## Plan Hardening

### Guardrails carried into execution

* Classification must operate on the source-captured `shipmentEventAppendError` only.
* The indeterminate branch must never call `restoreShipArtifacts`.
* The classification branch must sit **before** the `len(shipSnapshots)` guard.
* Untagged append errors must compensate, never suppress rollback.
* The append must never be retried.
* A short or partial write from the non-durable append path must be classified
  `ErrWriteIndeterminate`, never `not-applied`. Compensating over a partially written append-only
  log is the one outcome the `AppendEvent` contract explicitly forbids.
* A `partially-compensated` result must never be reported as `Retryable: true`.
* The item-log lock must not be hoisted out of `appendShipmentEventErr`; the ownership markers in
  `ctx` must never outlive the lock that set them.
* `releaseArtifactLocks` must be registered before the non-member-feature fallback so it unwinds
  after it. The fallback must remain reachable.
* Compensation must never silently skip an item; an un-restored item must surface as
  `CompensationState: "partially-compensated"` naming the ID.
* The doctor audit must be proven non-mutating by recursive byte comparison.
* **`.backlogit/reconcile/` sidecar: decided NO.** The indeterminate branch returns the
  `MutationPartialError` only. Deliberation `059-DL` deferred this question to plan hardening; the
  answer is that the plan does not write to a path it otherwise never touches.
* Lock-order invariant, stated honestly for the code as it exists: **no blocking artifact-lock
  acquisition while an item-log lock is held.** `lockArtifactMutation` uses non-blocking `TryLock`
  (`internal/core/task_lock.go:67-80`) and its busy error must surface as a retryable refusal, not a
  hang. Any path holding two or more item-log locks simultaneously must take them in `sort.Strings`
  order, as `lockAdoptionEventLogs` (`internal/core/shipment_lifecycle.go:1041-1058`) already does.

### Failure-injection matrix required before Unit 4 is considered green

Every row is injected through the `ws.shipmentEventAppend` seam, so each fault lands at exactly one
call site.

| Injected seam error | Expected class | Expected outcome | Owning scenario |
|---|---|---|---|
| `nil` (no injection) | none | Shipped event present and ordered before archival | Unit 2, scenario 1 |
| wraps `blerrors.ErrWriteNotApplied` | not-applied | Compensate to `active`; nothing archived; release scope not completed | Unit 2, scenario 2a |
| bare `errors.New` (untagged) | not-applied | Identical to the tagged not-applied case | Unit 2, scenario 2b |
| `ErrWriteNotApplied` plus one un-restorable release-scope item | not-applied, partial | `CompensationState: "partially-compensated"` naming the un-restored ID | Unit 2, scenario 2c |
| wraps `blerrors.ErrWriteIndeterminate` | indeterminate | No release-scope rollback; covering feature restored in-lock and re-read from disk; archival halted; `MutationPartialError` returned | Unit 2, scenario 3 |
| failure in `completeReleaseScope` (unrelated, pre-append) | not an append error | Existing unconditional rollback unchanged | existing `TestShipShipment_RollsBackReleaseScopeWhenShipmentPersistFails` |
| post-closure `archiveItems` failure | not an append error | Non-member covering feature still restored (defer-swap regression) | Unit 4 verification |

### Stop conditions

* If Unit 2's scenarios cannot be made to fail on the Unit 1 tree **for the right reason** - the
  failure must surface from the ship path, not from `"snapshot release scope"` - halt.
* If Unit 3 or Unit 4 turns any of `TestShipShipment_RollsBackReleaseScopeWhenShipmentPersistFails`,
  `TestShipShipment_RestoresNonMemberFeatureEvenWhenShipFailsAfterRollup`,
  `TestShipShipment_RestoresNonMemberFeatureBeforePostShipHooksObserveIt`, or
  `TestRestoreShipArtifactsReadFailureLeavesEventLogUntouched` red, halt and re-derive the
  classification boundary rather than editing the existing test.
* If `internal/mcp/error_mapping_test.go:200` goes red in Unit 10, halt: the guidance change has
  leaked out of the shipped-event producer.
* If any unit requires touching a path outside the freeze-scope list, halt and return to Stage.

## Release Observability

### Monitoring plan

| Field | Value |
|---|---|
| SLI 1 | Count of `missing_shipped_event` findings |
| SLI 1 query | `backlogit doctor --check-shipped-event-completeness`, counting findings of type `missing_shipped_event` |
| SLI 1 baseline | Record the pre-merge count on the real workspace as a frozen historical baseline. Historical residue is not repairable by this change, so a non-zero baseline is expected and must not decrease |
| SLI 1 alert threshold | Any value above the recorded pre-merge baseline |
| SLI 2 | Count of `shipped_unarchived_residue` findings |
| SLI 2 query | Same command; cross-check with `backlogit query "SELECT id, status FROM items WHERE artifact_type='shipment' AND status='shipped'"` |
| SLI 2 baseline | `0` |
| SLI 2 alert threshold | Any value greater than `0` persisting past one completed ship cycle |
| SLI 3 | Count of indeterminate shipped-event append failures |
| SLI 3 query | Grep session and CI logs for the literal `slog` message `shipment shipped-event append failed` with `class=indeterminate` and `failed_step=shipped-event-append` (shape fixed by Unit 4.8); corroborate with MCP `mutation_partial` results carrying the same `failed_step` |
| SLI 3 baseline | `0` |
| SLI 3 alert threshold | Any occurrence; each is an operator-actionable reconciliation event |
| SLI 4 | Count of ship refusals caused by the new gate |
| SLI 4 query | The same `slog` message with `class=not-applied`, counted over the observation window |
| SLI 4 baseline | `0` |
| SLI 4 alert threshold | Any occurrence whose `slog` record carries no underlying `os` error cause |
| SLI 5 | Count of partially-compensated ship failures |
| SLI 5 query | MCP `mutation_partial` JSON results carrying `compensation_state: "partially-compensated"`, and the Unit 4.8 `slog` record, which MUST include `compensation_state` and the un-restored IDs. `MutationPartialError.Error()` does not render `CompensationState`, so plain error text is not a measurement surface |
| SLI 5 baseline | `0` |
| SLI 5 alert threshold | Any occurrence; the named un-restored IDs must be reconciled before the next ship |
| Owner | Ship execution agent during the implementation cycle; repository operator `@softwaresalt` thereafter |
| Observation window | The first three completed `backlogit shipment ship` cycles after merge, or 7 days, whichever comes first |

This repository has no external monitoring system, so the plan is a manual observation checklist
executed by the owner during the window above and carried into the operational-closure artifact.

### Pre-deploy audit checklist

* [ ] Rollout gates: the doctor audit is off by default on both CLI and MCP. The durability change is
      intentionally unflagged - fail-closed on the shipped transition is the fix
* [ ] `make docs` re-run and `docs/cli-reference/backlogit_doctor.md` committed; CI job
      `cli-reference-drift` green
* [ ] Registry `doctor.params` carries `check_shipped_event_completeness`
* [ ] Rollback procedure documented and actionable (below)
* [ ] Data migration or schema change: none
* [ ] Backward compatibility: item JSONL is only appended to and read, never read-modified;
      historical residue is reported, never repaired
* [ ] Dependent surfaces updated in-tree: P-007, `shipment-reconcile` skill, `.ship.agent.md`,
      `governed-mutation-recovery-contract.md`, `.autoharness/drift-ignore`
* [ ] Monitoring plan complete and the frozen pre-merge `missing_shipped_event` baseline recorded
* [ ] Lock-order invariant re-verified: no blocking artifact-lock acquisition while an item-log lock
      is held

### Rollback triggers

| Trigger | Metric and threshold | Action |
|---|---|---|
| Ship refuses without a real append failure | SLI 4 greater than `0` with no underlying `os` error in the `slog` record | Revert the merge; re-open `143.003-T` and `143.004-T` |
| Indeterminate residue accumulates | SLI 2 greater than `0` across more than one ship cycle, or SLI 3 greater than `1` in the window | Halt further ships, run the audit, reconcile per the named-limitation procedure, then decide revert versus forward fix |
| Compensation reports partial | SLI 5 greater than `0` | Reconcile the named IDs immediately; if it recurs, revert `143.004-T` |
| The audit mutates JSONL | Any non-zero byte delta under `.backlogit/logs/` (recursive `**/*.jsonl` compare) attributable to a doctor run | Immediate revert; this violates the report-only contract |
| Existing ship rollback regression | Any of the four named existing ship tests fails post-merge | Immediate revert |
| Unrelated recovery guidance changed | `internal/mcp/error_mapping_test.go` red, or a `track_commit` or checkpoint caller reports doctor guidance naming the shipped-event flag | Revert `143.010-T` only |

### Rollback procedure

1. Identify the merge commit for the PR landing the shipment allocated at Stage harvest for feature
   `143-F` (tasks `143.001-T` through `143.011-T`). The concrete shipment ID is recorded in the
   closure artifact.
2. `git revert -m 1 <merge-sha>` on a fresh branch from `main`.
3. Run `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`, and `make docs`.
4. Open a revert PR and merge it with a merge commit (P-009).
5. Reverting restores best-effort append semantics. It does **not** delete any event already appended
   and does **not** repair historical residue; both are append-only and out of scope for a revert.
6. Re-open the affected tasks and record the revert reason on `143-F`.

## Runtime Verification and Closure

* Runtime surfaces changed
  * `core.ShipShipment` failure semantics on the `active -> shipped` transition, its compensation
    contract, and its defer ordering
  * `core.Doctor` finding set (two advisory, off-by-default finding types)
  * `backlogit doctor` CLI flag set and its generated reference documentation
  * `backlogit_doctor` MCP parameter set and the doctor registry row
  * `backlogit_ship_shipment` MCP error result recovery guidance
  * Workflow policy P-007, the `shipment-reconcile` skill post-mode gate, and the Ship agent Step 6
* Verification steps
  * Execute the full seam-injected failure matrix, including the partially-compensated row and the
    post-closure `archiveItems` defer-swap regression
  * Run `backlogit doctor --check-shipped-event-completeness` against the real workspace and confirm
    both finding types render with byte-identical `.backlogit/logs/` before and after
  * Confirm CLI and MCP produce identical findings for a shared fixture, from
    `internal/cli/registry_parity_test.go` and manually
  * Confirm the MCP indeterminate result names both the MCP parameter and the CLI flag and carries
    the reconcile-before-archiving instruction
  * Confirm `git diff --exit-code docs/cli-reference/` is clean after `make docs`
* Closure artifact expectations
  * Runtime verification note containing the failure-injection matrix results
  * The recorded pre-merge `missing_shipped_event` baseline count
  * The concrete shipment ID, for the rollback procedure
  * Observation-window outcome: healthy, degraded, or rolled back
  * Strict-safety `ActionResult` updated from `planned` to its terminal value
* Named closure follow-ups
  * Stash `47B48DB0` remains active as the prevention complement and now also carries the
    deliberation's `UpdateArtifactWithGate` minimum floor
  * A supported reconciliation transition out of `shipped`
  * Durability of the item-level events inside `ShipShipment` (`status_changed`,
    `returned_to_backlog`, parent cascades), which no current item owns
  * Reconciliation of the two pre-existing drifted registry doctor params
    (`check_partial_mutations`, `check_workspace_root_conflict`) and a repo-wide
    `params`-to-`InputSchema` parity assertion, after verifying its current state across all
    registry operations

## Plan Review

<!-- plan-review-attempt: 3 -->

dispatch_mode: multi-agent-dispatch

decision: PASS

Three review cycles ran against this plan in a clean worktree at `3ec95ee3`. Personas dispatched:
Architecture Strategist, Scope Boundary Auditor, Constitution Reviewer, Concurrency Reviewer,
Schema-CLI-Docs Coupling Reviewer.

### Cycle history

| Cycle | Outcome | Blocking findings | Effect on the plan |
|---|---|---|---|
| 1 | FAIL | P0=3, P1=16 across five personas | Rewritten: 7 units to 10; both core tracks split into harness plus implementation; safety-mode declaration added; policy and contract coherence brought in scope |
| 2 | FAIL | P0=2, P1=12 | The item-log lock hoist and the append-site retry, both introduced in cycle 1, were reviewed and DELETED; the defer-nil idea was replaced with a defer-registration swap; scaffold units added ahead of both harnesses; all source anchors regenerated |
| 3 | PASS | P0=0, P1=0 from the two personas whose blocking findings drove the cycle-2 revision | Remaining adversarial findings remediated in place |

### Cycle 3 records

* Concurrency Reviewer - PASS, P0=0 P1=0. Verified the defer swap against
  `internal/core/shipment_lifecycle.go:335-398`, `:454-468`, `:509-545`, confirmed LIFO reasoning,
  confirmed `ShipShipment` has no other function-scope defers, and confirmed deleting the hoist does
  not reintroduce the torn-state hazard because `eventsSinceSnapshot` already preserves
  non-operation events. One advisory recorded and adopted: make the compensation retry budget
  per-call rather than per-item.
* Schema-CLI-Docs Coupling Reviewer - PASS, P0=0 P1=0. All eight cycle-2 findings resolved; every
  corrected source anchor verified byte-exact; no generated or enumerated surface left uncovered.
  One advisory recorded and adopted: amend the `.autoharness/drift-ignore` header NOTE rather than
  only appending paths.
* Architecture Strategist (cycle 2) - the 10-unit decomposition was judged "not over-engineered
  relative to the bug", with Units 1-4 the minimum honest fix and Unit 11 required rather than
  optional. Its cycle-2 P1s (hoist inversion, retry duplication risk, compile-order cycle, anchor
  drift, dead fallback) are all addressed by the deletions and splits recorded above.
* Scope Boundary Auditor (cycle 2) - all four cycle-1 P1s resolved; Units 8-11 each returned KEEP at
  the unit level; its scope-down recommendations (drop the transient `cli_only_flags` entry, defer
  the pre-existing registry drift and the repo-wide parity invariant) are adopted and recorded in
  the Requirements Trace as DEFERRED.
* Constitution Reviewer (cycle 2) - its four cycle-1 P1s are addressed by the scaffold-plus-harness
  splits on both core tracks, the three-scenario cap, the safety-mode declaration, and the six-entry
  documented-deviations list.

Blocking findings outstanding: none.

## Adversarial Review

dispatch_mode: multi-model-adversarial

decision: PASS

Full report: `docs/reviews/2026-08-17-shipment-shipped-event-audit-log-adversarial-review.md`

Three independent reviewers on three different model families reviewed the third revision against
the clean `3ec95ee3` worktree, with deliberately different emphases and without seeing each other's
findings.

| Confidence | Definition | Count | Disposition |
|---|---|---|---|
| HIGH | identified by all three reviewers | **0** | - |
| MEDIUM | identified by two of three | 2 | Both fixed in this revision |
| LOW | identified by one reviewer | 11 | 9 fixed, 2 rejected with recorded rationale |

No HIGH-confidence P0 or P1 finding exists, so the adversarial gate is not blocked. Both
MEDIUM-confidence findings were fixed rather than deferred. The two rejected LOW-confidence
findings, and the reasons, are recorded in the report.
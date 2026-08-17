---
chunk_strategy: h1-h2-h3
description: "Implementation plan to guarantee the shipment shipped-event audit log is durable across active to shipped to archived transitions, with a report-only doctor audit."
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-16-shipment-shipped-event-audit-log-plan.md
title: "Shipment shipped-event audit-log durability and doctor reconciliation plan"
ms.date: 2026-08-16
ms.topic: how-to
---

## Problem Frame

Source document: `docs/decisions/2026-08-16-shipment-shipped-event-audit-log-deliberation.md`
(deliberation `059-DL`, stash `0115F71F`).

A shipment can persist `archived_status: shipped` while its append-only JSONL
event log omits the `shipment_status_changed: shipped` event. In current code:

* `internal/core/shipment.go` `moveShipmentStatusWithHeadGuard` persists the
  status transition through `persistArtifactWithGuard`, then emits the audit
  event via the best-effort `appendItemEvent(ctx, ws, shipmentID,
  "shipment_status_changed", {status})`. `appendItemEvent` (shipment.go:660 ->
  `appendItemEventWithCommit`) only `slog.Warn`s and returns on lock, append, or
  index failure. The failure never reaches the caller.
* `ShipShipment` (`internal/core/shipment_lifecycle.go:285`) runs
  `moveShipmentStatusWithHeadGuard(active -> shipped)` inside its locked closure,
  then after the closure returns calls `collectArchiveCandidateIDs`,
  `attachCommitToItems`, and `archiveItems`. `archiveItems` stamps
  `archived_status: shipped`. A silently dropped shipped-event append still lets
  archival proceed, producing the reported inconsistency.

The chosen direction (deliberation Option B) integrates an error-returning
shipped-event append into the existing envelope, classifies failures with the
two-class durable-write contract, reuses the existing `ShipShipment`
snapshot/rollback machinery for compensation, threads the caller EventWriter for
CLI/MCP parity, and adds a report-only doctor audit.

## Requirements Trace

| Stash requirement | Implementation action | Unit |
|---|---|---|
| (1) Integrate the shipped-event append into the shipment mutation/rollback envelope using an error-returning writer; archival must not continue without that durable event | Route the shipped transition through the error-returning `appendItemEventErr(ctx, ws, ...)`, scoped to `newStatus == ShipmentShipped`; return the append error from `moveShipmentStatusWithHeadGuard` so the locked closure aborts before `archiveItems` | Unit 1 |
| (2) Restore active/unarchived shipment and release-scope state on append failure, or return an explicit indeterminate reconciliation error | Make the closure rollback class-aware: `ErrWriteNotApplied` (and any clean pre-durable failure) compensates via the existing snapshot rollback; `ErrWriteIndeterminate` (and any untagged/unknown failure, treated as indeterminate by safe default) is never rolled back, halts archival, restores transient covering-feature rollups while locks are held, and returns a `MutationPartialError` | Unit 2 |
| (3) Test successful active -> shipped -> archived ordering and injected append failure across the shared CLI/MCP core path | Use a per-`ws` append seam; assert success ordering (shipped event before archival), injected NotApplied compensation, and injected Indeterminate/untagged no-rollback plus reconciliation error plus halted archival, against the shared `ShipShipment` core | Unit 3 |
| (4) Add a doctor audit detecting historical archived shipments with archived_status: shipped but no shipped event; report only, never rewrite historical JSONL | Add `FindingMissingShippedEvent` and `FindingShippedUnarchivedResidue` plus `DoctorOptions.CheckShippedEventCompleteness`; scan the full canonical queue-and-archive raw-Markdown refs (queue for the shipped-but-unarchived residue, archive for the archived state), detect the archived `archived_status: shipped` missing-event state (`missing_shipped_event`) and the shipped-but-unarchived residue anomalous regardless of event presence (`shipped_unarchived_residue`); advisory, never writes JSONL | Unit 4 |
| MCP structured surfacing of the new indeterminate error and dual-surface doctor exposure | Map the indeterminate `MutationPartialError` in MCP `handleShipShipment` to a structured `mutation_partial` result; expose the doctor check via CLI flag, MCP parameter, and registry | Units 5a, 5b, 6 |

## Implementation Units

Every unit follows the 2-hour rule (fewer than 3 files, fewer than 5 functions,
fewer than 4 test scenarios), width isolation (single domain), and produces an
atomic, verifiable milestone. Execution posture is test-first for all units
(Constitution Principle II).

### Unit 1: Error-returning shipped-event append scoped to the shipped transition

* Changes: in `moveShipmentStatusWithHeadGuard`, route the
  `shipment_status_changed` emission through the error-returning
  `appendItemEventErr(ctx, ws, ...)` and return its error, but ONLY for the
  active-to-shipped transition (gate on `newStatus == ShipmentShipped`) so the
  claim (queued-to-active) and abandon transitions keep their current best-effort
  semantics and are not destabilized. Parity comes from passing the same `ws` (the
  ws-configured, durable-aware writer); do not add an EventWriter parameter to
  `ShipShipment` or mint a fresh writer. Introduce a per-`ws` append seam (mirroring
  `ws.gateEvidenceAppend`, for example `ws.shipmentEventAppend`) so the append is
  injectable in tests without a package-global.
* Files: `internal/core/shipment.go` plus the per-`ws` seam field on the
  `Workspace` type (one small field and its default wiring).
* Tests: Unit 1 lands the per-`ws` seam and a failing red assertion that an injected
  shipped-append failure makes `moveShipmentStatusWithHeadGuard` return a non-nil
  error for the shipped transition; broader scenarios are Unit 3.
* Verification: an injected shipped-event append failure causes
  `moveShipmentStatusWithHeadGuard` to return a non-nil error on the shipped
  transition; claim and abandon transitions are unchanged; the success path writes
  the shipped event before the closure returns.
* Execution posture: test-first. Unit 1 is NOT independently releasable; it is
  unsafe without Unit 2 (which makes the rollback class-aware) and must land with
  it.
* Acceptance criteria:
  * The shipped transition emits `shipment_status_changed` through
    `appendItemEventErr(ctx, ws, ...)` whose error is returned to the caller.
  * The change is scoped to `newStatus == ShipmentShipped`; claim and abandon
    semantics are unchanged and asserted so.
  * The append is injectable through a per-`ws` seam, not a package-global var.

### Unit 2: Class-aware rollback, two-class classification, and reconciliation error

* Changes: make the `ShipShipment` locked-closure rollback CLASS-AWARE for the
  shipped-event-append failure SPECIFICALLY (an explicit, tested change to the
  hardened closure, not a no-op reuse). Capture the shipped-append error at its
  source -- the ship-transition return site in `moveShipmentStatusWithHeadGuard` --
  as a distinct, privately-typed boundary (for example a `shipEventAppendError`
  wrapper) rather than inspecting the generic `closureErr`. Only that captured
  shipped-append error is classified: `ErrWriteNotApplied` (and a clean pre-append
  failure such as event-log lock acquisition) compensates via the existing deferred
  `restoreShipArtifacts` over `shipSnapshots` back to active/unarchived; a
  post-append durability-uncertain `ErrWriteIndeterminate` (and any untagged error
  from the append step itself, treated as indeterminate by safe default) does NOT
  roll back. ALL OTHER closure errors -- including untagged failures from earlier
  steps (`completeReleaseScope`, `returnUnreleasedFeatureItems`, feature status
  cascades) -- retain the EXISTING unconditional rollback so release-scope mutations
  are never left applied with the shipment still active. On the indeterminate branch:
  suppress `restoreShipArtifacts`, synchronously restore any transient non-member
  covering-feature rollups while the artifact locks are still held (preserving the
  133-F ordering) and mark the outer fallback consumed (`restored = true`) on both
  restore success and failure so it cannot rerun after `releaseArtifactLocks`, halt
  archival, and return `MutationPartialError{Class: "indeterminate", FailedStep:
  "shipped-event-append", CompensationState: "not-compensated", Cause: err}` wrapping
  the cause with `%w`. A compensation (restore) failure is JOINED onto the returned
  error (never swallowed), and indeterminate dominates any joined classification.
  Emit a structured `slog` record on both the compensation and the indeterminate
  branch.
* Files: `internal/core/shipment_lifecycle.go` (the closure defer and the
  ship-transition return path) and `internal/core/shipment.go` (the captured
  append-error boundary), reusing `blerrors.IsWriteIndeterminate` and
  `errors.MutationPartialError`; no new error infrastructure.
* Dependency note: the append classification carries tagged sentinels only when the
  workspace runs durable writes; when `durable_writes` is off the appender returns
  untagged errors, which the safe default routes to the indeterminate
  (never-roll-back) branch ONLY for the captured shipped-append error, not for other
  closure errors. State this dependency in the closure comment. To realize the
  NotApplied compensate path for a clean pre-append event-log lock-acquisition
  failure, tag that failure as `ErrWriteNotApplied` at the appender (a small addition
  to `appendItemEventWithActorErr`); absent that tag it safely degrades to the
  indeterminate branch and the Unit 4 doctor audit detects the residue.
* Tests: Unit 2 lands its own unit-level failing (red) classification assertions
  before the production change -- NotApplied (and clean pre-append lock-acquisition)
  compensates, post-append Indeterminate/untagged never rolls back, and all
  other/pre-append closure errors keep the existing unconditional rollback. The
  broader integration scenarios (success ordering, combined append-plus-rollback,
  covering-feature restore) are added in Unit 3.
* Verification: NotApplied (and clean pre-append lock-acquisition) compensates to
  active/unarchived with no shipped event and no archival; post-append Indeterminate
  (and untagged) leaves the shipment shipped and unarchived, restores covering-feature
  rollups under lock, halts archival, and returns the `MutationPartialError`; a
  pre-append or other closure error still rolls back the release scope; an
  indeterminate append is never rolled back.
* Execution posture: test-first.
* Acceptance criteria:
  * Only the captured shipped-append error is classified; NotApplied (and clean
    pre-append lock-acquisition) compensates, post-append Indeterminate/untagged never
    rolls back, and all other/pre-append closure errors keep the existing
    unconditional rollback.
  * The indeterminate branch halts archival, marks the outer fallback consumed, joins
    any restore failure onto the error, and returns a `MutationPartialError` with
    `Class: "indeterminate"`, wrapping the cause with `%w`.
  * Transient covering-feature rollups are restored synchronously while artifact locks
    are held; the membership lock and release-scope flow are otherwise unchanged.

### Unit 3: Failure-injection and success-ordering tests across the shared core

* Changes: using the per-`ws` append seam from Unit 1 (restored with `t.Cleanup`),
  add tests exercising the shared `ShipShipment` core used by both CLI and MCP.
* Files: `internal/core/shipment_lifecycle_test.go` or
  `internal/core/shipment_test.go`.
* Tests (three scenarios, under the 4-scenario cap):
  * Success: active -> shipped -> archived persists AND the
    `shipment_status_changed: shipped` event is present, ordered before archival.
  * Injected `ErrWriteNotApplied`: compensation to active, no archival, no shipped
    event.
  * Injected `ErrWriteIndeterminate` (with an in-scenario assertion that an untagged
    error is handled identically), using a fixture whose ship would roll up a
    non-member covering feature: the release scope is NOT rolled back, the covering
    feature is restored (not left done or archived), the shipment is left
    shipped-and-unarchived, archival is halted, and a `MutationPartialError` is
    returned.
* Verification: all three scenarios pass; the seam is restored; because the seam is
  per-`ws`, tests remain parallel-safe. CLI/MCP ship-path parity is guaranteed by
  construction (single shared core) and needs no separate surface-level ship tests.
  The pre-append and other-error rollback is unchanged and stays covered by the
  existing `TestShipShipment_RollsBack*` tests.
* Execution posture: test-first at the integration level. Units 1 and 2 each land
  their own unit-level failing (red) assertions; Unit 3 sequences after them
  (depends on Units 1 and 2) and adds the broader cross-cutting integration
  scenarios that exercise the combined append-plus-rollback behavior.
* Acceptance criteria:
  * The three scenarios pass against the shared `ShipShipment` core path.
  * The success test asserts event ordering (shipped event before archival), not
    only presence.
  * The untagged-error assertion confirms the safe-default indeterminate handling.

### Unit 4: Report-only doctor audit for missing shipped events (core)

* Changes: add `FindingMissingShippedEvent DoctorFindingType = "missing_shipped_event"`
  and `FindingShippedUnarchivedResidue DoctorFindingType = "shipped_unarchived_residue"`,
  plus `DoctorOptions.CheckShippedEventCompleteness bool`; in `Doctor`, gate a new
  check (off by default, advisory and exit-code-neutral like
  `FindingOverArchivedCoveringFeature`) that walks the FULL canonical artifact scan
  already used by `Doctor` (both `queue/` and `archive/`, since a shipped-but-unarchived
  shipment stays under `queue/` because `shipped` is not an archive-routed status),
  parsing each exact path raw Markdown once (extend `artifactRef` to carry
  `archived_status`), not a per-ID `findArtifact` second lookup, and flags a shipment when
  EITHER (a) it is archived with `archived_status: shipped` and its item JSONL log has
  no `shipment_status_changed` event with `status == shipped` -> `FindingMissingShippedEvent`,
  OR (b) it has `status: shipped` but is not archived (the indeterminate residue from
  Unit 2) -> `FindingShippedUnarchivedResidue`, which is anomalous regardless of event
  presence because a shipped shipment normally archives, so an event-present residue is
  reported truthfully as `shipped_unarchived_residue` and never mislabeled
  `missing_shipped_event`; record whether the shipped event is present in the finding
  detail in both cases. Carry
  the same "verify actual history; may be transient during an in-flight ship" caveat
  that `FindingOverArchivedCoveringFeature` uses so a doctor run racing the brief
  window between the status persist and the shipped-event append does not false
  positive. Never write or rewrite JSONL.
* Files: `internal/core/doctor.go` (mirrors the 133-F `CheckOverArchivedFeatures`
  registration at doctor.go:522) plus a small helper if needed.
* Tests: `internal/core/doctor_test.go` (three scenarios): archived
  `archived_status: shipped` with no shipped event -> one `missing_shipped_event`
  finding; the same with the shipped event present -> none; a `status: shipped`
  unarchived shipment -> one `shipped_unarchived_residue` finding (the residue state),
  asserted even when the shipped event is present so the distinct type is exercised.
  Assert the run modifies no JSONL bytes.
* Verification: findings are correct for both detected states and the check performs
  no writes.
* Execution posture: test-first.
* Acceptance criteria:
  * Two advisory, off-by-default finding types (`missing_shipped_event` and
    `shipped_unarchived_residue`) and one option exist; the check never
    changes the doctor exit code.
  * Detection reads authoritative archived state from the canonical raw-Markdown scan,
    not the DB projection, and covers both the archived missing-event state and the
    shipped-but-unarchived residue state.
  * The event-present shipped-but-unarchived residue is reported as
    `shipped_unarchived_residue`, never mislabeled `missing_shipped_event`.
  * The check never writes or rewrites historical JSONL (asserted by unchanged log
    bytes).

### Unit 5a: Doctor audit CLI surface

* Changes: expose the check through the CLI `doctor` command
  (`--check-shipped-event-completeness`), wiring it to
  `DoctorOptions.CheckShippedEventCompleteness`.
* Files: `internal/cli/doctor.go` and `internal/cli/doctor_test.go`.
* Tests: the CLI flag surfaces the `missing_shipped_event` finding for a seeded
  fixture and is absent by default.
* Execution posture: test-first.
* Acceptance criteria:
  * The CLI flag enables the check and appears in help text.
  * Defaults are unchanged when the flag is absent.

### Unit 5b: Doctor audit MCP surface and registry parity

* Changes: expose the check through the MCP `backlogit_doctor` tool (a
  `check_shipped_event_completeness` parameter) and record it in the backlog
  registry. Justify read-only MCP exposure explicitly: this check is read-only and
  never writes, so exposing it to the agent surface is appropriate even though the
  CLI-only `CheckOverArchivedFeatures` is not exposed. Do not cite a
  `CheckGateEvidence` MCP precedent (that check is CLI-only). Sequences after Unit 5a
  (depends on it): the CLI/MCP parity assertion compares this MCP surface against the
  Unit 5a CLI flag, so the CLI flag must already exist.
* Files: `internal/mcp/tools.go` and `.autoharness/backlog-registry.yaml` (doctor
  operation params) plus an `internal/mcp` contract test.
* Tests: a shared fixture yields identical `missing_shipped_event` and
  `shipped_unarchived_residue` findings through the CLI and the MCP tool (CLI/MCP
  parity).
* Execution posture: test-first.
* Acceptance criteria:
  * The MCP parameter enables the check and is recorded in the registry metadata.
  * A shared fixture yields identical findings (both `missing_shipped_event` and
    `shipped_unarchived_residue`) through CLI and MCP.

### Unit 6: MCP structured surfacing of the indeterminate ship error

* Changes: ensure the MCP `handleShipShipment` path surfaces the new indeterminate
  `MutationPartialError` as a structured `mutation_partial` result (the shared
  `domainError` mapping may already do this via `errors.As`, mirroring
  `handleTrackCommit`; verify it is not consumed earlier by `gateErrorResult`). The
  substantive change is the RECOVERY GUIDANCE: the indeterminate result must direct
  the caller to `doctor --check-shipped-event-completeness` (the Unit 4 check), NOT
  `check_partial_mutations`, which cannot detect a missing shipped event. Update
  `mutationPartialRecovery` (or the equivalent guidance text) accordingly.
* Files: `internal/mcp/tools.go` and an `internal/mcp` contract test.
* Tests: an injected indeterminate ship error yields a structured `mutation_partial`
  MCP result carrying the class and failed step, not a generic error string.
* Execution posture: test-first.
* Acceptance criteria:
  * `handleShipShipment` maps the indeterminate error to a structured MCP result.
  * The `errors.As(&MutationPartialError)` check is ordered before the existing
    `gateErrorResult` dispatch so the indeterminate result is not consumed as a
    generic gate error.
  * The recovery guidance directs the caller to
    `doctor --check-shipped-event-completeness`, not `check_partial_mutations`.
  * A contract test asserts the structured shape (class, failed step) is preserved.

## Dependency Graph

```text
Unit 1 (error-returning shipped-scoped append + per-ws seam)
  -> Unit 2 (class-aware rollback + classification + reconciliation error)
Unit 1 + Unit 2
  -> Unit 3 (failure-injection + ordering tests)
Unit 2
  -> Unit 6 (MCP indeterminate error mapping)
Unit 4 (doctor core audit)          [independent of Units 1-3]
  -> Unit 5a (doctor CLI surface)
       -> Unit 5b (doctor MCP surface + registry parity)
  -> Unit 5b (doctor MCP surface + registry parity)
```

No cycles. Units 1 and 4 can start in parallel. Unit 2 depends on Unit 1; Unit 3
depends on Units 1 and 2; Unit 6 depends on Unit 2; Unit 5a depends on Unit 4; Unit 5b
depends on Unit 4 and Unit 5a (its CLI/MCP parity assertion compares against the Unit
5a CLI flag, so the CLI surface must land first). Units 1 and 2 land together (Unit 1
is not independently releasable).

## Decisions and Rationale

* Reuse the existing error-returning appender (`appendItemEventErr`) rather than a
  new writer: it already implements the "audit record must land or the transition
  fails" semantics on the gated completion path. Rationale: minimize new surface
  and match a proven pattern.
* Scope the error-returning append to the active-to-shipped transition
  (`newStatus == ShipmentShipped`). Rationale: `moveShipmentStatusWithHeadGuard`
  also serves claim and abandon, which have no snapshot/rollback; changing their
  failure semantics is out of scope and would create new untested divergence.
* Make the `ShipShipment` closure rollback class-aware rather than assuming the
  existing unconditional rollback is safe. Rationale: the deferred
  `restoreShipArtifacts` fires on ANY closure error; if an indeterminate append
  error propagated unchanged it would roll back a possibly-written shipped event,
  the exact unsafe Option C the deliberation rejected. NotApplied compensates;
  Indeterminate never rolls back.
* Treat untagged or unknown append errors as indeterminate by safe default, and
  note that classification carries tagged sentinels only when `durable_writes` is
  on. Rationale: the two-class contract dominance rule fails safe toward
  never-roll-back when durability is uncertain.
* Return a concrete `MutationPartialError{Class: "indeterminate", ...}` and map it
  in MCP `handleShipShipment`. Rationale: preserve a structured, reconcilable
  signal for agent callers instead of a flattened generic error.
* Broaden the doctor audit to detect both the archived `archived_status: shipped`
  without a shipped event (`missing_shipped_event`) and the shipped-but-unarchived
  indeterminate residue (`shipped_unarchived_residue`, a distinct truthful finding
  type reported regardless of event presence), scanning the full canonical
  queue-and-archive refs (raw Markdown, parsed once). Rationale:
  the indeterminate state Unit 2 can produce must be detectable; a single
  `missing_shipped_event` type would mislabel an event-present residue, so the residue
  gets its own type; `loadArtifact`
  omits `archived_status`, and a per-ID second lookup risks a duplicate-ID mismatch.
* Achieve CLI/MCP parity by passing the same `ws` through the shared core, not by
  adding an EventWriter parameter to `ShipShipment`. Rationale: the ws-configured
  writer already preserves MCP append serialization (2026-07-04 learning); a new
  signature would be needless churn across CLI and MCP callers.

## Risks and Caveats

* Regressing the hardened `ShipShipment` closure. Mitigation: the only closure
  change is making the rollback class-aware; the membership lock and release-scope
  flow are untouched; covered by Unit 3 scenarios plus the existing rollback tests.
* Rolling back an indeterminate append. Mitigation: explicit classification with a
  safe default; Indeterminate and untagged errors never roll back.
* The indeterminate residue (a shipped-but-unarchived shipment) is invisible or
  unrecoverable. Mitigation: Unit 4 detects it; `ShipShipment` refuses to rerun a
  non-active shipment, so recovery is manual reconciliation, not idempotent rerun
  (the runtime-verification and closure notes state this correction).
* Blast radius on claim and abandon transitions. Mitigation: the error-returning
  append is scoped to `newStatus == ShipmentShipped`; other transitions are
  unchanged and asserted so.
* Doctor false positives from reading the DB projection or a wrong duplicate-ID
  copy. Mitigation: read raw Markdown via the canonical archived scan, parsed once;
  assert in tests.
* Process-kill or power-loss window between the status persist and the append is
  not covered by an in-process envelope. Caveat stated honestly: the doctor audit
  is the standing detection surface; recovery is manual reconciliation; this plan
  does not add a journal (descoped 099-S).
* Alternate entry points (for example `UpdateArtifactWithGate`) could drive a
  shipment to `shipped` outside `moveShipmentStatusWithHeadGuard`. Mitigation: Unit 1
  verifies the ship path routes through the guarded envelope, and the Unit 4 doctor
  audit is the standing detection net for any historical or future bypass. PREVENTION
  (universally rejecting a generic shipment-to-shipped transition outside
  `ShipShipment`) is a distinct hardening deferred to stash `47B48DB0` to respect the
  audit-log-completeness bug boundary; detection ships now, prevention follows.

## Constitution Check

* I. Safety-First Go: pass. Production stays in Go; the append error is wrapped
  with `%w`; no `unsafe`; the change replaces a swallowed error with a handled
  one.
* II. Test-First Development (NON-NEGOTIABLE): pass. Each unit lands a failing
  harness before production code (Unit 1 lands the append-return red assertion,
  Unit 2 the classification red assertions); Unit 3 adds integration-level coverage
  over the combined behavior and therefore sequences after Units 1-2.
* III. Workspace Isolation and Security Boundaries: pass. All reads and writes
  resolve within the workspace root; the doctor audit only reads.
* IV. CLI Workspace Containment (NON-NEGOTIABLE): pass. No out-of-tree writes.
* V. Structured Observability: pass. The change strengthens the audit trail by
  guaranteeing the shipped event is durable, and emits a structured `slog` record on
  both the compensation and the indeterminate branch so failure paths are traceable.
* VI. Single Responsibility: pass. No new dependencies; existing primitives are
  reused.
* VII. Destructive Command Approval (NON-NEGOTIABLE): N/A. No destructive terminal
  commands; the doctor audit is report-only and never rewrites JSONL.
* VIII. Explicit Safety Modes: pass. Investigate-first posture was applied during
  planning; the risky rollback path is classified in Plan Hardening.
* IX. Git-Friendly Persistence: pass. Markdown plus append-only JSONL; historical
  JSONL is never rewritten.
* X. Agent Context Efficiency: pass. The doctor audit is a targeted check.
* XI. Merge Commit History Preservation (NON-NEGOTIABLE): N/A for Stage. The
  downstream Ship PR must merge via a merge commit; recorded here for Ship.

Constitution Check: pass

## Plan Hardening Signals

* Public API, schema, or contract change: present. A new doctor CLI flag, a new MCP
  `backlogit_doctor` parameter and registry entry, a new `DoctorFindingType`, a
  class-aware change to the hardened `ShipShipment` rollback closure, a new
  `MutationPartialError` return from the ship path, and its structured MCP
  `mutation_partial` mapping are added.
* Security, auth, permission, or compliance-sensitive behavior: absent. No auth or
  permission surface is touched.
* Migration, backfill, destructive data/config action, or irreversible step:
  absent. The doctor audit is report-only; no data migration; the rollback path is
  a compensating restore, and historical JSONL is never rewritten.
* External integration, operator checkpoint, or external dependency: absent.
* High runtime, rollout, or rollback risk: present. The change modifies the
  hardened `ShipShipment` rollback envelope and the durability/ordering of a
  lifecycle audit event; rollback correctness is central.

Requires plan hardening: yes

## Runtime Verification and Closure

Changed runtime surfaces: the `backlogit shipment ship` CLI command and the
`backlogit_ship_shipment` MCP tool (shared `ShipShipment` core), plus the
`backlogit doctor` CLI command and `backlogit_doctor` MCP tool.

* Runtime verification to prove before absorption:
  * A normal ship over a fixture shipment emits the `shipment_status_changed: shipped`
    event and THEN stamps `archived_status: shipped` (event before archival).
  * With an injected NotApplied append failure, ship leaves the shipment active and
    unarchived; with an injected Indeterminate (or untagged) failure, ship returns
    the `MutationPartialError` and does not archive, leaving the shipment shipped and
    unarchived.
  * The MCP `backlogit_ship_shipment` tool surfaces the indeterminate failure as a
    structured `mutation_partial` result, not a generic error string.
  * `backlogit doctor --check-shipped-event-completeness` reports both the seeded
    archived inconsistency and a seeded shipped-but-unarchived residue, reports clean
    on a consistent fixture, and writes no JSONL.
  * CLI and MCP produce identical doctor findings for the same workspace.
* Operational closure artifacts expected:
  * A closure note recording the honest boundary (no cross-process or crash
    durability guarantee; the doctor audit is the detection surface and recovery is
    manual reconciliation, not idempotent rerun, because `ShipShipment` refuses a
    non-active shipment).
  * A rollback trigger: if the shipped transition regresses (ship failures or spurious
    indeterminate errors in normal operation), revert the envelope change; the doctor
    audit remains valid independently.
  * Owner and validation window: the Ship agent during post-merge closure.

## Release Observability

Release-observability evidence for this release unit (a local Go CLI plus MCP tool;
the monitoring surface is the doctor command and the test suite, not a live service).
Produced here once and carried into operational closure.

### Monitoring plan

* SLI / key metrics: (1) shipment ship success rate -- successful `backlogit shipment
  ship` / `backlogit_ship_shipment` completions without a spurious indeterminate
  `MutationPartialError` in normal (non-injected) operation; (2) the count of
  shipped-event-completeness doctor findings (`missing_shipped_event` +
  `shipped_unarchived_residue`) over the shipment corpus.
* Concrete local command/query:
  * `backlogit doctor --check-shipped-event-completeness` -- counts
    `missing_shipped_event` and `shipped_unarchived_residue` findings (report-only).
  * `backlogit query "SELECT COUNT(*) FROM items WHERE artifact_type='shipment' AND status='shipped'"`
    -- the shipped-but-unarchived residue count (expected 0 in steady state).
  * `go test ./internal/core/... ./internal/cli/... ./internal/mcp/...` -- the ship
    and doctor regression suites (green gate).
* Baseline (pre-change): ship success rate 100% on the current corpus;
  `shipped_unarchived_residue` count = 0 (the indeterminate branch does not yet
  exist, so no residue is possible today); the first audit run over the existing
  archived-shipment corpus fixes the historical `missing_shipped_event` count, which
  must not grow for any shipment shipped after the change.
* Failure threshold / alert condition: any shipment shipped after the change that
  surfaces a `missing_shipped_event` or `shipped_unarchived_residue` finding, OR ship
  success rate < 100% in normal (non-injected) operation.
* Owner / role: the Ship agent during post-merge closure, escalating to the
  repository operator on any threshold breach.
* Observation window / duration: the first three post-merge ships OR seven days after
  merge, whichever comes first; the Ship agent runs
  `doctor --check-shipped-event-completeness` at the start and end of the window
  rather than assuming silence means success.

### Pre-deploy audit checklist

* Feature flags / rollout gates: none required; the error-returning append is
  always-on but scoped to the governed `newStatus == ShipmentShipped` transition, so
  claim and abandon are unaffected.
* Rollback path documented and actionable: yes (see below) -- revert the single
  envelope-change commit; the report-only doctor audit is independent and remains
  valid after revert.
* Data migration / schema change: none. The doctor audit is report-only; append-only
  historical JSONL is never rewritten; the new `DoctorFindingType` values, the
  `DoctorOptions` field, the CLI flag, the MCP parameter, and the registry entry are
  all additive.
* Backward compatibility: no existing contract changed.
* Dependent services / cross-boundary: none; the change is internal to the backlogit
  CLI and MCP core.
* Monitoring plan complete: yes (above).

### Rollback trigger and procedure

* Rollback trigger (named metric + threshold): normal (non-injected) ships begin
  failing OR emit spurious indeterminate `MutationPartialError`s (ship success rate
  < 100%), OR a shipment shipped after the change surfaces a `missing_shipped_event`
  or `shipped_unarchived_residue` doctor finding.
* Rollback procedure: revert the envelope-change commit (the Unit 1/Unit 2 change to
  `moveShipmentStatusWithHeadGuard` and the `ShipShipment` class-aware rollback); the
  shipped-event append reverts to best-effort and behavior returns to the pre-change
  baseline. The Unit 4 doctor audit and its CLI/MCP surfaces are report-only and
  independent; they may remain or be reverted separately without affecting the
  envelope revert.

### Releasability evidence contract

Evidence entries for operational-closure to mark READY / READY_WITH_CONDITIONS /
BLOCKED:

* Validator evidence (pre-merge, Ship-run): `go test ./...`, `go vet ./...`,
  `golangci-lint run`, `gofmt -l .` clean.
* Runtime evidence: the Runtime Verification scenarios above proven on fixtures
  (event ordering, NotApplied compensation, Indeterminate halt, doctor detection of
  both states, CLI/MCP parity).
* Post-deploy evidence: the observation-window doctor runs recorded with finding
  counts and the outcome (healthy / degraded / rolled back).

## Plan Hardening

Hardening required: yes. Triggers: a public contract change (new doctor CLI flag,
new MCP parameter, new finding type, new error/reconciliation return path) and
high rollback risk (the change modifies the hardened `ShipShipment` rollback
envelope and the durability and ordering of a lifecycle audit event).

Learnings and instruction files consulted:

* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
  (NotApplied vs Indeterminate; indeterminate dominates and is never rolled back).
* `docs/design-docs/governed-mutation-recovery-contract.md` and
  `docs/exec-plans/2026-08-07-f5-idempotent-multi-mutation-envelope-plan.md`
  (F5 envelope and `MutationPartialError` classification, reused not rebuilt).
* `docs/exec-plans/2026-08-14-shipshipment-rollback-cas-plan.md` (140-F parent;
  this is its Unit 1, deferred from `106.033-T`).
* `docs/closure/2026-08-01-133-shipshipment-cascade-fix-closure.md` (report-only
  doctor precedent; read raw Markdown; never rewrite JSONL).
* `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md` (append seam,
  `t.Cleanup`, no `t.Parallel`).
* `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`
  (thread the caller EventWriter; do not mint a fresh one).
* `docs/compound/security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md`
  (audit alternate writers that can reach `shipped`).
* `.github/instructions/constitution.instructions.md` (Principles II, V, IX, XI).

Protected invariants:

* Once a shipment persists `archived_status: shipped`, a durable
  `shipment_status_changed: shipped` event MUST exist (or an explicit
  indeterminate reconciliation error MUST have been surfaced).
* The `ShipShipment` membership lock and release-scope ordering (106-F, 133-F)
  remain unchanged.
* Historical `.backlogit/` JSONL is never rewritten or synthesized.

Risky actions:

* ProposedAction: change the shipment status-transition envelope to make the
  shipped-event append error-returning and abort the closure on failure.
  * ActionRisk: high (touches the hardened ship/rollback path).
  * Approval: covered by the pre-authorized dark-mode Ship merge; no Stage
    execution. ActionResult: planned.
* ProposedAction: on `ErrWriteIndeterminate`, halt archival and surface a
  reconciliation error without rollback.
  * ActionRisk: high (incorrect handling can corrupt the audit trail).
  * Approval: same. ActionResult: planned.
* ProposedAction: add a report-only doctor audit reading raw Markdown.
  * ActionRisk: low (read-only; never writes JSONL).
  * ActionResult: planned.

Deepened verification (carried into runtime verification):

* Assert event ordering explicitly (shipped event strictly before archival), not
  only presence.
* Inject BOTH `ErrWriteNotApplied` and `ErrWriteIndeterminate` and assert the two
  distinct outcomes (compensate vs surface-and-halt).
* Assert the doctor run mutates zero JSONL bytes (byte-compare before/after).
* Assert CLI/MCP parity on identical fixtures.

Rollback and monitoring:

* Rollback trigger: normal ships begin failing or emit spurious indeterminate
  errors after the change; revert the envelope change. The doctor audit is
  independent and remains valid after a revert.
* Monitoring signal: ship success rate and the count of `missing_shipped_event`
  doctor findings over the archived-shipment corpus (expected to trend to zero for
  new ships).

Unresolved operator decisions (non-blocking for harvest):

* Whether the indeterminate path also writes a `.backlogit/reconcile/` sidecar
  (117-S precedent) or only returns the reconciliation error. Default: return the
  error only; add the sidecar only if Ship finds it necessary during
  implementation. Kept out of scope to avoid gold-plating.

## Plan Review

dispatch_mode: multi-agent-dispatch

decision: PASS

Reviewer subagent dispatch is available in this environment (Copilot CLI `task`
tool), so real persona subagents were dispatched. `TOOL_OK: reviewer-subagent-dispatch`.

Personas covered (always-on plus triggered cross-model):

* Constitution Reviewer (caller model)
* Go Reviewer (caller model)
* Scope Boundary Auditor (caller model)
* Learnings Researcher (caller model; Step 1.8 result folded in, HIGH confidence)
* Architecture Strategist (cross-model, gpt-5.6-sol)
* Agent-Native Parity Reviewer (cross-model, gemini-3.1-pro-preview; triggered by
  the MCP tool surface change)

The Security Lens Reviewer was not triggered: the plan touches no auth, authz,
secrets, or sensitive data store.

Plan hardening: required (public contract change plus high rollback risk) and
satisfied. The `## Plan Hardening` section is present with classified risky actions,
deepened verification, and rollback/monitoring detail.

### Gate history

* Attempt 1: FAIL. Multiple P1 findings (see below).
* Attempt 2 (after revision 1): Go Reviewer confirmed all prior P1/P2 RESOLVED with
  no new P0/P1; Architecture Strategist confirmed most resolved but raised residual
  P1s (append-scoped classification, chokepoint prevention, residue detection).
* Attempt 3 (after revision 2): Architecture Strategist confirmed all five residual
  items RESOLVED with no remaining or new P0/P1. Gate PASS.

Cycle count: 2 revision cycles (attempt 3 is the PASS), within the plan-review
re-entry limit.

### Findings and dispositions

P1 (all resolved before PASS):

* Unconditional closure rollback would roll back an indeterminate append (unsafe
  Option C). Resolved: Unit 2 makes the rollback class-aware and scopes it to the
  source-captured shipped-append error only; all other closure errors keep the
  existing unconditional rollback.
* Two-class sentinels inert when `durable_writes` is off; untagged errors undefined.
  Resolved: untagged errors from the append step are treated as indeterminate by
  safe default; the `durable_writes` dependency is stated.
* `moveShipmentStatusWithHeadGuard` shared by claim/abandon. Resolved: Unit 1 scopes
  the error-returning append to `newStatus == ShipmentShipped`; claim/abandon are
  unchanged and asserted.
* Indeterminate residue (shipped-but-unarchived) undetectable/unrecoverable.
  Resolved: Unit 4 flags every shipped-but-unarchived shipment; recovery reframed as
  manual reconciliation (not idempotent rerun).
* Skipping rollback conflicts with 133-F covering-feature restore/lock ordering.
  Resolved: Unit 2 restores transient rollups synchronously under lock and marks the
  outer fallback consumed on both restore success and failure.
* MCP `handleShipShipment` must map the indeterminate error to a structured result.
  Resolved: Unit 6 adds the `errors.As` mapping ordered before `gateErrorResult`.
* Chokepoint prevention (`UpdateArtifactWithGate` bypass). Resolved by
  policy-compliant defer: prevention is tracked as stash `47B48DB0`; the Unit 4
  doctor audit is the interim detection net. The Scope Boundary Auditor considered
  prevention out of scope for this bug.

P2 (resolved): EventWriter parity restated as passing the same `ws` (no new
signature); per-`ws` test seam; unit ordering (seam lands with Unit 1); concrete
`MutationPartialError` shape with double-fault joining and indeterminate dominance;
Unit 5 split into 5a (CLI) and 5b (MCP plus registry); doctor scan via canonical
archived refs (not per-ID lookup); clean pre-append lock-acquisition classified as
NotApplied.

P3 (acknowledged, folded into acceptance criteria): advisory/exit-code-neutral
doctor finding; read-only MCP exposure justified against the `CheckGateEvidence`
precedent; in-flight-ship transient caveat on the doctor finding; structured `slog`
on the failure paths; deliberation updated for EventWriter and recovery-surface
consistency.

Runtime verification and operational closure gaps: none outstanding. The changed
runtime surfaces (`shipment ship` and `doctor`, CLI and MCP) carry explicit
verification scenarios and closure expectations in the Runtime Verification and
Closure section.

### Adversarial multi-model review

An explicit adversarial multi-model review was run after the plan-review gate PASS,
because the work touches durable event ordering, a hardened rollback envelope, and
doctor reconciliation. Three independent reviewers across different model families
(GPT `gpt-5.6-terra`, Gemini `gemini-3.1-pro-preview`, Grok `grok-4.6`) probed the
plan and decision adversarially. Full record and consensus dispositions:
`docs/reviews/2026-08-16-shipment-shipped-event-audit-log-adversarial-review.md`.

Result: no HIGH-confidence (unanimous) P0/P1. The Gemini reviewer independently
validated the core design as sound. MEDIUM P1s were resolved in-plan (MCP recovery
guidance to the new doctor check; Unit 3 covering-feature-restore assertion; Unit 4
full queue-and-archive scan) or handled by a policy-compliant defer with an
actionable backlog ID (prevention hardening -> stash `47B48DB0`, broadened to cover
the `ArchiveItem` path). Adversarial review result: PASS. Cleared for harvest.

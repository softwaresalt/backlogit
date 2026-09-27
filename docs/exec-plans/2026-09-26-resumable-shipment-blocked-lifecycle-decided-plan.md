---
chunk_strategy: h1-h2-h3
description: "Decisions and verified outcomes for the 155-S resumable blocked-shipment lifecycle release."
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-26-resumable-shipment-blocked-lifecycle-decided-plan.md
title: "Decided Plan: Resumable shipment blocked lifecycle status (155-S)"
docline:
  date: 2026-09-26T20:40:00Z
  status: decided
  decided_from: docs/archive/plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md
  shipment_id: 155-S
  feature_id: 174-F
---

## Decision

Ship a governed, resumable `blocked` lifecycle status for shipment artifacts.
`blocked` is a nonterminal lifecycle pause, distinct from a dependency edge,
and does not occupy the single active-shipment slot. Only governed block,
unblock, claim, and recovery paths may change the protected lifecycle state.

The completed release is PR #450, merged at
`2c8759c3f7583d678ef674b3c6566541b4945375`. The implementation and review
history remain in the archived source plan and
`docs/closure/2026-09-25-155-S-final-review.md`; operational evidence is in
`docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md`.

## Scope and invariants

* Shipment `155-S` covers exactly its 45 explicit manifest members: feature
  `174-F` and 44 tasks. The feature is a member but not a task wave. Four tasks
  (`174.069-T`–`174.072-T`) already had valid terminal archive evidence before
  the final governed close.
* Shipment behavior uses the flat manifest. A listed feature does not expand
  membership to unlisted descendants or linked deliberations. Locking,
  snapshots, mutation, rollback, archival, and returns use the shipment
  control record plus explicit manifest members only.
* A blocked shipment is excluded from the active slot but retains the reason,
  timestamp, member-status snapshot, branch, actor, and resume-checkpoint
  reference needed for governed resumption.
* `154-S` bootstrap procedures are documented in the archived source plan but
  were not executed by this release. No `154-S` work, E1–E5 ensemble work, or
  unrelated release unit is in this scope.

## Accepted design

### Lifecycle and active-slot control

* Add the shipment-only status `blocked`, with legal governed transitions
  `active → blocked` and `blocked → queued|active`. Ordinary status moves,
  generic updates, bulk/cascade writers, and create-as-active paths cannot
  bypass the governed transition envelope.
* `BlockShipment` records a durable member-status snapshot and returns active
  manifest members to `queued`, freeing the global active slot. `UnblockShipment`
  checks that the requested target is valid and that the slot is available,
  then restores members from the snapshot.
* Claim, block, and unblock serialize through the workspace-global lifecycle
  gate before membership and item-log locks. A pre-existing multiple-active
  condition fails closed and is surfaced by Doctor rather than treated as
  permission to proceed.
* Blocked-state guards apply only to shipment artifacts; work-item `blocked`
  status remains a separate status-axis value.

### Evidence, recovery, and normalization

* Lifecycle changes use durable intent, preimage, applied evidence, terminal
  event, and journal ordering. Compensation records restored status evidence
  before a `compensated` terminal and journal phase. A commit whose terminal
  evidence is already durable is never compensated; an indeterminate write
  fails closed for recovery.
* The unblock `shipment_status_changed` event carries the validated reason,
  actor, target, and prior resume reference before frontmatter clears the
  blocked envelope. The event is provisional until the correlated terminal
  event is durable.
* Normalization uses the target-guarded recovery path on warm and cold MCP
  servers. It refuses pending lifecycle or return-blocked journals referencing
  the target shipment or its manifest members, including malformed
  return-blocked files attributable by name. Unrelated poison journals are
  left untouched. The MCP `by` argument is required and checked for
  whitespace; normalization intentionally has no CLI counterpart.
* Doctor and recovery remain the evidence oracle for malformed blocked
  metadata, multiple active shipments, and torn lifecycle intent. No
  ungoverned repair or journal editing is part of this release.

### Surfaces and compatibility

* Queue, ready-work selection, index, dependency, Doctor, CLI, and MCP
  behavior preserve the blocked/non-active distinction. Public lifecycle
  operations retain actor and reason evidence; `blocked_by` is advisory, not
  an authentication boundary.
* The feature is additive and requires no data migration. Rollback is a
  reviewed code revert; before using an older binary, operators must first
  inspect blocked shipments and governably unblock them to a status the older
  binary recognizes.
* The unfiltered governed full-suite command is the literal
  `go test -timeout=30m ./...`. The explicit per-test-binary timeout was
  selected from existing Windows run evidence and replaces the implicit
  10-minute default without a wrapper, new package, formula, or result
  collector.

## Wave 20 corrective decisions

Final-review remediation added tasks `174.077-T`–`174.082-T` in three
dependency waves:

| Wave | Tasks | Contract completed |
|---|---|---|
| 1 | `174.077-T`, `174.079-T`, `174.082-T` | Preserve the blocked envelope at generic-write boundaries; order compensation evidence; guard normalization against aggregate journals. |
| 2 | `174.078-T`, `174.080-T` | Record unblock reason before clearing metadata; route warm normalization through the guarded MCP path and require actor attribution. |
| 3 | `174.081-T` | Avoid compensation after committed event evidence is durable. |

The three blocking dependencies are `174.078-T → 174.079-T`,
`174.080-T → 174.082-T`, and `174.081-T → 174.078-T`. The archived plan
retains exact harness selectors, regression command counts, per-task SHA
boundaries, and detailed implementation assertions.

## Rejected alternatives and deferred boundaries

* Rejected using generic status movement to block a shipment: it would not
  disposition active members or preserve governed resumption evidence.
* Rejected expanding a listed feature into descendants or linked
  deliberations: that would violate flat-manifest membership and could mutate
  unrelated artifacts during rollback or archive.
* Rejected compensating after durable committed evidence: it can create
  contradictory committed and compensated terminal events.
* Rejected an 18-task dynamic test-budget wrapper. Existing evidence supported
  one explicit 30-minute timeout with adequate headroom and fewer failure
  modes.
* Deferred the wider CI, script, documentation, and harness-template command
  migration to ensembles E1–E5; only the governed final-gate surfaces needed
  for 155-S were changed.
* Deferred cross-surface and robustness expansions were captured for Stage;
  they are not implicit authorization to widen this release. The closure record
  is the canonical residual-risk list, including `FA6AE139`, `9900D0DD`,
  `09D06A75`, `8AF55264`, `7D8717B1`, `388C586D`, `3340E97C`, `9CA03F5D`,
  `2F7FCA8B`, `4A0B7BCF`, `7AA35A39`, `AC6B669D`, `A4B62D86`, `0FFBF819`,
  `45BD3B36`, `67F17B6B`, and `D116AF58`. E1–E5 remain deferred under
  `checkpoint-20260925-005049.json`.

## Review and verification outcome

* Wave 20 plan review rev23.5 followed the operator-approved attempt-4
  advisory disposition. The implementation then passed the final standard
  and four-slot adversarial reviews with no unresolved P0/P1 or in-scope P2.
* The lock-order test progressed through `85570a3b`, `f3deced0`, and
  `bd6c00ee`; its independent gpt-6-sol re-review was READY with zero
  findings, and the targeted test passed 10/10.
* The one governed suite at `3a240fe0` passed. The later operator-authorized
  run at `0b11459f` exited 1 after 2029 seconds solely because the final
  review document lacked two docline soft keys. The keys were added in
  `24e75400507977dd5d0825252719f74e8773addb` and targeted verification
  passed. The full suite was not rerun locally; PR #450 CI passed.
* PR #450 CI checks passed, P-018 Copilot review completed for the reviewed
  head with all Copilot threads resolved, and the PR merged by normal
  merge-commit strategy without admin fallback.
* The governed post-merge close archived all 45 explicit manifest members;
  post-close reconciliation found no missing, duplicate, mismatched, or
  returned members. See the operational-closure artifact for complete
  archive, sync, monitoring, rollback, and runtime-verification evidence.

## Traceability

* Archived source plan and review history:
  `docs/archive/plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md`.
* Product spec:
  `docs/product-specs/2026-09-14-resumable-shipment-blocked-lifecycle-status.md`.
* Deliberation:
  `docs/decisions/2026-09-14-resumable-shipment-blocked-lifecycle-status-deliberation.md`.
* Final review:
  `docs/closure/2026-09-25-155-S-final-review.md`.
* Post-merge operational closure:
  `docs/closure/155-S-resumable-shipment-blocked-lifecycle-status-post-merge-closure.md`.

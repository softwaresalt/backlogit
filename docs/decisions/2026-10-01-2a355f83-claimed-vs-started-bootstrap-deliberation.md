---
title: "Deliberation: Claimed-versus-started bootstrap repair for Ship wave admission (2A355F83)"
doc_type: "decision"
source: "docs/decisions/2026-10-01-2a355f83-claimed-vs-started-bootstrap-deliberation.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: "Stage decision for the operator-authorized narrow bootstrap repair. It separates claim-assigned from started shipment tasks, teaches Ship wave admission to consume the 154-S scheduler_baseline_claim marker, and records a single-shipment condition (C) exception without attesting consumption."
topic: "Bootstrap repair: claimed-versus-started tracking and Ship wave-admission task selection"
depth: "standard"
decision_status: "decided (operator authorization 2026-10-02T00:29:30.253Z; Stage design choices recorded below)"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-10-01-2a355f83-claimed-vs-started-bootstrap-plan.md"
  - "docs/memory/2026-10-01/orchestrator-shipment-ledger-clarification-memory.md"
  - "docs/design-docs/scheduler-baseline-marker-contract.md"
  - "docs/decisions/2026-09-28-513e62ab-condition-b-enforcement-deliberation.md"
  - "docs/closure/154-S-173-F-scheduler-baseline-marker-post-merge-closure.md"
  - "docs/closure/2026-09-30-154-s-runtime-verification.md"
  - "docs/memory/2026-09-29-ship-154s-wave-schedule-contract-halt.md"
bootstrap_id: "2A355F83"
tags:
  - "bootstrap-exception"
  - "shipment-claim"
  - "wave-admission"
  - "condition-b"
  - "scheduler-baseline-marker"
---

## Operator Authorization (verbatim)

> 'Yes, I authorize that narrowly scoped bootstrap exception'

* Received: `2026-10-01T17:29:30.253-07:00` (`2026-10-02T00:29:30.253Z`).
* Relayed by the Orchestrator to Stage in this session; Stage records it verbatim and does not
  paraphrase it into a broader grant.
* It answers the Orchestrator's precise proposal: a small, separate prerequisite repair for
  claimed-versus-started tracking and Ship task selection, with other ledger improvements kept
  separate. It allows repairing and verifying this consumer **before** a truthful consumption
  attestation exists, while keeping the admission gate for every existing successor.
* **What it is not.** It is not permission to:
  * attest on the operator's behalf, or declare consumption already proven;
  * bypass any other gate, or claim 187-S;
  * modify archived 154-S, or remove existing dependency edges;
  * broadly redesign the ledger, activate dark mode, or preauthorize merges.

  No merge or admin-fallback authorization was granted.
* Binding: the exception is recorded against exactly ONE concrete bootstrap shipment. Its ID is
  written into the plan's `## Harvest Record` and into the shipment's own comment log after
  assembly (see Exception Matrix).

## Problem Frame

Since 154-S shipped (merge `6d233d21`), `ClaimShipment` bulk-activates every `queued` member and
stamps it `custom_fields.scheduler_baseline_claim = <shipment ID>` (M2, accepted). The installed
Ship consumer never reads that marker:

* Ship Step 4.0 item 4 halts on any `active` member at wave admission with `WAVE_NO_PROGRESS`
  (detail `active residual`).
* Item 6 admits only `queued` members to `ready_k`.

So wave 1 of every claimed shipment halts. 154-S itself could only run under an
operator-approved, shipment-specific active-residual exception
(`docs/memory/2026-09-29-ship-154s-wave-schedule-contract-halt.md`).

There is a second gap: once a member is claim-activated, nothing durable tells a member that
Ship has started apart from one that is merely assigned.

Who cares: the operator, and every queued successor. Condition (C), the operator's consumption
attestation on `154-S`, cannot truthfully be given until a consumer actually uses the marker.
Without a carve-out, the repair that would make C true is itself gated on C, which is a real
self-dependency loop.

### Success criteria (bounded)

1. Claim-assigned and actually started members are distinguishable from durable state, and the
   start record is idempotent.
2. Ship admission treats `active` + marker == the current active shipment + explicit live-manifest
   membership + not started as **assigned**. Such a member is subject to dependency readiness.
3. Fail-closed handling is preserved for:
   * genuinely active residuals (started, wrong or missing marker, not in the live manifest);
   * ambiguous shipments, stale reads, and manifest drift.
4. Real consumer use is proven against the current producer, not only by producer or read-only
   tests.
5. The repair's own eligibility does not depend on its post-repair attestation. P is preserved for
   154-S and every successor; C is preserved for every existing successor.

### Out of scope (explicit)

* additional subtask scheduling;
* universal removal approval;
* archive/completion redesign and parent rollup rules;
* gate-off auto-completion;
* a generalized immutable ledger;
* any product API, status, event, or schema change;
* marker v2;
* revisiting M2 bulk claim activation;
* adopting 182-F/182.001-T;
* repackaging the frozen 46-task decomposition (187-S→188-S→189-S…, side branch 186-S).

Unsupported cases keep their existing fail-closed behavior. Attestation evidence produced by this
work concerns marker consumption only. It does not declare the broader ledger gaps solved.

## Research Findings

Sources: targeted known-path reads, the Orchestrator assessment, and a learnings retrieval
(confidence medium). Engram was degraded (no third bind), so a targeted literal fallback was used.

### Producer facts (current HEAD `e85b85e1`, which contains `6d233d21`)

* `internal/core/shipment_lifecycle.go` `ClaimShipment` activates only members whose preimage is
  `queued`, through `setArtifactStatusWithClaimMarker(..., StatusActive, "shipment claimed", S)`.
  * It writes `scheduler_baseline_claim = S`.
  * It appends a `status_changed` event `{from: queued, to: active, reason: "shipment claimed"}`
    to `.backlogit/logs/<id>.jsonl`.
  * Observed shape, from `173.001-T.jsonl`: actor `backlogit`, `event_type` `status_changed`,
    delta reason `shipment claimed`.
* `setArtifactStatusWithClaimMarker` returns immediately for a same-status call when
  `claimMarker == ""`.
* **Correction to the Orchestrator assessment.** The ordinary per-task move does **not** go
  through that function.
  * MCP `handleMoveItem` (`internal/mcp/tools.go:639`) and CLI `internal/cli/move.go:62` call
    `core.UpdateArtifactWithGate` (`internal/core/gate_transition.go:89`), which calls
    `updateArtifactUngated` (`internal/core/artifacts.go:523`).
  * An `active → active` move therefore persists the file, bumps `updated_at`, fires hooks, and
    appends `artifact_mutation` intent/committed events.
  * It keeps the marker. It emits no `status_changed` unless a `commit_sha` is supplied.
  * So the move is not a no-op, but it is not a distinguishable or idempotent start record either:
    a repeat move is indistinguishable from the first.
* A generic `custom_fields` update **replaces the whole map**, except reserved sizing keys
  (`mergePreserveReservedSizingKeys`). Using it to write a start flag can silently drop the
  marker.
* Comments (`backlogit_append_comment`, CLI `backlogit comment add`) append a `comment` event to
  the item log only. Observed shape: `{actor, item_id, event_type: "comment", delta: {comment}}`.
  They never touch frontmatter or the marker.
* Item logs (`.backlogit/logs/`) are git-ignored and workspace-local. The condition (C)
  attestation rule already treats raw `.backlogit/logs/154-S.jsonl` as source of truth, so a raw
  log read is an established consumer pattern.
* Normative contract: `docs/design-docs/scheduler-baseline-marker-contract.md`
  (`scheduler-baseline-marker/v1`, option A).
  * It defines a three-part predicate: `active`, marker == the single active shipment, and the
    item is in that shipment's `custom_fields.items`.
  * The marker is preserved through generic update/move. Any change to its key, value, or
    predicate requires v2.
  * Residuals: R1 stale markers; R2 any-activation-route residual (accepted); R3 the indeterminate
    rule.
  * Mixed-binary caveat: every binary that opens the workspace must include U1b before the first
    claim.
* **Live MCP binary.** `v1.10.0-1023-g2c8759c3-dirty-debug`, commit `2c8759c3`.
  `git merge-base --is-ancestor 6d233d21 2c8759c3` returns exit 1, so this binary does **not**
  contain the producer. A claim through it would activate members **without** writing the marker.

### Consumer facts (`.github/agents/_ship.agent.md`, read-only)

* Step 0.5 claims at about line 210.
* Step 3 freezes `M` and replays `scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue`.
* Step 4.0 items 4 and 6 are quoted in the Problem Frame.
* Step 4.1b "Claim Task" is "Update task status to `active` using the backlog tool's move
  operation".
* Matching policy text: `.github/policies/workflow-policies.md` P-002.6 "Active leftovers halt
  too" and per-wave procedure step 1 (about lines 703-720), the P-002.2 taxonomy (about lines
  290-300), and policy version `1.31.0`.
* `build-feature/SKILL.md` references only "Step 4.1a claim-time gate" and "before this task was
  claimed". If Step 4.1b keeps its heading and defines a task claim as the durable start, it
  needs no edit.
* The sim fixture (`tests/simulation/wave-scheduler-contract.json`, source shipment `130-S`) has
  21 scenarios.
  * Unknown scenario mutation keys are ignored, and unknown `expect` keys fail as
    `<no such observable>` assertions.
  * A fixture-first scenario therefore goes RED by assertion (exit 1), not by a thrown exception.
  * No Go test or CI workflow consumes the sim or pins the Step 4.0/4.1b text.

### 182-F / 182.001-T inspection

* 182.001-T is queued and has no shipment. Its dependencies (173.001-T, 173.003-T, 181.001-T) are
  all terminal.
* It is a producer-side regression net (`internal/core/shipment_claim_marker_lifecycle_test.go`)
  that pins that block, unblock, return, ship, abandon, and generic update/move all keep the
  marker byte-identical.
* Its own body routes it "only under the 074-DL rule after 154-S has shipped provenance (and the
  operator's autoharness-consumption attestation)".
* **Decision: do not adopt it into this bootstrap.**
  * It is producer-side, out of the authorized consumer scope, and C-gated by its own contract.
  * Adopting it would silently widen the exception.
  * The chosen start semantics never mutate the marker, so they stay compatible with its scenario
    (2).
  * The bootstrap's producer pin (plan UCS5a/UCS5b) characterizes only the comment/claim log facts the
    consumer reads, and does not duplicate 182.001-T scenarios.

### Learnings applied

* `atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27`: claim-activation is
  not a start, and a new marker or record must be handled on every transition path. That is why
  the start record lives outside frontmatter.
* `2026-07-06-ancestor-aware-shipment-gate-staleness`: when a gate depends on its own fix, prove
  closure with the rebuilt binary and do not add an ad-hoc force.
* `2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative`: a merged fix is inert
  until the self-hosted binary is rebuilt.
* `source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11`: pin stable tokens only.
* `2026-08-18-shipment-shipped-prevention-envelope`: keep the exception structural and scoped,
  never a flag.

## Options Evaluated

### Start-record semantics

* **S1: Same-status `move` to `active` (status quo text).**
  * Pros: no new text.
  * Cons: not distinguishable from claim activation in frontmatter, not idempotent (each repeat
    looks the same), and leaves no start fact a consumer can read. **Rejected.**
* **S2: Write a start flag through a generic `custom_fields` or label update.**
  * Pros: committed frontmatter, visible in the index.
  * Cons:
    * `custom_fields` replace semantics can drop the marker, contradicting v1 option A and
      182.001-T (2).
    * Labels are replaced wholesale and have no reset on return/re-claim.
    * It churns tracked queue files mid-shipment. **Rejected.**
* **S3: Product change — consume or clear the marker on per-task move, or a new start
  API/event/status.**
  * Pros: one durable product fact.
  * Cons: a breaking v1 predicate change (needs v2), contradicts option A and 182.001-T,
    speculative API surface, and makes this a producer shipment. **Rejected** (out of bootstrap
    scope).
* **S4 (chosen): Append-only start record through the existing comment operation.**
  * Ship Step 4.1b appends, with `backlogit_append_comment` (CLI fallback
    `backlogit comment add`), a comment whose first line is exactly `WORK_STARTED: <S>`, with
    actor `ship`, where `<S>` is the session shipment ID.
  * Pros:
    * It reuses an existing explicit operation and never touches frontmatter or the marker.
    * It is idempotent by read-before-append.
    * It is keyed per shipment, so a later shipment starts clean.
  * Cons:
    * The log is workspace-local and git-ignored. This is mitigated fail-closed: a missing or
      unparseable log, or one without the claim event, is indeterminate.
    * A truncated log could lose a start record. Accepted as residual RB2.

### Admission placement

* **A1: Exempt the marker check only for this one shipment and keep the defect.**
  Rejected: it does not repair the consumer for successors.
* **A2 (chosen): Amend Step 4.0/4.1b and P-002.6/P-002.2 for all shipments, with fail-closed
  sub-partitioning of `active`.** Mirror the change in the contract sim.
* **A3: Revisit M2, so that claim does not bulk-activate.**
  Rejected: it changes accepted product behavior and the marker contract.

## Trade-off Comparison

| Criterion | S1 move | S2 frontmatter flag | S3 product change | S4 comment record |
|---|---|---|---|---|
| Distinguishable start | no | yes | yes | yes |
| Idempotent | no | partly | yes | yes (read-before-append) |
| Marker v1 / 182.001-T compatible | yes | **no** (replace risk) | **no** | yes |
| New product surface | none | none | API/schema | none |
| Durability | n/a | git-tracked | git-tracked | workspace log (fail-closed on loss) |
| Bootstrap-scope fit | n/a | poor | out of scope | good |

## Decision

### D1 — Start semantics (S4)

* For a member that is claim-assigned (classification below), Step 4.1b does not move it. It is
  already `active`, and Ship never issues a redundant same-status move.
* For a `queued` ready member, Step 4.1b keeps the existing move to `active`.
* In both cases Ship then:
  1. reads the item log;
  2. appends `WORK_STARTED: <S>` only if no valid record for `<S>` exists (idempotent);
  3. re-reads the log and requires exactly one valid record for `<S>`.
* If the record is absent after the append, or more than one valid record for `<S>` exists, Ship
  halts with the new P-002.2 token `TASK_START_NOT_RECORDED` before any dispatch.
* A **valid start record** is a `comment` event in the item log `logs/<id>.jsonl` (under the
  workspace storage root the serving binary uses; `.backlogit/` here) whose actor is `ship`, whose
  comment text's first line is exactly `WORK_STARTED: <S>`, and which lies inside the current
  **start epoch**: after the latest `status_changed` `{to: active, reason: "shipment claimed"}`
  event, or anywhere in the log if no such event exists. A re-claim therefore starts a fresh
  epoch.
* Log parse policy: file order, one trailing `\r` stripped per line, blank lines ignored, and
  every other line must parse as a JSON object (one malformed non-blank line makes the log
  unparseable). Only `event_type`, `actor`, `delta.reason`, `delta.to`, and `delta.comment` are
  read.
* No `build-feature` dispatch, and no Step 4.1c or later action, happens before the re-read
  confirms exactly one valid record (H1).
* In non-shipment mode (`frozen_task_ids`, no session shipment), nothing is claim-assigned and
  Step 4.1b is unchanged.
* (Attempt-2 refinement, from plan review attempt 1: the start epoch, the parse policy, the
  storage-root path, H1, and non-shipment mode.)

### D2 — Admission classification (Step 4.0 / P-002.6)

At wave admission, for frozen `M` and session shipment `S`, partition `active` further:

* **Indeterminate** → halt `WAVE_CLAIM_STATE_INDETERMINATE` (new P-002.2 token, wave admission
  only), when any `active` member exists and any of the following holds:
  * the live active-shipment count is not exactly 1, or the single active shipment is not `S`;
  * two reads of `S`'s `custom_fields.items` within the admission step differ (manifest drift);
  * a member's item read errors, or its status disagrees with the snapshot (stale read); the
    marker is read through the marker contract's read recipe;
  * any R3 condition in the marker contract holds;
  * a marker==`S` member's item log is missing or unparseable, or lacks a
    `status_changed` `{to: active, reason: "shipment claimed"}` event.
* **Precedence:** if any member is indeterminate, halt `WAVE_CLAIM_STATE_INDETERMINATE` and
  report the residuals too.
* **Claim-assigned** (`assigned`): `active`, marker == `S`, explicitly listed in `S`'s live
  `custom_fields.items`, and no valid `WORK_STARTED: S` record in the current start epoch.
* **Active residual**: every other `active` member. That is: started (a valid record exists),
  marker missing or not `S`, or not in the live manifest. It halts `WAVE_NO_PROGRESS` (detail
  `active residual`) exactly as today.
* **Frontier:** `ready_k = { t in queued ∪ assigned : every dependency of t is terminal_success }`.
  An assigned member whose dependencies are unfinished waits. It is neither progress nor a
  residual.
* **Unchanged:** completion (`terminal_success = M`), frozen `M`, the blocked and unsupported
  halts, the empty-frontier `WAVE_NO_PROGRESS`, the cycle, budget, and snapshot halts, and
  R1/R2/R3.

### D3 — Consumer model alignment

The P-002.6 contract simulation gains the same classification, so that the mandatory Step 3
replay models the repaired consumer.

### D4 — Root type

The new root is a **chore**: internal maintenance. Live metadata confirms `chore` (prefix C,
hierarchy level 0, allowed children `task` and `review`). This is the workspace's first chore
root; see Risks.

### D5 — No new edges on existing shipments

The bootstrap shipment gets exactly one `blocks` edge, onto `154-S` (P). No edge is added to or
removed from any existing shipment, and the frozen decomposition is untouched.

## Exception Matrix (one shipment)

`B` denotes the single bootstrap shipment created at harvest. Its concrete ID is recorded in the
plan's Harvest Record and in `B`'s comment log.

| Shipment(s) | P: `blocks` edge onto 154-S, shipped provenance | C: operator consumption attestation on 154-S | Bootstrap exception | Admission |
|---|---|---|---|---|
| `B` only | **Required.** Satisfied: `154-S` `archived_status: shipped`, merge `6d233d21` | **Waived for `B` only**, under this authorization | Yes — C waiver only | Eligible once dispatch preconditions E1–E4 hold |
| 141, 147, 152, 153, 176–179, 183–194 (all 20 queued shipments with a 154-S edge, including 186-S and frozen 187-S→188-S→…) | Required (unchanged) | **Required (unchanged)** | No | Not eligible until a legitimate operator `CONDITION_B_ATTESTED:` exists |
| Other queued shipments | Unchanged | Unchanged (074-DL scope) | No | Unchanged |

* **Scope:** only the tasks harvested from this decision's plan, under one chore root.
* **Allowed admission exception:** `B` may be dispatched without C. P is still required.
* **Stop/expiry:** the exception ends at whichever comes first:
  * `B` reaches `shipped` or `abandoned`;
  * any manifest or scope change to `B` beyond the reviewed plan, including a member removed by
    `backlogit_return_blocked` (a return-blocked member leaves `B`'s live items, so the exception
    expires; resuming needs new operator authority and is never silent);
  * revocation: any comment on `B` whose first line is exactly
    `BOOTSTRAP_EXCEPTION_REVOKED: 2A355F83` (from any actor; actor fields confer no authority to
    grant and are not needed to revoke), or an explicit operator statement relayed verbatim.

  The authoritative binding to `B` is the committed plan `## Harvest Record` (concrete ID and
  approved member list), read only through the plan's Verified Main Read. `B`'s comment, written
  by actor `stage`, is informational only.

  It does not carry over to any other shipment.
* **Controls retained:** normal plan/code review, CI, runtime verification, Copilot/review-thread
  gates, and merge controls. No merge or admin fallback, no dark mode, no auto-merge.
* **Not an attestation:** shipping `B` does not satisfy C for anyone. After `B`'s consumer
  implementation and runtime verification are actually done, normal C for successors is restored
  only by a legitimate operator `CONDITION_B_ATTESTED:` comment on `154-S` (or an explicitly
  requested verbatim relay). Neither Stage nor Ship writes it.

## Dispatch Preconditions (not satisfied by this decision)

* **E1 — Producer binary.** Every binary that opens the workspace for `B`'s claim and execution
  must contain `6d233d21` (U1b). The live MCP `2c8759c3` does not. The operator performs the
  correct-binary handoff (plan §Correct-Binary Handoff). The binary must report a full 40-hex
  commit with no `-dirty` and no `unknown`; a failing check halts `BOOTSTRAP_E1_BINARY_UNMET`.
  Stage and Ship do not replace, terminate, or restart the live server.
  * The server-side check is run by the **Orchestrator** (whose tool set includes
    `backlogit_get_version`; Ship's does not). The Orchestrator passes the observed commit `X`
    and the operator-recorded `go version -m` evidence (`vcs.revision=X`, `vcs.modified=false`,
    binary path, binary SHA-256) in the dispatch text. A CLI `version` result never satisfies
    the server-side check, because it reports the CLI binary, not the served one.
* **E2 — Staging merge.** The Stage artifacts and `B`'s queue files must reach `main` through the
  normal staging PR. The last-known `origin/main` is `046c01303e76fe45b18093efe548bd8d63ee96af`.
* **E3 — New authority NOT granted: execution-time active-residual exception for `B`.** `B` is
  executed under the *installed, pre-repair* Ship contract, so its own wave 1 hits
  `WAVE_NO_PROGRESS` (`active residual`), exactly as 154-S did.
  * The current authorization covers the C admission waiver, not this wave-admission halt.
  * Stage does not assume it. The operator must decide it explicitly before Ship dispatch.
  * Exact wording (amended after plan review attempt 1, because the original admission-only
    wording was unsound: the pre-repair Step 4.1b writes no start records, so a started then
    crashed member of `B` would be re-admitted; amended again after attempt 3, because the
    replacement list left installed policy halts and Ship Step 4.0 item 7 and Step 4.6 item 1 in
    force). The plan reproduces the fenced wording text byte for byte (this list indentation
    excluded):

    <!-- BEGIN:e3-wording -->

    ```text
    For the bootstrap shipment B named in the plan 2A355F83 Harvest Record only, until B's repaired Ship and policy text is on main and installed, Ship executes B by applying the repaired text specified in plan 2A355F83 (D1, D2, start epoch, H1) in place of exactly these installed passages: Ship Step 2 item 1; Ship Step 4.0 items 4, 6, and 7; the Ship Step 4.1a never-stranded-active sentence; Ship Step 4.1b; Ship Step 4.6 item 1; policy P-002.6 Definitions ready_k; policy P-002.6 Active leftovers halt too; policy P-002.6 per-wave steps 1 and 2; and the policy P-002.2 WAVE_NO_PROGRESS row (both conditions). For B's session only, Ship recognizes WAVE_CLAIM_STATE_INDETERMINATE (wave admission only) and TASK_START_NOT_RECORDED (Step 4.1b) as halts, each reported with the plan's report line and recorded through P-005. Every other installed step, halt, circuit breaker, review, CI, runtime, and merge control remains.
    ```

    <!-- END:e3-wording -->

  * A grant is recognized only when ALL of these hold (amended after plan review attempt 2, to
    stop a self-attested grant, and after attempt 3, to stop trusting an unverified local ref):
    * the file `docs/memory/<yyyy-MM-dd>/orchestrator-2a355f83-e3-grant.md` is read through the
      plan's Verified Main Read: a fresh `git fetch` of `main`; `git remote get-url origin`
      exactly `https://github.com/softwaresalt/backlogit.git`; `git rev-parse origin/main` equal
      to the `git ls-remote origin refs/heads/main` hash; a `git show` pinned to that hash; and,
      for every commit that touched the file,
      `gh api repos/softwaresalt/backlogit/commits/<sha>/pulls` returning a merged pull request
      into `main` (no admin fallback exists);
    * that file quotes the operator's grant verbatim, the grant contains the exact `e3-wording`
      text above, and the file contains a line `B=<B>` naming `B`'s concrete ID;
    * `B`'s own log (read under the plan's scoped P-012 raw-log exception, read set (b)) carries a
      comment whose first line is exactly `BOOTSTRAP_E3_GRANTED: 2A355F83 B=<B>`, which quotes
      the same grant byte-for-byte and names that path.

    A comment alone, an unmerged memory file, or a file reachable only through an unverified ref
    grants nothing.
  * Considered alternative: running `B`'s tasks in non-shipment mode (`frozen_task_ids`) would
    avoid the claim. It is not recommended, because it bypasses the shipment lifecycle the
    exception is bound to, and it would also need new authority.
* **E4 — L1 interplay.** If 186-S (L1 pre-claim checks) ships first, its C check must honor this
  matrix's `B` row. Otherwise the Orchestrator applies the matrix at dispatch.

## Rejected Alternatives

S1, S2, S3, A1, and A3 are rejected for the reasons given above. Adopting 182-F is rejected: it
is out of scope and C-gated. Adding edges to existing shipments is rejected because it repackages
the frozen decomposition.

## Unresolved Questions

* E3 needs an operator decision, which is real new authority.
* Whether the operator wants `B` sequenced explicitly ahead of 187-S. No edge is proposed: C
  already orders the successors.
* **The review gate is halted (2026-10-02).** The plan failed plan-review three times. Attempt 3
  (plan SHA prefix `91305A6BBBF0`) found four P1 contract-consistency gaps, listed in the plan's
  final `## Plan Review` record.
  * Both allowed re-entry cycles are used. Escalation resolved to `ESCALATION_DEGRADED`, so
    Stage halted for the operator.
  * No harvest ran and no shipment exists. No bootstrap shipment ID was assigned, so the
    exception is not yet bound to one.
  * Continuing needs new operator authority: either one more targeted revision and review cycle,
    or a decision to rescope or abandon. The bootstrap authorization alone does not grant it.
  * **Resumed (2026-10-02T01:57:13Z, renewed 02:44:49Z).** The operator authorized resuming the
    halted checkpoint for exactly one targeted fourth revision and review cycle. It is the final
    cycle; no fifth is authorized. It waives no gate and grants no E3 authority. The attempt-4
    outcome is recorded in the plan's final `## Plan Review` record.

## Risks and Mitigations

* **RB1 — Workspace-local log.** A fresh clone or a deleted log loses start records. Mitigation:
  a missing log, or one without the claim event, is `WAVE_CLAIM_STATE_INDETERMINATE` (fail
  closed). It is never read as "not started".
* **RB2 — Partially truncated log.** Loses a start record while the claim event survives.
  Accepted residual (low likelihood, append-only file). Recorded in the plan.
* **RB3 — Mixed binary.** A claim by a pre-U1b binary writes no marker, so all members would be
  residuals (fail closed). Mitigation: E1, plus a version check before claim.
* **RB4 — First chore root.** No prior chore shipment exists in this workspace.
  * The ship path completes the explicit scope generically; feature cascades skip non-features.
  * `covering_feature` is omitted for a chore root.
  * Mitigation: any Ship or Orchestrator rejection of a chore root halts and returns to Stage. It
    is never silently re-typed.
* **RB5 — R2 any-activation-route residual.** Inherited and accepted. An unrelated activation of
  a marked member is still classified by the start record.

## Counters

This is a new objective with its own counters, starting at zero. Prior plan/decomposition review
counters (four cycles each) are untouched and are not replayed. The same error is not allowed
more than three times.

Current values (2026-10-02):

* Plan review: attempts 1-3 FAIL (P1 count 9 → 6 → 4). Attempt 4, the single operator-authorized
  final cycle, is ADVISORY (0 P0, 0 P1, 10 deduplicated P2). Harvest waits for explicit operator
  approval of that ADVISORY; no fifth cycle is authorized.
* Engram CLI bind: the 2 earlier failures are carried forward, and no third bind was attempted.
  The daemon startup chain tripped at 3 twice (both histories kept). After initialization settled,
  one authorized read-only `workspace-status` succeeded, and the Engram CLI served attempt 4.
* Full `verify-workspace`: the 2 earlier failures are carried forward and it was not re-run.
  Config validity was established by a focused schema check instead.

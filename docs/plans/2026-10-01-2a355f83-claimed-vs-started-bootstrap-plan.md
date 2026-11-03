---
chunk_strategy: h1-h2-h3
description: "Implementation plan for the operator-authorized 2A355F83 bootstrap repair: Ship wave admission recognizes claim-assigned members (active, scheduler_baseline_claim equal to the session shipment, listed in the live manifest, no start record in the current claim epoch), Ship Step 4.1b records an idempotent WORK_STARTED comment instead of a redundant move, and the P-002.6 simulation and a real-binary fixture proof cover the new classification"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-10-01-2a355f83-claimed-vs-started-bootstrap-plan.md
title: "Implementation Plan: Claimed-versus-started bootstrap repair for Ship wave admission (2A355F83)"
docline:
    bootstrap_id: 2A355F83
    status: draft
    created_at: 2026-10-02T01:40:00Z
    revised_at: 2026-10-02T03:10:00Z
---

## Objective

Repair the one consumer that ignores the 154-S `scheduler_baseline_claim` marker. That consumer
is Ship wave admission (`.github/agents/_ship.agent.md` Step 4.0, mirrored in P-002.6), together
with its per-task start step (Step 4.1b).

After the repair:

* a member that the claim activated, but that has not been started in the current claim epoch,
  is admissible subject to dependency readiness;
* a task start is durable, idempotent, and distinguishable from a claim;
* every other `active` state still halts fail closed.

The repair is proved with a provenance-checked binary built from the repaired branch, not with
producer-only or read-only tests. It ships as ONE small queued chore shipment `B` under the
bootstrap exception recorded in the source decision.

**Execution precondition, stated up front.** `B` cannot be executed under the installed,
pre-repair Ship contract: its own claim activates its members, and installed Step 4.0 item 4
halts `WAVE_NO_PROGRESS` (`active residual`) at wave 1, exactly as 154-S did. Ship dispatch of `B`
therefore needs the separate operator decision **E3** (source decision, Dispatch Preconditions).
E3 is new authority. It is NOT granted, and nothing in this plan assumes it. Harvest and
shipment assembly do not depend on E3. Only dispatch does.

## Source and Intake Record

* Source decision: `docs/decisions/2026-10-01-2a355f83-claimed-vs-started-bootstrap-deliberation.md`
  (decisions D1–D5, Exception Matrix, Dispatch Preconditions E1–E4, Risks RB1–RB5).
* Operator authorization (verbatim, quoted in the decision):
  'Yes, I authorize that narrowly scoped bootstrap exception'. Received
  `2026-10-01T17:29:30.253-07:00` (`2026-10-02T00:29:30.253Z`).
* Orchestrator assessment reused, not repeated:
  `docs/memory/2026-10-01/orchestrator-shipment-ledger-clarification-memory.md`.
* Producer contract: `docs/design-docs/scheduler-baseline-marker-contract.md`, the v1 predicate,
  its read recipe, and R1–R3. Producer merge `6d233d21` is the 154-S closure.
* No stash entry is consumed. This is an operator-directed work unit, so Step 5.6 archives
  nothing, and no stash entry is created (the stash file is preserved dirty state).
* State at planning:
  * HEAD `e85b85e1387337e1e54cf2b7d4b2cb4575c52232` on `stage/condition-b-enforcement-staging`.
  * `origin/main` `046c01303e76fe45b18093efe548bd8d63ee96af`, which contains `6d233d21`.
  * Live MCP `2c8759c3`, which does NOT contain `6d233d21`.

## Problem Frame

`ClaimShipment` moves every queued member to `active` through
`setArtifactStatusWithClaimMarker(..., StatusActive, "shipment claimed", S)`. That call stamps
`scheduler_baseline_claim = S` and appends a `status_changed` `shipment claimed` event to the
member's item log.

Installed Step 4.0 item 4 then sees `active` members at wave 1 and halts `WAVE_NO_PROGRESS`
(`active residual`). That is the exact halt 154-S hit.

The per-task start has the matching gap. Step 4.1b's move to `active` goes through
`UpdateArtifactWithGate` → `updateArtifactUngated` (not the claim-marker function). For an
already-active member, that move rewrites the file, bumps `updated_at`, and emits
`artifact_mutation` events. It keeps the marker and emits no `status_changed` event. So the move
is not a no-op, but it is indistinguishable from the claim and from any repeat of itself: a claim
and a start cannot be told apart, and a crash between them cannot be classified.

The repair must:

* separate *assigned by claim* from *started by Ship*;
* make admission consume the marker;
* keep every genuine leftover, wrong or missing marker, ambiguity, stale read, and drift fail
  closed.

## Scope

In scope (only):

* `.github/agents/_ship.agent.md`:
  * Step 2 item 1 (its `ready_k` restatement);
  * Step 4.0 items 4, 6, and 7;
  * the "Scheduler replay validation" paragraph;
  * one qualifying sentence in Step 4.1a (the "never stranded `active`" sentence);
  * Step 4.1b;
  * Step 4.6 item 1, narrowed so that its `active` check reads "no member of `ready_k` is
    `active`" (a claim-assigned member still waiting on unfinished dependencies is outside
    `ready_k` and is classified by the next Step 4.0, never halted here).
* `.github/policies/workflow-policies.md`:
  * P-002.6 Definitions (`ready_k`);
  * "Active leftovers halt too";
  * per-wave steps 1 and 2;
  * the existing P-002.2 `WAVE_NO_PROGRESS` row: its first condition rewritten to "no `queued` or
    claim-assigned member has all dependencies terminal", and its second condition narrowed to the
    D2 residual class;
  * two new P-002.2 rows;
  * one version-history row.
* The P-002.6 contract simulation: `scripts/wave-scheduler-sim.ps1` and
  `tests/simulation/wave-scheduler-contract.json`.
* Two new contract tests under `tests/integration/`.
* Two real-binary fixture proofs recorded under `docs/closure/`.

Out of scope (preserved fail closed; no follow-up shipment is created here):

* subtask scheduling, universal removal approval, archive/completion redesign, parent rollup
  rules, gate-off auto-completion, a generalized immutable ledger;
* any producer (Go) change, and any new API, status, event type, or schema field;
* any change to bulk claim activation (accepted M2 stays as is);
* any edit to archived 154-S, any removal of an existing edge, and any repackaging of the frozen
  46-task decomposition (187-S → 188-S → … and side branch 186-S);
* `.autoharness/harness-manifest.yaml`. Both edited files already carry `drift_allowed: true`. No
  rule requires a `drift_reason` change, and pinning a bookkeeping token there would add a
  same-wave file overlap;
* `build-feature` and `harness-architect`. A targeted check found no wave-admission or
  active-residual wording in `build-feature`;
* the `plugin/agents/` copies. `plugin/agents/ship.agent.md` is a separately generated plugin
  surface, not a mirror that this contract edit must keep in sync;
* every other Step 4.6 line. Only item 1's `active` check changes (in scope above). The
  circuit-breaker row "Active residual at wave admission" is unchanged: it still applies to the
  D2 residual class.

## Requirements Trace

| ID | Requirement | Units |
|---|---|---|
| R1 | Claim-assigned is recognized only as `active` + marker == `S` + listed in `S`'s live `custom_fields.items` + no valid `WORK_STARTED: S` record in the current start epoch | UCS2a, UCS2b, UCS4, UCS5a |
| R2 | Started, wrong or missing marker, and out-of-manifest `active` members halt `WAVE_NO_PROGRESS` (`active residual`) | UCS2a, UCS2b, UCS4, UCS5b |
| R3 | Shipment ambiguity, manifest drift, a stale or errored read, R3 marker conditions, and a marker == `S` member whose log is missing, unparseable, or lacks the claim event halt `WAVE_CLAIM_STATE_INDETERMINATE`, which takes precedence over a residual | UCS2a, UCS2b, UCS4, UCS5b |
| R4 | Frontier `ready_k = { t in queued or claim-assigned : every dependency terminal_success }`; an assigned member with unfinished dependencies waits | UCS2a, UCS2b, UCS4, UCS5a |
| R5 | Task start is an idempotent `WORK_STARTED: S` comment by actor `ship`; a claim-assigned member is never moved again; nothing dispatches before the record is verified; an absent or duplicate record halts `TASK_START_NOT_RECORDED` | UCS2a, UCS2b, UCS5a |
| R6 | Every other admission rule, R1 and R2 of the marker contract, and completion semantics are unchanged | UCS1, UCS3 (Preserved guards) |
| R7 | Real consumer use is proved with a provenance-checked binary that contains `6d233d21` | UCS5a, UCS5b, post-merge closure |

### Start semantics (D1, restated for implementers)

* **Item log.** `logs/<id>.jsonl` under the workspace storage root that the serving backlogit
  binary uses. In this workspace that is `.backlogit/`; a fresh `init` creates `.backlog/`, so the
  name is never assumed. The Orchestrator records the absolute served storage root in the
  dispatch text, and Ship reads `<served storage root>\logs\<id>.jsonl` from it. Any CLI fallback
  uses `--cwd "<served workspace root>"`. The root is never a worktree-relative guess.
* **Raw-log read, scoped P-012 exception.** No configured backlog tool returns an item's full
  event stream with `actor` and `delta`, so these reads are a declared, scoped exception to
  P-012 (no ad hoc filesystem read when a configured tool is available). The exception covers
  exactly two read-only sets, always under the served storage root, and nothing else:
  * **(a) Task evidence.** The item logs `<served storage root>\logs\<id>.jsonl` of `S`'s
    members, read at Step 4.0 classification and at Step 4.1b start-record verification.
  * **(b) The bootstrap shipment's own log.** `<served storage root>\logs\<B>.jsonl`, read for
    the in-force, revocation, and E3 recognition checks at each of the four dispatch checkpoints:
    before `B`'s claim, at every wave admission of `B`, before pull request creation, and before
    merge.

  Every other backlog read still goes through the configured tools. Each use is declared in the
  session record as `P-012 SCOPED RAW-LOG READ (2A355F83): <path>`. If a configured tool later
  returns that stream, the tool is used instead.
* **Log parse policy.**
  * Lines are read in file order. One trailing `\r` is stripped from each line, and blank lines
    are ignored.
  * Every other line must parse as a JSON object. One malformed non-blank line makes the whole
    log unparseable.
  * Only these fields are read: `event_type`, `actor`, `delta.reason`, `delta.to`, and
    `delta.comment`.
* **Start epoch.** The epoch begins at the latest `status_changed` event with `delta.to` `active`
  and `delta.reason` `shipment claimed`. If the log has no such event, the epoch is the whole
  log. A re-claim therefore starts a fresh epoch, and a returned-then-re-claimed member is
  re-admitted exactly as a returned `queued` member is today. Two accepted edge cases:
  * A member removed from and re-added to the same active shipment without a re-claim keeps its
    epoch. If it already has a start record it is an active residual, which fails closed.
  * A member started in an earlier epoch and then re-claimed is claim-assigned in the new epoch,
    which is the same treatment a returned `queued` member gets today. The simulation does not
    model multiple epochs; this is a documented acceptance, not a tested behavior.
* **Valid start record.** A `comment` event inside the epoch whose `actor` is `ship` and whose
  `delta.comment` first line (split at `\n`, one trailing `\r` stripped) is exactly
  `WORK_STARTED: <S>`.
* **Step 4.1b, shipment mode (session shipment `S`).**
  * A claim-assigned member is never moved again.
  * A `queued` ready member keeps the existing move to `active`.
  * In both cases, Ship then:
    1. reads the log;
    2. appends the record only when no valid record exists, through `backlogit_append_comment`
       with `{item_id: <t>, actor: "ship", comment: "WORK_STARTED: <S>"}`;
    3. if the MCP append errors, re-reads the log before any CLI fallback, and appends through
       `backlogit comment add <t> --actor ship --comment "WORK_STARTED: <S>"` only if the
       re-read still shows no valid record (the CLI binary must pass the E1 provenance check);
    4. re-reads the log and requires exactly one valid record.
  * Zero records, more than one record, or an unparseable log halts `TASK_START_NOT_RECORDED`.
    Recovery from a duplicate is an operator decision; Ship never deletes a log line.
  * **H1 ordering.** No `build-feature` dispatch, and no Step 4.1c or later action for the task,
    happens before step 4 confirms exactly one valid record. "No valid record" therefore always
    means "nothing dispatched for this task in this epoch".
* **Step 4.1b, non-shipment mode** (`frozen_task_ids`, no session shipment). Nothing can be
  claim-assigned, so behavior is unchanged: move to `active`, and no start record is written.
  Every `active` member at admission stays an active residual.
* No new API is introduced. The existing comment operation is reused.

### Admission classification (D2, restated for implementers)

When any member of `M` is `active` at Step 4.0, Ship classifies before any other active check.
Indeterminate takes precedence: if any member is indeterminate, halt with
`WAVE_CLAIM_STATE_INDETERMINATE` and report the residuals too.

* **Indeterminate** (`WAVE_CLAIM_STATE_INDETERMINATE`, wave admission only):
  * the live active-shipment count is not exactly 1, or the single active shipment is not `S`;
  * two reads of `S`'s `custom_fields.items` within the step differ (manifest drift);
  * an item read errors, or its status disagrees with the Step 4.0 snapshot (stale read);
  * any R3 condition of the marker contract holds when the marker is read through the
    contract's read recipe;
  * a marker == `S` member's log is missing or unparseable, or lacks a claim event.
* **Claim-assigned:** `active`, marker == `S`, explicitly listed in `S`'s live
  `custom_fields.items`, and no valid start record in the current epoch.
* **Active residual** (`WAVE_NO_PROGRESS`, detail `active residual`): every other `active`
  member. That covers a valid start record in the current epoch, a missing or non-`S` marker,
  and a member absent from the live manifest.

## Implementation Units

Every unit follows the 2-hour rule: fewer than 3 files, fewer than 5 functions, and 3 or fewer
scenarios. Every unit is test-first: the two red harnesses (UCS1, UCS3) land before the edits
they gate.

No unit introduces a production Go declaration, so no AST harness or production stub is needed.
No stub opens, so none needs closing.

**Harness classification (closed set, P-002.1/P-002.6).** Every task carries exactly one
classification, and harness-architect scaffolds only the two red deliverables:

| Task | Classification | Harness owner | Loop command (`harness_cmd`) | Gate command |
|---|---|---|---|---|
| UCS1 | red deliverable (`red-deliverable-contract`) | itself | its `red_selector_command` | RED on `Ship` and `Policy`, `Preserved` green |
| UCS3 | red deliverable (`red-deliverable-contract`) | itself | its `red_selector_command` | RED on `Fixture` and `Replay`, `Preserved` green |
| UCS2a | `harness-exempt`, class `covered-by` | UCS1 | its `harness_owner_command` | its `exempt_verification_command` (`EXEMPT_VERIFY_OK:<UCS2a>`) |
| UCS2b | `harness-exempt`, class `covered-by` | UCS1 | its `harness_owner_command` | its `exempt_verification_command` (`EXEMPT_VERIFY_OK:<UCS2b>`) |
| UCS4 | `harness-exempt`, class `covered-by` | UCS3 | its `harness_owner_command` | its `exempt_verification_command` (`EXEMPT_VERIFY_OK:<UCS4>`) |
| UCS5a | `harness-exempt`, class `verification-only` | none | its `exempt_verification_command` | the same command |
| UCS5b | `harness-exempt`, class `verification-only` | none | its `exempt_verification_command` | the same command |

This is the documented pattern in Ship Step 4.0 ("a `covered-by` owner such as a harness-only
contract unit"). Each owner is a direct dependency of its covered task, is not itself exempt, and
is a red deliverable whose red evidence lands in wave 1, so the claim-time `covered-by` checks
(Ship Step 4.1a item 2) are satisfiable at wave 2. Every `covered-by` delta is non-test files
named in the task, with no `*_test.go` change (P-002.4). The closed exempt set for this plan is
{UCS2a, UCS2b, UCS4, UCS5a, UCS5b}.

Unit names follow build-feature Step 0.5: unit `CS1` maps to selector `^TestUCS1_`, and unit
`CS3` to `^TestUCS3_`. No `TestUCS` function exists in `tests/integration` today.

Contract tests:

* read files through the existing `testRepoRoot(t)` helper
  (`tests/integration/plugin_manifest_test.go`);
* slice the named section and normalize whitespace;
* assert stable literal tokens only (no line numbers, no prose paraphrase).

Each test runs a `Preserved` subtest first, which uses `require` and is already green. The red
token lists then use `assert`, so every missing token is reported in one run. Helpers are local
closures, or package functions prefixed `ucs1` or `ucs3`, to avoid collisions in the shared
package.

Section slices. Every start and end anchor is matched at the beginning of a line (`(?m)^`), and
each anchor must match exactly once (`require`), so a heading phrase quoted inside prose can never
open or close a slice:

* **Ship Step 2:** from `### Step 2: Harness Generation` to `#### Step 2a:`.
* **Ship Step 4.0:** from `#### Step 4.0: Wave Admission (P-002.6)` to `#### Step 4.1a`. The
  slice includes item 7 and the replay paragraph.
* **Ship Step 4.1a:** from `#### Step 4.1a` to `#### Step 4.1b`.
* **Ship Step 4.1b:** from `#### Step 4.1b: Claim Task` to `#### Step 4.1c`.
* **Ship Step 4.6:** from `#### Step 4.6: Wave Convergence Gate (P-002.6)` to
  `### Step 5: PR Lifecycle`.
* **Policy P-002.6:** from `### P-002.6 — Dependency-Aware Harness Waves (Scheduling Contract)`
  to the next line that starts with `### `.
* **Policy P-002.2 row `X`:** the table lines that start with `` | `X` | ``. Exactly one line
  must match. In `Preserved` the count is `require`d; in a red subtest the count is `assert`ed,
  and that row's content checks are skipped when the count is not one (so the failure is
  reported once, not masked).

Whitespace is normalized (every run of whitespace, including CRLF, becomes one space) before any
`Contains` or `NotContains` check.

Shared condition-token set `K` is asserted word-for-word in both the Ship Step 4.0 slice and the
P-002.6 slice:

* `claim-assigned`, `scheduler_baseline_claim`, `custom_fields.items`;
* `WORK_STARTED:`, `start epoch`, `shipment claimed`, `logs/<id>.jsonl`;
* `manifest drift`, `stale read`;
* `WAVE_CLAIM_STATE_INDETERMINATE`, `active residual`;
* `queued or claim-assigned`, `fail closed`.

Execution safety modes (`safety-modes` skill):

* Careful mode for UCS2a, UCS2b, and UCS4: they change the admission contract and its model.
* Freeze-scope for UCS5a and UCS5b: their delta is exactly one evidence file each.

### UCS1: Ship and policy claim-start contract harness (tests, red deliverable)

* File: `tests/integration/claim_start_admission_contract_test.go` (new), package
  `integration_test`.
* Function `TestUCS1_ClaimStartContract` has three subtests, in order:
  1. `Preserved` (H2 characterization, green from the start, `require`):
     * The Ship Step 4.0 slice contains `WAVE_MEMBER_BLOCKED`, `WAVE_STATUS_UNSUPPORTED`,
       `WAVE_CYCLE_DETECTED`, `WAVE_BUDGET_EXCEEDED`, and `terminal_success = M`.
     * The Ship file contains exactly one line `#### Step 4.1b: Claim Task` (the heading is kept
       verbatim, so the slice anchor cannot drift).
     * The P-002.6 slice contains `"No ready tasks" is never a completion condition`.
     * **Log-fact pin.** `internal/core/shipment_lifecycle.go` contains
       `models.StatusActive, "shipment claimed", shipmentID`, and `internal/core/commits.go`
       contains `EventType: "comment",` and `Delta: map[string]any{"comment": comment},` (after
       whitespace normalization). The start-epoch anchor and the start-record shape depend on
       these producer literals, so a producer rename fails this guard instead of silently
       breaking classification.
  2. `Ship` (red, `assert`):
     * The Step 4.0 slice contains every token in `K`, and contains
       `WAVE_CLAIM_STATE_INDETERMINATE` at least 3 times (classification, item 7, and replay
       paragraph).
     * The Step 2 slice contains `queued or claim-assigned`.
     * The Step 4.1a slice contains `claim-assigned`.
     * The Step 4.1b slice contains each exact phrase:
       `WORK_STARTED:`, `backlogit_append_comment`, `backlogit comment add`, `--actor ship`,
       `TASK_START_NOT_RECORDED`, `P-005`, `A claim-assigned task is never moved again`,
       `only when no valid start record exists`, `re-reads the item log before any CLI fallback`,
       `exactly one valid start record`, and `before any dispatch`.
     * **Removed-sentence assertions (`assert.NotContains`, whitespace-normalized).** None of
       these pre-repair sentences survives:
       * Step 4.0 slice: ``ready_k = { t in queued : every dependency of t is terminal_success }``;
       * Step 4.0 slice: ``If any member is still `active` at wave admission, it is an unfinished
         claim from a prior wave, not progress``;
       * Step 2 slice: ``the queued tasks of the target feature or chore whose dependencies are``;
       * Step 4.1b slice: ``Update task status to `active` using the backlog tool's move
         operation.``
     * The Step 4.6 slice contains ``no member of `ready_k` is `active` `` and
       ``A member of `ready_k` still `active` ``, and does not contain
       ``and no member is `active`, `blocked`, or in an unsupported status`` (the pre-repair item 1
       wording, which would halt a claim-assigned member that is still waiting on dependencies).
  3. `Policy` (red, `assert`):
     * The P-002.6 slice contains every token in `K`.
     * **Removed-sentence assertions (`assert.NotContains`, whitespace-normalized):**
       * ``ready_k = { t ∈ queued : deps(t) ⊆ terminal_success }``;
       * ``If `active` is non-empty → **halt** with `WAVE_NO_PROGRESS` ``;
       * ``it never treats an `active` member as satisfied``;
       * ``A member still carrying `active` at wave admission is a claim from a prior wave that
         never reached `done` ``;
       * ``The scheduler never admits a new wave over an unfinished claim``.
     * The single P-002.2 row for `WAVE_CLAIM_STATE_INDETERMINATE` contains
       `wave admission only`.
     * The single row for `TASK_START_NOT_RECORDED` contains `Step 4.1b`.
     * The single row for `WAVE_NO_PROGRESS`:
       * contains ``no `queued` or claim-assigned member has all dependencies terminal`` (the
         rewritten first condition) and `claim-assigned`;
       * does not contain ``no `queued` member has all dependencies terminal`` (the pre-repair
         first condition, which contradicts the repaired `ready_k`);
       * does not contain ``**or** a member is still `active` at wave admission`` (the pre-repair
         second condition).
* It compiles against the current tree (stdlib, `testify`, and `testRepoRoot` only), and it fails
  on named assertions. It is never a build error.
* RED: `go test -count=1 -run '^TestUCS1_' ./tests/integration` fails with `--- FAIL:` lines for
  `Ship` and `Policy`. `Preserved` passes.
* Green makers: UCS2a (`Ship`) and UCS2b (`Policy`), both in wave 2.
* Posture: test-first (red deliverable).

Body block, written at harvest with concrete IDs:

<!-- BEGIN:red-deliverable-contract -->

```text
red_deliverable: true
red_deliverable_reason: Deliverable IS the persistent RED claim-start contract harness for Ship Step 4.0/4.1b and policy P-002.6/P-002.2; its Preserved subtest is green, and it is driven green only by <UCS2a> (Ship subtest) and <UCS2b> (Policy subtest) in wave 2.
red_selector_command: go test -count=1 -run '^TestUCS1_' ./tests/integration
green_maker_tasks: <UCS2a>, <UCS2b>
green_maker_closes_wave: 2
```

<!-- END:red-deliverable-contract -->

### UCS2a: Ship Step 4.0 and Step 4.1b claim-start edit (covered-by UCS1)

* File: `.github/agents/_ship.agent.md` only.
* Step 4.0 item 4 is rewritten as **Classify active members, then halt on residuals**. It states
  the D2 classification exactly as restated above:
  * the indeterminate conditions, with indeterminate taking precedence;
  * the claim-assigned definition;
  * the active-residual class;
  * a missing or unparseable log, or one without the claim event, is never read as "not started"
    and always fails closed.
* Step 4.0 item 6: the frontier becomes `ready_k = { t in queued or claim-assigned : every
  dependency of t is terminal_success }`. A claim-assigned member with unfinished dependencies
  waits, and is neither progress nor a residual.
* Step 4.0 item 7: add `WAVE_CLAIM_STATE_INDETERMINATE` to the deterministic halt report. Each
  indeterminate member is reported with its reason, and recorded through P-005.
* Scheduler replay paragraph: the replay must also halt `WAVE_CLAIM_STATE_INDETERMINATE` on
  injected shipment ambiguity, and must admit claim-assigned members.
* Step 4.1a: qualify the "never stranded `active`" sentence for claim-assigned tasks, in one
  sentence. Such a task stays claim-assigned, with no start record.
* Step 4.1b: the start semantics restated above, using the exact phrases that UCS1 `Ship`
  asserts (`A claim-assigned task is never moved again`, `only when no valid start record
  exists`, `re-reads the item log before any CLI fallback`, `exactly one valid start record`,
  `before any dispatch`, and `P-005`), including:
  * the shipment-mode and non-shipment-mode paths;
  * the exact MCP payload and CLI fallback;
  * re-read after an MCP error, before any fallback append;
  * the sentence "A claim-assigned task is never moved again";
  * the halt `TASK_START_NOT_RECORDED`, with a Report line naming the task, `S`, and the
    observed record count;
  * the H1 ordering rule, using the words "before any dispatch".
* Step 2 item 1: `ready_k` becomes "the queued or claim-assigned tasks of the target feature or
  chore whose dependencies are **all** `done`", matching Step 4.0 item 6.
* Step 4.6 item 1 (mandatory): the first sentence becomes "Every member of `ready_k` is `done`
  (or `archived`), no member of `ready_k` is `active`, and no member is `blocked` or in an
  unsupported status." The halt sentence becomes "A member of `ready_k` still `active` → halt
  with `WAVE_NO_PROGRESS` (detail: `active residual`)." A claim-assigned member waiting on
  unfinished dependencies is outside `ready_k`, so convergence never halts on it; the next
  Step 4.0 classifies it. The `blocked` and unsupported checks are unchanged. No other Step 4.6
  line changes. Steps 0.5 and 3 are untouched, and the `#### Step 4.1b: Claim Task` heading is
  kept verbatim.
* Size: one file, no functions, and one harness subtest (`Ship`), so the unit is within the
  2-hour rule. Splitting admission from start was considered and rejected: the two halves edit
  the same file, so they would have to run in sequential waves, adding a fourth wave and a
  same-file handoff for a text-only edit. The admission and start text must also stay mutually
  consistent (H1), which is easier to review in one diff.
* Posture: test-first (turns UCS1 `Ship` green). Careful mode.

<!-- BEGIN:harness-exemption-contract -->

```text
harness_exemption_class: covered-by
harness_exemption_reason: Edits only .github/agents/_ship.agent.md (Step 2 item 1, Step 4.0 items 4/6/7 and replay paragraph, one Step 4.1a sentence, Step 4.1b, Step 4.6 item 1) so that the Ship subtest of predecessor harness owner <UCS1> turns green; adds or modifies no *_test.go file.
harness_owner: <UCS1>
exempt_verification_command: $o = go test -count=1 -v -run '^TestUCS1_ClaimStartContract$/^(Preserved|Ship)$' ./tests/integration 2>&1 | Out-String; if ($LASTEXITCODE -ne 0) { Write-Output $o; Write-Error 'UCS1 Ship not green'; exit 1 }; foreach ($k in @('--- PASS: TestUCS1_ClaimStartContract/Preserved','--- PASS: TestUCS1_ClaimStartContract/Ship')) { if (-not $o.Contains($k)) { Write-Error "missing $k"; exit 1 } }; if ($o -match '--- FAIL:|--- SKIP:') { Write-Error 'fail or skip present'; exit 1 }; Write-Output 'EXEMPT_VERIFY_OK:<UCS2a>'
exempt_precondition: must-fail-before-deliverable
harness_owner_command: go test -count=1 -v -run '^TestUCS1_ClaimStartContract$/^(Preserved|Ship)$' ./tests/integration
```

<!-- END:harness-exemption-contract -->

### UCS2b: Policy P-002.6 and P-002.2 claim-start edit (covered-by UCS1)

* File: `.github/policies/workflow-policies.md` only.
* P-002.6:
  * Definitions: `ready_k = { t ∈ queued or claim-assigned : deps(t) ⊆ terminal_success }`.
  * "Active leftovers halt too": the D2 classification, word-for-word consistent with Ship
    (token set `K`). The two pre-repair sentences ("A member still carrying `active` at wave
    admission is a claim from a prior wave that never reached `done`" and "The scheduler never
    admits a new wave over an unfinished claim") are removed, and neither phrase is reused; the
    rewrite speaks of an active residual instead.
  * Per-wave step 1: classify before halting on `active`.
  * Per-wave step 2: compute the frontier over queued or claim-assigned members.
* P-002.2:
  * Rewrite the existing `WAVE_NO_PROGRESS` row's **first** condition from "no `queued` member
    has all dependencies terminal" to "no `queued` or claim-assigned member has all dependencies
    terminal", matching the repaired `ready_k`.
  * Narrow the same row's **second** condition to an `active` member that is not claim-assigned
    (the D2 residual class).
  * Add `WAVE_CLAIM_STATE_INDETERMINATE`, scope `**wave admission only** (Ship Step 4.0,
    P-002.6)`, condition per D2, with a report line naming each indeterminate member and its
    reason.
  * Add `TASK_START_NOT_RECORDED`, scope `per-task start (Ship Step 4.1b)`, with a report line
    naming the task, the shipment, and the observed record count.
  * Use the existing four-column row shape.
* Version history: one row at the next available minor version at implementation time (currently
  `1.31.0`, so `1.32.0` unless another change landed first), citing 2A355F83.
* Posture: test-first (turns UCS1 `Policy` green). Careful mode.

<!-- BEGIN:harness-exemption-contract -->

```text
harness_exemption_class: covered-by
harness_exemption_reason: Edits only .github/policies/workflow-policies.md (P-002.6 ready_k, Active leftovers, per-wave steps 1-2, both conditions of the WAVE_NO_PROGRESS row, two new P-002.2 rows, one version row) so that the Policy subtest of predecessor harness owner <UCS1> turns green; adds or modifies no *_test.go file.
harness_owner: <UCS1>
exempt_verification_command: $o = go test -count=1 -v -run '^TestUCS1_ClaimStartContract$/^(Preserved|Policy)$' ./tests/integration 2>&1 | Out-String; if ($LASTEXITCODE -ne 0) { Write-Output $o; Write-Error 'UCS1 Policy not green'; exit 1 }; foreach ($k in @('--- PASS: TestUCS1_ClaimStartContract/Preserved','--- PASS: TestUCS1_ClaimStartContract/Policy')) { if (-not $o.Contains($k)) { Write-Error "missing $k"; exit 1 } }; if ($o -match '--- FAIL:|--- SKIP:') { Write-Error 'fail or skip present'; exit 1 }; Write-Output 'EXEMPT_VERIFY_OK:<UCS2b>'
exempt_precondition: must-fail-before-deliverable
harness_owner_command: go test -count=1 -v -run '^TestUCS1_ClaimStartContract$/^(Preserved|Policy)$' ./tests/integration
```

<!-- END:harness-exemption-contract -->

### UCS3: Wave-simulation claim-start harness (tests, red deliverable)

* File: `tests/integration/claim_start_wave_sim_contract_test.go` (new), package
  `integration_test`.
* Function `TestUCS3_WaveSimClaimStart` has three subtests, in order:
  1. `Preserved` (`require`): decode `tests/simulation/wave-scheduler-contract.json` into a typed
     struct (`halt_wave *int`). Require that the 21 existing scenario IDs are still present.
  2. `Fixture` (red, `assert`): require three scenarios by `id`:
     * `claim_assigned_all`:
       * expects outcome `COMPLETE`, with the same `waves` and `scheduled` values as the baseline
         scenario;
       * expects `claim_assigned_at_admission` equal to that `scheduled` value. This observable
         is new. An unknown expect key fails today as `<no such observable>`, so a model that
         ignores the claim cannot pass it.
     * `claim_state_residuals`: expects outcome `WAVE_NO_PROGRESS`, halt detail
       `active residual`, and three `active_ids`.
     * `claim_state_indeterminate`: expects outcome `WAVE_CLAIM_STATE_INDETERMINATE` and
       halt wave 1.
  3. `Replay` (red, `assert`):
     * Run the full simulation once:
       `pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1` through `exec.CommandContext`, with
       a 2-minute timeout, `cmd.Dir` set to the repo root, and `NO_COLOR=1` added to the
       inherited environment. ANSI escape sequences are stripped from the output before matching.
     * The exit code is taken with `errors.As(err, &exitErr)` on `*exec.ExitError`; any other
       error (including the timeout) fails the subtest with the captured output.
     * Require exit 0, a final `WAVE_SIM_OK:` line, and no `WAVE_SIM_FAIL` line.
     * After CRLF normalization, require exactly 24 lines matching `(?m)^\s*\S+\s+outcome=`
       (21 existing plus 3 new scenarios), and one line matching
       `(?m)^\s*<id>\s+outcome=<expected>(\s|$)` for each of the three IDs.
     * This exercises all 24 scenarios and closes the vacuous-pass gap (an unknown `-Scenario`
       still exits 0 today).
     * If `exec.LookPath("pwsh")` fails, call `t.Fatalf`, never `t.Skip`.
* Scenario lookups in `Fixture` go through a `require`d find-by-ID before any field is indexed,
  so a missing scenario is a named failure, never a panic.
* Helpers are closures inside the test function, or package functions prefixed `ucs3`; at most
  two helpers.
* RED: `go test -count=1 -run '^TestUCS3_' ./tests/integration` fails on named assertions in
  `Fixture` and `Replay`. `Preserved` passes.
* Green maker: UCS4 (wave 2).
* Posture: test-first (red deliverable).

<!-- BEGIN:red-deliverable-contract -->

```text
red_deliverable: true
red_deliverable_reason: Deliverable IS the persistent RED wave-simulation claim-start harness (fixture plus full replay); its Preserved subtest is green, and it is driven green only by <UCS4> landing the three scenarios and the model change in wave 2.
red_selector_command: go test -count=1 -run '^TestUCS3_' ./tests/integration
green_maker_tasks: <UCS4>
green_maker_closes_wave: 2
```

<!-- END:red-deliverable-contract -->

### UCS4: Wave-simulation claim-start model (covered-by UCS3)

* Files: `scripts/wave-scheduler-sim.ps1` and `tests/simulation/wave-scheduler-contract.json`.
  The model and the scenario data it executes are one testable unit, and UCS3 is their harness.
* One new mutation key, `claim_overrides`, a single map:
  * `claim_all` (shipment ID): every non-terminal member becomes `active`, with marker == the ID,
    a claim event, and live-manifest membership;
  * `started` (IDs): a valid start record in the current epoch;
  * `marker` (ID → value);
  * `off_manifest` (IDs);
  * `active_shipments` (list).
* Functions, at most four:
  * new `Apply-ClaimOverrides` (projection mutation);
  * new `Get-ClaimStateClass` (the D2 partition, indeterminate first);
  * changed `Invoke-WaveScheduler` (classify before the active-residual halt; frontier over
    queued or claim-assigned);
  * changed `Test-Scenario` (the `claim_assigned_at_admission` observable).
* Scenarios:
  * `claim_assigned_all`: `claim_all` only.
  * `claim_state_residuals`: `claim_all`, plus one `started` member, one `marker` mismatch, and
    one `off_manifest` member.
  * `claim_state_indeterminate`: `claim_all`, plus `active_shipments` naming two shipments.
* All 21 existing scenarios, and `-VerifyAgainstQueue`, must remain green unchanged.
* **Default when `claim_overrides` is absent:** exactly one active shipment, equal to the
  scenario's shipment; no member carries a marker, a claim event, or a start record. Under D2
  every `active` member is then an active residual, so the existing `active_residual` scenario
  keeps its current outcome and detail without any fixture change.
* Within the task, add the scenarios first and observe `WAVE_SIM_FAIL` naming them, then
  implement.
* Posture: test-first (turns UCS3 green). Careful mode.
* Accepted consequence: the integration package now needs `pwsh` on `PATH`. The GitHub-hosted
  runners provide PowerShell 7.

<!-- BEGIN:harness-exemption-contract -->

```text
harness_exemption_class: covered-by
harness_exemption_reason: Edits only scripts/wave-scheduler-sim.ps1 and tests/simulation/wave-scheduler-contract.json (claim_overrides map, D2 classification, three scenarios) so that the Fixture and Replay subtests of predecessor harness owner <UCS3> turn green; adds or modifies no *_test.go file.
harness_owner: <UCS3>
exempt_verification_command: $o = go test -count=1 -v -run '^TestUCS3_WaveSimClaimStart$' ./tests/integration 2>&1 | Out-String; if ($LASTEXITCODE -ne 0) { Write-Output $o; Write-Error 'UCS3 not green'; exit 1 }; foreach ($k in @('--- PASS: TestUCS3_WaveSimClaimStart/Preserved','--- PASS: TestUCS3_WaveSimClaimStart/Fixture','--- PASS: TestUCS3_WaveSimClaimStart/Replay')) { if (-not $o.Contains($k)) { Write-Error "missing $k"; exit 1 } }; if ($o -match '--- FAIL:|--- SKIP:') { Write-Error 'fail or skip present'; exit 1 }; Write-Output 'EXEMPT_VERIFY_OK:<UCS4>'
exempt_precondition: must-fail-before-deliverable
harness_owner_command: go test -count=1 -v -run '^TestUCS3_WaveSimClaimStart$' ./tests/integration
```

<!-- END:harness-exemption-contract -->

### Proof Protocol (shared by UCS5a and UCS5b)

The executing agent applies the repaired Step 4.0 and Step 4.1b text from the branch HEAD, step
by step, to isolated fixture workspaces, and records every read and decision. The protocol below
is mandatory. Any failed check aborts the proof with the halt token `PROOF_CONTAINMENT_BREACH`
(for steps 2, 3, and 8) or `PROOF_PRECHECK_FAILED` (for every other step), recorded through P-005
telemetry with the report line
`PROOF ABORT {task_id} step {n}: {token} — {observed} (expected {expected})`. It never falls back
to the live workspace or the live MCP server.

Each proof unit performs ONE provenance build and ONE baseline/containment pair. Each row then
uses its own fresh fixture workspace under the same `$run`.

1. **Per-run root.** `$run` is
   `<absolute repo root>\logs\2a355f83-proof\<task-id>-<UTC yyyyMMddTHHmmssZ>`, created new. An
   existing directory is never reused or overwritten.
   `git check-ignore -v "$run\probe"` must name the `.gitignore` line `logs/`.
2. **Live-state baseline.** No other backlog session (agent or operator) may run against this
   workspace during the proof window; the executing agent records that it is the only active
   session. Then record:
   * `git status --porcelain --ignored -- .backlogit`;
   * a SHA-256 manifest of every file under the live storage root `.backlogit\`, recursively,
     except `backlogit.db`, `backlogit.db-shm`, and `backlogit.db-wal` (the derived index, which
     the live server may refresh on its own).
3. **No live tools.** From step 2 until step 8, no MCP backlog tool is called. Every backlog
   operation in the proof uses the fixture binary from step 4 with an absolute
   `--cwd "<fixture ws>"`. `$env:BACKLOGIT_WORKSPACE_DIR` must be unset for the whole window
   (`Test-Path Env:BACKLOGIT_WORKSPACE_DIR` is false), and that observation is recorded.
4. **Provenance build** (once per unit), from a clean private clone so that preserved or
   unrelated dirty state in the working checkout can neither leak into the binary nor block the
   proof:
   * `$c = git rev-parse HEAD`. The task's prior work must already be committed; uncommitted
     changes are simply absent from the clone and therefore from the binary.
   * `git clone --quiet "<absolute repo root>" "$run\src"`, then
     `git -C "$run\src" checkout --quiet --detach $c`, then `git -C "$run\src" status --porcelain`
     must print nothing (untracked files included). The clone is a separate repository inside
     the ignored `$run`, not a worktree (P-016 is unaffected), and is never pushed.
   * `go -C "$run\src" build -ldflags "-X github.com/softwaresalt/backlogit/internal/version.Commit=$c" -o "$run\bin\backlogit.exe" ./cmd/backlogit`.
   * `go version -m "$run\bin\backlogit.exe"` must show `vcs.revision=$c` and
     `vcs.modified=false`. This stamp, not the version string, is the dirty check. (Go counts
     untracked files as modifications, which is why the clone's full status must be empty.)
   * `& "$run\bin\backlogit.exe" version --format json --no-update-check` must report commit
     `$c` (40 hex characters).
   * `git merge-base --is-ancestor 6d233d21162a072ddbdfecb52ec62a8fb8a63793 $c` must exit 0.
   * The evidence file is written only after step 8, so it is never part of the built commit.
5. **Fixture init** (per row). For row `r`, `$ws = "$run\ws-<r>"`. `Test-Path $ws` must be false.
   Run `& "$run\bin\backlogit.exe" init "$ws"`. Afterwards:
   * `Test-Path "$ws\.backlog"` must be true and `Test-Path "$ws\.backlogit"` must be false
     (`init` creates `.backlog`; two roots would be ambiguous and are refused by the resolver);
   * `list --format json --cwd "$ws"` must return zero items;
   * the fixture item logs are `$ws\.backlog\logs\<id>.jsonl`.

   The resolver never searches parent directories, so a fixture root cannot fall through to the
   live `.backlogit`.
6. **Setup** (per row), with the fixture binary and `--cwd "$ws"`:
   * `add` two tasks, `T1` and `T2`, under a fixture root;
   * `dep add T2 T1`;
   * `shipment create` holding `T1` and `T2`;
   * `shipment claim <S>`.

   Confirm each flag shape with `--help` on the fixture binary, and record the exact commands.
   Fixture reads follow the marker contract's read recipe through the CLI
   (`shipment list --status active --format json`, then `get <id> --format json`), which reads
   item frontmatter.
7. **Rows.** Run the unit's three rows (below), each in its own `$ws`.
8. **Containment check** (once per unit, after the last row). Re-take both step-2 records. Any
   difference aborts with `PROOF_CONTAINMENT_BREACH`. No row deletes a file. Fixture removal
   needs explicit operator approval.

Each evidence file is a docs-lint-valid document: docline frontmatter (`title`, `description`,
`doc_type: closure`, `schema_version`, `source`) and one H1, followed by these lines, each
starting at column 0:

* `PROOF_SCOPE: 2A355F83`;
* `PROOF_BINARY_COMMIT: <40-hex>` and `PROOF_BINARY_DIRTY: false`;
* `PROOF_WORKSPACE_OVERRIDE: UNSET`;
* `PROOF_LIVE_STATE: UNCHANGED`;
* one `ROW <name>: PASS` line per row, each followed by its commands and an output excerpt;
* `PROOF_RESULT: PASS`.

`scripts/md-lint.ps1` reports zero violations on the evidence file before it is committed.

### UCS5a: Real-binary positive claim-start proof (verification-only, harness-exempt)

* Delta: `docs/closure/<B>-claim-start-proof-positive.md` (new) only. Freeze-scope.
* Each row uses its own fresh fixture `$run\ws-<row>` (protocol steps 5–6) under the unit's single
  build and single containment pair.
* Rows:
  * `claim-marks-members`: both members are `active`, with marker `S` and one claim event each.
  * `wave1-admits-claim-assigned`: wave-1 classification admits `T1` as claim-assigned. `T2` is
    claim-assigned and waits on `T1`. No `WAVE_NO_PROGRESS` and no indeterminate result.
  * `start-recorded-once`: Step 4.1b on `T1`, applied twice, leaves exactly one valid
    `WORK_STARTED: S` record. `T1` stays `active`, and no new `status_changed` event appears.
* Posture: verification-only. Commits evidence only.

<!-- BEGIN:harness-exemption-contract -->

```text
harness_exemption_class: verification-only
harness_exemption_reason: Executes and records provenance-checked real-binary fixture evidence for the positive claim-start admission and start-record behavior delivered by prerequisites <UCS2a>, <UCS2b>, and <UCS4>; commits only one docs/closure evidence file, never a new red assertion.
harness_owner: none
exempt_verification_command: $f = 'docs/closure/<B>-claim-start-proof-positive.md'; if (-not (Test-Path -LiteralPath $f)) { Write-Error 'proof missing'; exit 1 }; $t = Get-Content -LiteralPath $f -Raw; foreach ($k in @('PROOF_SCOPE: 2A355F83','PROOF_BINARY_DIRTY: false','PROOF_WORKSPACE_OVERRIDE: UNSET','PROOF_LIVE_STATE: UNCHANGED','ROW claim-marks-members: PASS','ROW wave1-admits-claim-assigned: PASS','ROW start-recorded-once: PASS','PROOF_RESULT: PASS')) { if (-not $t.Contains($k)) { Write-Error "missing $k"; exit 1 } }; if ($t -notmatch '(?m)^PROOF_BINARY_COMMIT: ([0-9a-f]{40})\s*$') { Write-Error 'binary commit missing'; exit 1 }; $c = $Matches[1]; git cat-file -e "$c^{commit}"; if ($LASTEXITCODE -ne 0) { Write-Error 'proof commit unknown'; exit 1 }; git merge-base --is-ancestor 6d233d21162a072ddbdfecb52ec62a8fb8a63793 $c; if ($LASTEXITCODE -ne 0) { Write-Error 'producer 6d233d21 not in proof binary'; exit 1 }; git merge-base --is-ancestor $c HEAD; if ($LASTEXITCODE -ne 0) { Write-Error 'proof commit not on this branch'; exit 1 }; $s = (git show "${c}:.github/agents/_ship.agent.md") -join "`n"; if (-not ($s.Contains('WORK_STARTED:') -and $s.Contains('claim-assigned'))) { Write-Error 'proof commit lacks repaired contract'; exit 1 }; Write-Output 'EXEMPT_VERIFY_OK:<UCS5a>'
exempt_precondition: must-fail-before-deliverable
```

<!-- END:harness-exemption-contract -->

### UCS5b: Real-binary fail-closed claim-start proof (verification-only, harness-exempt)

* Delta: `docs/closure/<B>-claim-start-proof-fail-closed.md` (new) only. Freeze-scope.
* Each row uses its own fresh fixture `$run\ws-<row>` (protocol steps 5–6) under the unit's single
  build and single containment pair, so the rows are independent.
* Rows:
  * `started-member-is-residual`: run
    `comment add T1 --actor ship --comment "WORK_STARTED: <S>"`. Admission then classifies `T1`
    as an active residual and halts `WAVE_NO_PROGRESS` (`active residual`).
  * `marker-mismatch-is-residual`: no CLI flag sets `custom_fields` (`update --json` is an output
    flag), so the row edits the fixture item file directly, which is permitted only inside the
    ignored fixture. In `$ws\.backlog\queue\`, the `T1` item file must contain exactly one line
    matching `^\s*scheduler_baseline_claim:\s*<S>\s*$`; replace that one line's value with
    `OTHER-S`, then run `sync --cwd "$ws"`. `get T1 --format json --cwd "$ws"` must then report
    `custom_fields.scheduler_baseline_claim` = `OTHER-S` and status `active`. Admission then
    halts `active residual` for `T1`. Zero or several matching lines abort the row with
    `PROOF_PRECHECK_FAILED`. If the fixture binary rejects or quarantines the hand-edited file,
    the row is also recorded as `PROOF_PRECHECK_FAILED` and returned to Stage; it is never
    silently replaced by a different mechanism. (The same classification is already proven in
    the simulation scenario `claim_state_residuals`.)
  * `missing-log-is-indeterminate`: rename (never delete) the fixture
    `$ws\.backlog\logs\T1.jsonl` to `T1.jsonl.withheld`. Admission then halts
    `WAVE_CLAIM_STATE_INDETERMINATE`, with reason "log missing".
* Shipment-count ambiguity cannot be produced with real binaries: `ensureShipmentActiveSlotAvailable`
  refuses a second claim. That case is proven in the simulation (UCS4 `claim_state_indeterminate`)
  and is not a row here.
* Posture: verification-only. Commits evidence only.

<!-- BEGIN:harness-exemption-contract -->

```text
harness_exemption_class: verification-only
harness_exemption_reason: Executes and records provenance-checked real-binary fixture evidence for the fail-closed residual and indeterminate classifications delivered by prerequisites <UCS2a>, <UCS2b>, and <UCS4>; commits only one docs/closure evidence file, never a new red assertion.
harness_owner: none
exempt_verification_command: $f = 'docs/closure/<B>-claim-start-proof-fail-closed.md'; if (-not (Test-Path -LiteralPath $f)) { Write-Error 'proof missing'; exit 1 }; $t = Get-Content -LiteralPath $f -Raw; foreach ($k in @('PROOF_SCOPE: 2A355F83','PROOF_BINARY_DIRTY: false','PROOF_WORKSPACE_OVERRIDE: UNSET','PROOF_LIVE_STATE: UNCHANGED','ROW started-member-is-residual: PASS','ROW marker-mismatch-is-residual: PASS','ROW missing-log-is-indeterminate: PASS','PROOF_RESULT: PASS')) { if (-not $t.Contains($k)) { Write-Error "missing $k"; exit 1 } }; if ($t -notmatch '(?m)^PROOF_BINARY_COMMIT: ([0-9a-f]{40})\s*$') { Write-Error 'binary commit missing'; exit 1 }; $c = $Matches[1]; git cat-file -e "$c^{commit}"; if ($LASTEXITCODE -ne 0) { Write-Error 'proof commit unknown'; exit 1 }; git merge-base --is-ancestor 6d233d21162a072ddbdfecb52ec62a8fb8a63793 $c; if ($LASTEXITCODE -ne 0) { Write-Error 'producer 6d233d21 not in proof binary'; exit 1 }; git merge-base --is-ancestor $c HEAD; if ($LASTEXITCODE -ne 0) { Write-Error 'proof commit not on this branch'; exit 1 }; $s = (git show "${c}:.github/agents/_ship.agent.md") -join "`n"; if (-not ($s.Contains('WAVE_CLAIM_STATE_INDETERMINATE') -and $s.Contains('claim-assigned'))) { Write-Error 'proof commit lacks repaired contract'; exit 1 }; Write-Output 'EXEMPT_VERIFY_OK:<UCS5b>'
exempt_precondition: must-fail-before-deliverable
```

<!-- END:harness-exemption-contract -->

## Dependency Graph

```text
UCS1 (tests, red) ──► UCS2a (Ship)   ─┐
             └──────► UCS2b (policy) ─┼─► UCS5a (positive proof)
UCS3 (tests, red) ──► UCS4 (sim)     ─┴─► UCS5b (fail-closed proof)
```

| Wave | Members | Gate at wave close |
|---|---|---|
| 1 | UCS1, UCS3 | Both selectors RED on named assertions, with `Preserved` green; compile check green |
| 2 | UCS2a, UCS2b, UCS4 | The three `covered-by` exempt commands print `EXEMPT_VERIFY_OK`; `^TestUCS1_` and `^TestUCS3_` green; both red deliverables closed |
| 3 | UCS5a, UCS5b | Both exempt commands print their `EXEMPT_VERIFY_OK` markers; full quality gates green |

Edges recorded at harvest:

* UCS2a → UCS1, UCS2b → UCS1, UCS4 → UCS3;
* UCS5a → {UCS2a, UCS2b, UCS4}, UCS5b → {UCS2a, UCS2b, UCS4};
* one shipment edge, `B` blocks-on `154-S` (P).

No existing edge changes. There are no same-wave file overlaps: each unit touches distinct
files. Seven tasks at ≤2h each, about 14h total.

## Decisions and Rationale

* **D1 to D5** are inherited from the source decision. Start-epoch anchoring and the log parse
  policy are part of D1. Indeterminate precedence and the contract read recipe are part of D2.
* **P1 — Split the harness edit by file (UCS2a/UCS2b).** Each half turns exactly one UCS1
  subtest green. No manifest edit is needed (both files are `drift_allowed: true`).
* **P2 — Use a Go harness for the simulation (UCS3).** The red-deliverable branch accepts only
  an anchored `go test` selector with `--- FAIL:` lines. A PowerShell selector would halt
  `WAVE_RED_MAPPING_UNRESOLVED`. The full-run `Replay` subtest gives the simulation its first
  executable consumer.
* **P3 — Scenarios land with the model (UCS4), not with the harness.** If scaffolding put them in
  the fixture at wave 1, the Step 3 simulation replay would be red on any resume between waves.
* **P4 — Gate the proofs by evidence, provenance, and ancestry (UCS5a/UCS5b).** Each command
  checks the evidence rows, a 40-hex non-dirty commit that exists, contains `6d233d21`, is an
  ancestor of the branch HEAD, and contains the repaired contract. Transcript honesty is a review
  obligation (RB9).
* **P5 — No green-regression blocks.** The scoped selectors plus the wave-close quality gates are
  enough. The canonical default is empty.
* **P6 — Split the proof into two three-row units.** This keeps the granularity rule. Shipment
  ambiguity is proven in the simulation, because the producer cannot create it.
* **P7 — Green makers are `covered-by` exempt tasks.** UCS2a, UCS2b, and UCS4 write no test of
  their own; the owners' harnesses are what they turn green. Classifying them `covered-by` gives
  each a task-scoped loop command (`harness_owner_command`) and a marker-printing gate command,
  and stops harness-architect from scaffolding unplanned harnesses for them.
* **P8 — E1 is checked by the Orchestrator.** Ship's tool set has no version operation, and a
  CLI version reports the wrong binary. Editing Ship's tool list would widen this bootstrap's
  scope, so the Orchestrator runs the check and passes the evidence instead.

## Correct-Binary Handoff (E1)

Neither Stage nor Ship replaces, terminates, or restarts the live MCP server. Before Ship claims
`B`, the operator:

1. builds `backlogit` from a clean checkout of `origin/main` (or later) that contains `6d233d21`,
   injecting `-ldflags "-X github.com/softwaresalt/backlogit/internal/version.Commit=<full sha>"`;
2. records `go version -m <binary>` (showing `vcs.revision=X` and `vcs.modified=false`), the
   absolute binary path, and the binary's SHA-256;
3. starts it as this workspace's MCP server through the operator's normal procedure.

The server-side check is run by the **Orchestrator**, because `backlogit_get_version` is in the
Orchestrator's tool set (`backlogit/*`) and not in Ship's. Immediately before dispatching Ship,
the Orchestrator calls `backlogit_get_version` with `{no_update_check: true}`, confirms a 40-hex
commit `X` (no `-dirty`, not `unknown`), runs
`git merge-base --is-ancestor 6d233d21162a072ddbdfecb52ec62a8fb8a63793 X` (exit 0), and confirms
that `X` equals the operator-recorded `vcs.revision`. It then passes `X`, the operator's
`go version -m` evidence, and the absolute served storage root in the dispatch text.

* Ship halts `BOOTSTRAP_E1_BINARY_UNMET` if the dispatch text lacks that evidence.
* A CLI `version` result never satisfies the server-side check, because it reports the CLI
  binary, not the served one.
* Any CLI binary that Ship uses as a fallback must itself show `vcs.revision=Y` and
  `vcs.modified=false` under `go version -m`, with
  `git merge-base --is-ancestor 6d233d21162a072ddbdfecb52ec62a8fb8a63793 Y` exiting 0; otherwise
  Ship halts `BOOTSTRAP_E1_BINARY_UNMET` instead of using it.
* Report line for every `BOOTSTRAP_*` halt:
  `BOOTSTRAP HALT {token} B={B}: {observed} — returned to Orchestrator`, recorded through P-005.

## Exception and Dispatch Preconditions

The Exception Matrix in the source decision is authoritative. The binding is structural:

* **Authoritative binding.** The `## Harvest Record` appended to this plan at harvest contains
  exactly one fenced block delimited by `<!-- BEGIN:harvest-record -->` and
  `<!-- END:harvest-record -->`. It names `B`'s concrete ID and its approved member IDs as an
  ordered list (chore root first, then tasks in harvest order). It is committed and reaches `main`
  through E2. Every check reads it through the Verified Main Read below, never from a working
  copy and never through an unverified local ref. The waiver applies to that ID only.
* **Informational record.** Stage appends a comment on `B` with actor `stage`. Its first line is
  `BOOTSTRAP_EXCEPTION_RECORDED: 2A355F83 B=<B>`. It quotes the verbatim authorization and
  references the Harvest Record. The revocation token never appears on its first line. A
  `BOOTSTRAP_EXCEPTION_RECORDED` comment on any other shipment is an anomaly and grants nothing.
* **Revocation.** The exception is revoked by either of:
  * any comment on `B` whose first line is exactly `BOOTSTRAP_EXCEPTION_REVOKED: 2A355F83`, from
    any actor;
  * an explicit operator statement relayed verbatim.

  Actor fields confer no authority to grant. They are never needed to revoke.
* **In-force check.** The exception is in force only when all of these hold (the same wording is
  copied verbatim into dispatch item (2)):
  * the Harvest Record, read through the Verified Main Read, names `B`;
  * `B`'s live `custom_fields.items`, as an ordered list, equals the recorded member list exactly;
  * `B`'s own log `<served storage root>\logs\<B>.jsonl` is present and parseable (log parse
    policy above), and carries no revocation;
  * no stop or expiry condition has occurred.

  The check runs before `B`'s claim, at every wave admission of `B`, before pull request
  creation, and before merge. A missing or unparseable `B` log means "not in force".
* **Scoped raw-log reads.** The `B` log read above and the E3 recognition read below are read set
  (b) of the scoped P-012 exception (Start semantics). The member item logs read at Step 4.0 and
  Step 4.1b are read set (a). No other backlog read bypasses the configured tools.
* **Return-blocked interplay.** `backlogit_return_blocked` removes a member from `B`'s live items,
  so the in-force check fails and the exception expires. That is a terminal outcome for this
  exception: resuming needs new operator authority, and is never silent.
* **E2** (staging merge) and **E4** (L1 interplay) are as decided.

### Verified Main Read (VMR)

`origin/main` is a local ref that can be stale or forged, so it is never trusted on its own. Every
read of an authority artifact from `main` (the Harvest Record in this plan, and the E3 grant file)
runs this procedure fresh, at each checkpoint where the check runs. Any failed step fails the
check that needed the read (`BOOTSTRAP_EXCEPTION_NOT_IN_FORCE` for the Harvest Record,
`BOOTSTRAP_E3_NOT_GRANTED` for the grant), with observed detail `VMR step <n>`. There is no
fallback to a cached result, a working copy, or an earlier checkpoint's read.

1. **Fresh fetch.** `git fetch --no-tags origin +refs/heads/main:refs/remotes/origin/main` exits 0.
2. **Canonical remote.** `git remote get-url origin` prints exactly
   `https://github.com/softwaresalt/backlogit.git`. Any other value, including a fork, a mirror,
   or an SSH form, fails.
3. **Ref equality.** `$m = git rev-parse --verify 'origin/main^{commit}'`, and `$r` is the first
   field of the single output line of `git ls-remote --exit-code origin refs/heads/main`. Both are
   40 lowercase hex characters, and `$m` equals `$r`.
4. **Pinned read.** The artifact is read only as `git show "${m}:<path>"`, pinned to the verified
   SHA `$m`, never through the ref name.
5. **Merged-PR provenance.** `git log --format=%H $m -- <path>` lists at least one commit. For
   every listed commit `$h`, `gh api repos/softwaresalt/backlogit/commits/$h/pulls` succeeds and
   returns at least one pull request with a non-null `merged_at` and `base.ref` equal to `main`.
   A `gh` error (including missing authentication), an empty array, or no merged pull request
   into `main` fails. The repository allows merge commits and disables squash and rebase merges,
   so a pull request's commits keep their own SHAs on `main` and stay resolvable this way.

The session record carries `VMR <path>: main=<$m> url=<url> commits=<$h:#PR,...>` for each read.

### E3 Replacement List and Recognized Halts

**E3** (execution under the pre-repair installed contract) is new authority: NOT granted, and not
assumed. Its exact wording is the `e3-wording` block of the source decision, reproduced here byte
for byte:

<!-- BEGIN:e3-wording -->

```text
For the bootstrap shipment B named in the plan 2A355F83 Harvest Record only, until B's repaired Ship and policy text is on main and installed, Ship executes B by applying the repaired text specified in plan 2A355F83 (D1, D2, start epoch, H1) in place of exactly these installed passages: Ship Step 2 item 1; Ship Step 4.0 items 4, 6, and 7; the Ship Step 4.1a never-stranded-active sentence; Ship Step 4.1b; Ship Step 4.6 item 1; policy P-002.6 Definitions ready_k; policy P-002.6 Active leftovers halt too; policy P-002.6 per-wave steps 1 and 2; and the policy P-002.2 WAVE_NO_PROGRESS row (both conditions). For B's session only, Ship recognizes WAVE_CLAIM_STATE_INDETERMINATE (wave admission only) and TASK_START_NOT_RECORDED (Step 4.1b) as halts, each reported with the plan's report line and recorded through P-005. Every other installed step, halt, circuit breaker, review, CI, runtime, and merge control remains.
```

<!-- END:e3-wording -->

The replacement list in that wording is the complete set of installed passages that would
otherwise halt `B` on its own claim-assigned members or contradict the repaired `ready_k`. It
matches the UCS2a and UCS2b edit scope, except the Step 4.0 replay paragraph, which is not
replaced: the replay validates the simulation, not `B`'s live state. The installed
circuit-breaker row "Active residual at wave admission" keeps applying to the D2 residual class.

* A grant is recognized only when ALL of these hold:
  * `docs/memory/<yyyy-MM-dd>/orchestrator-2a355f83-e3-grant.md` is read through the Verified
    Main Read (all five steps, including merged-pull-request provenance of every commit that
    touched the file);
  * that file quotes the operator's grant verbatim, the grant contains the `e3-wording` text
    above exactly, and the file contains a line `B=<B>` naming `B`'s concrete ID;
  * `B`'s own log (scoped raw-log read set (b)) carries a comment whose first line is exactly
    `BOOTSTRAP_E3_GRANTED: 2A355F83 B=<B>`, which quotes the same grant byte-for-byte and names
    that path.

  A comment alone, an unmerged memory file, or a file reachable only through an unverified ref
  grants nothing.
* `B` gets no `do-not-claim` label. The Orchestrator applies the precondition check at dispatch.

Exact Orchestrator dispatch instruction (to be passed to Ship verbatim, with `<B>` substituted):

<!-- BEGIN:orchestrator-dispatch-contract -->

```text
DISPATCH B=<B> UNDER BOOTSTRAP 2A355F83.
Served storage root: <absolute path, recorded by the Orchestrator>. Served workspace root: <absolute path, recorded by the Orchestrator>. E1 evidence: server commit X=<40-hex> from backlogit_get_version; operator go version -m record vcs.revision=X vcs.modified=false, binary <path>, SHA-256 <hash>.
Scoped P-012 raw-log reads (declare each use as "P-012 SCOPED RAW-LOG READ (2A355F83): <path>"): (a) <served storage root>\logs\<id>.jsonl for members of <B>, at Step 4.0 classification and Step 4.1b start-record verification; (b) <served storage root>\logs\<B>.jsonl, at each checkpoint below. Every other backlog read uses the configured tools.
Verified Main Read (VMR) for every read from main: git fetch --no-tags origin +refs/heads/main:refs/remotes/origin/main exits 0; git remote get-url origin is exactly https://github.com/softwaresalt/backlogit.git; $m = git rev-parse --verify origin/main^{commit} equals the hash from git ls-remote --exit-code origin refs/heads/main (both 40-hex); read only with git show $m:<path>; every commit in git log --format=%H $m -- <path> has, via gh api repos/softwaresalt/backlogit/commits/<sha>/pulls, at least one pull request with non-null merged_at and base.ref main. Any failure fails the check that needed the read.
Before claim, before every wave admission of <B>, before pull request creation, and before merge, verify, and halt on any failure (report "BOOTSTRAP HALT {token} B=<B>: {observed} — returned to Orchestrator" and record it through P-005):
(1) BOOTSTRAP_E1_BINARY_UNMET unless the E1 evidence above is present and complete (the Orchestrator ran backlogit_get_version; a CLI version result never satisfies this item), and any CLI fallback binary shows vcs.modified=false with 6d233d21162a072ddbdfecb52ec62a8fb8a63793 an ancestor of its vcs.revision;
(2) BOOTSTRAP_EXCEPTION_NOT_IN_FORCE unless all of these hold: the Harvest Record, read through VMR from docs/exec-plans/2026-10-01-2a355f83-claimed-vs-started-bootstrap-plan.md with exactly one BEGIN:harvest-record/END:harvest-record block, names <B>; <B>'s live custom_fields.items, as an ordered list, equals the recorded member list exactly; <B>'s own log <served storage root>\logs\<B>.jsonl is present and parseable, and carries no revocation (no comment on <B> whose first line is BOOTSTRAP_EXCEPTION_REVOKED: 2A355F83); and no stop or expiry condition has occurred;
(3) BOOTSTRAP_E3_NOT_GRANTED unless docs/memory/<yyyy-MM-dd>/orchestrator-2a355f83-e3-grant.md is read through VMR, quotes the operator's grant containing the decision's exact e3-wording block text, and contains the line B=<B>, and <B>'s own log carries a comment whose first line is exactly BOOTSTRAP_E3_GRANTED: 2A355F83 B=<B> that quotes that grant byte-for-byte and names that path; under E3, apply the plan's repaired text (D1, D2, start epoch, H1) in place of exactly the e3-wording replacement list (Ship Step 2 item 1; Ship Step 4.0 items 4, 6, and 7; the Ship Step 4.1a never-stranded-active sentence; Ship Step 4.1b; Ship Step 4.6 item 1; policy P-002.6 Definitions ready_k; policy P-002.6 Active leftovers halt too; policy P-002.6 per-wave steps 1 and 2; the policy P-002.2 WAVE_NO_PROGRESS row, both conditions), recognize WAVE_CLAIM_STATE_INDETERMINATE (wave admission only) and TASK_START_NOT_RECORDED (Step 4.1b) as halts of this session with the plan's report lines and P-005 recording, and keep every other installed step, halt, circuit breaker, review, CI, runtime, and merge control;
(4) P: the blocks edge from <B> to 154-S exists and 154-S is shipped.
C is waived for <B> only. This dispatch grants no attestation, no merge or admin fallback, no dark mode, and no authority over any other shipment.
```

<!-- END:orchestrator-dispatch-contract -->

## Risks

* **RB1 to RB5**: as in the source decision (workspace-local log, truncation, mixed binary, first
  chore root, R2 any-activation).
* **RB6 — Resume across waves.** A crash between wave 1 and wave 2 leaves UCS1 and UCS3 red. That
  is the declared open red, carried by `open_red_deliverables`. Per P3, the simulation replay is
  unaffected.
* **RB7 — Conflict with 186-S (L1).** Both edit the policy version history. Mitigation: append
  only, and use the next available minor version.
* **RB8 — `pwsh` dependency in the integration test.** This is an accepted consequence. A
  developer without `pwsh` gets an explicit failure, not a skip. A Linux path defect in the
  simulation is a UCS4 defect, fixed inside UCS4's two files.
* **RB9 — Self-recorded proof.** UCS5a and UCS5b gate on presence, provenance, and ancestry, not
  on truth. Mitigation: per-row command and output excerpts, normal PR review, and the
  post-merge re-run.
* **RB10 — First-time red deliverables in `tests/integration`.** The selector form matches the
  shipped `174.074-T` precedent (`./tests/integration`).
* **RB11 — Unversioned log schema and deliberate log edits.** The start record depends on the
  item-log event shape, and anyone with filesystem access can edit a log.
  * Mitigation: the parse policy is fail closed, and any structural surprise is indeterminate.
    The UCS1 `Preserved` log-fact pin fails if the producer's `shipment claimed` reason literal
    or the comment event shape changes.
  * A deliberately inserted `WORK_STARTED:` only makes a member a residual (fail closed). A
    deliberately removed one is the RB2 class.
  * CI runs `tests/integration` only on code-changing pull requests (the `ci.yml` paths filter),
    so a markdown-only pull request that edits Ship or the policy does not run the UCS1 guard.
    This is a named follow-up, recorded here and not staged, together with a history-read tool;
    this bootstrap needs neither.
* **RB12 — Chore-root child-ID collision.** Per
  `docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md`, Stage checks
  both `.backlogit/queue/` and `.backlogit/archive/` for the candidate chore number and its
  children before creation, without suppressing errors. After harvest it verifies the exact
  child count (7). Any collision halts harvest. The root is a chore, not a feature, because the
  work is internal workflow maintenance with no user-facing capability, and live metadata allows
  chore roots with task children.
* **RB13 — Hand-edited fixture frontmatter.** The `marker-mismatch-is-residual` row edits a
  fixture file directly, because no CLI surface sets `custom_fields`. The edit is confined to the
  ignored fixture, is exact-match guarded, and is followed by `sync`. If the binary rejects it,
  the row fails closed (`PROOF_PRECHECK_FAILED`) rather than switching mechanism.

## Constitution Check

* **I Safety-First Go:** test code only; no production Go change. Pass.
* **II Test-First:** UCS1 and UCS3 land red (with green `Preserved` guards) before UCS2a, UCS2b,
  and UCS4. UCS5a and UCS5b are verification-only with must-fail commands. No stub. Pass.
* **III and IV Workspace Isolation and CLI Containment:** the proof protocol uses a private clone
  for the build, per-row absolute `--cwd` fixture roots (`.backlog`) inside ignored `logs/`, an
  unset workspace override, a single-session window, and a whole-storage-root hash baseline, and
  calls no live tools. The live MCP server is untouched. Pass.
* **V Observability:** new halts use exact P-002.2 tokens with report lines. The `BOOTSTRAP_*`
  halts, `PROOF_CONTAINMENT_BREACH`, and `PROOF_PRECHECK_FAILED` are session-scoped tokens of this
  bootstrap (not P-002.2 rows); each has a defined report line and is recorded through P-005.
  Pass.
* **VI Single Responsibility:** one file per concern. Pass.
* **VII Destructive Command Approval:** no deletion; the one rename stays inside the fixture.
  Fixture removal needs operator approval. Pass.
* **VIII Safety Modes:** Careful mode for UCS2a, UCS2b, and UCS4; Freeze-scope for UCS5a and
  UCS5b. No DARK_MODE. Pass.
* **IX Persistence:** a documented deviation. The durable start record lives in the
  workspace-local, git-ignored item log, not in git-tracked frontmatter. Justification: D1 (S2
  and S3 break marker v1 or the bootstrap scope). Containment: a loss fails closed (RB1, RB11).
* **X Context:** text edits plus two evidence documents. Pass.
* **XI Merge Commit History:** unaffected. Pass.
* **Execution precondition:** a documented deviation. `B`'s dispatch depends on the ungranted
  E3. This is disclosed, not assumed.
* **P-012 scoped raw-log reads:** a documented deviation. Exactly two read-only sets (member item
  logs at Step 4.0 and Step 4.1b; `B`'s own log at the four dispatch checkpoints), each declared
  per use. No configured tool returns the event stream; the tool is used if one later does.
* **Task Granularity:** seven tasks, each with fewer than 3 files, at most 4 functions, and at
  most 3 scenarios. Every task has exactly one harness classification (two red deliverables,
  three `covered-by`, two `verification-only`). Pass.
* **Quality gates** (pre-merge): `go vet ./...`, `golangci-lint run`, `gofmt -l` (empty), and
  `go test ./... -count=1`, plus the gates listed under Runtime Verification and Closure.

Constitution Check: documented-deviations

## Plan Hardening Signals

* Changes the wave-admission contract that gates every shipment's execution.
* Introduces governed exception semantics (a single-shipment C waiver), plus new fail-closed halt
  tokens.
* Depends on binary provenance (E1) and on a workspace-local log as durable state.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Pre-merge:**
  * `go test -count=1 -run '^TestUCS' -v ./tests/integration`;
  * the full quality gates above;
  * `pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -Quiet`, and with `-VerifyAgainstQueue`;
  * docs lint and `scripts/md-lint.ps1` (P-008) with zero violations on every changed Markdown
    file;
  * all five exempt commands, each printing its `EXEMPT_VERIFY_OK` marker;
  * the dispatch in-force check (item (2)) re-run immediately before pull request creation and
    before merge.
* **Post-merge closure** is Ship's standard closure, outside the seven tasks. It runs in the
  single active worktree, under P-020 with a scoped compact-context. Ship records
  `docs/closure/<B>-post-merge-closure.md`:
  * re-run the rows `wave1-admits-claim-assigned`, `start-recorded-once`, and
    `started-member-is-residual` under the full Proof Protocol, with a binary built from the
    merged `main` commit and with the installed `main` contract;
  * record the live MCP version and the CLI version at closure time;
  * state "merged but not yet operative" until the operator has installed a binary from that
    commit (E1);
  * include a self-check equivalent to the exempt commands, run against the closure file.
* **The closure states plainly:**
  * the evidence concerns marker consumption only;
  * broader ledger gaps remain open;
  * the operator alone decides whether to write `CONDITION_B_ATTESTED:` on `154-S`.
* Neither Ship nor Stage writes or relays an attestation without an explicit operator request.

## Plan Hardening

Hardening required: yes. The plan changes the wave-admission contract that gates every
shipment's execution, adds two fail-closed halt tokens, carries a governed single-shipment
exception, and depends on binary provenance plus a workspace-local log as durable state.

### Inputs

* Learnings (Step 1.8, `docs/compound/`, medium confidence), applied as follows:
  * Claim activation is not a start: D1, the start epoch, and H1.
  * A marker must be handled on every transition path: RB5, the indeterminate conditions, and
    the start epoch on re-claim.
  * Close a self-dependent gate with a rebuilt binary: E1, and the UCS5 provenance build.
  * Self-hosted binary skew can make a merged fix inert: the E1 provenance check, and the
    post-merge "not yet operative" record.
  * Source-shape harnesses pin stable tokens only: UCS1 slices by heading and asserts literals.
  * Exceptions must be structural: the Harvest Record binding and the dispatch contract block.
  * The chore-root child-ID collision trap: RB12.
* Instructions re-read:
  * `.github/policies/workflow-policies.md`: P-002 and P-002.1 (harness-ready and exemption
    classes), P-002.3 (exempt command and marker), P-002.4 (class delta surface and ignored
    scratch path), and P-002.6 (red-deliverable block, green-regression block, convergence,
    non-shipment `frozen_task_ids`).
  * `.github/skills/build-feature/SKILL.md` Step 0.5: the red selector is an anchored `go test`
    command with `--- FAIL:` lines.
  * `.github/instructions/constitution.instructions.md`: Principles II, IV, VII, VIII, IX, and
    Task Granularity.

### Protected Invariants

* **H1 — Dispatch strictly after a verified start record.** No `build-feature` dispatch, and no
  Step 4.1c or later action for a task, precede the re-read that confirms exactly one valid
  `WORK_STARTED: S` record in the current epoch.
  * A crash before the record leaves an assigned member that is safe to re-admit.
  * A crash after it leaves a residual that halts.
  * Item 10 scaffolding, which runs before Step 4.1b, is pre-existing behavior and unchanged.
* **H2 — Every existing admission halt is unchanged.** Blocked, unsupported, empty-frontier,
  cycle, budget, and snapshot halts all stay. Completion is still only `terminal_success = M`, and
  `M` stays frozen. The UCS1 and UCS3 `Preserved` subtests pin this.
* **H3 — Never a fail-open read.** All of these halt `WAVE_CLAIM_STATE_INDETERMINATE`, with
  precedence over any residual, and none is ever read as "not started" or as "assigned":
  * a missing, unparseable, or claim-event-less log;
  * an errored or disagreeing item read;
  * manifest drift;
  * shipment ambiguity;
  * an R3 marker condition.
* **H4 — Producer unchanged.** No Go production delta, no change to bulk claim activation (M2),
  and no new API, status, event type, or schema field.
* **H5 — Exception boundary.** The C waiver applies to the Harvest-Record-named `B` only. P is
  still required. Every successor keeps P and C. Nothing writes, relays, or implies
  `CONDITION_B_ATTESTED:`. Neither E3 nor E1 is assumed.
* **H6 — Live-state containment.** The live `.backlogit/` and the live MCP server are never
  fixture targets, and are never replaced or restarted by Stage or Ship. The Proof Protocol's
  baseline, no-live-tools window, and containment check enforce this.

### Risky Actions

| ProposedAction | ActionRisk | Approval | Rollback |
|---|---|---|---|
| PA1: edit Ship Step 2 item 1, Step 4.0 items 4, 6, and 7, the replay paragraph, the Step 4.1a sentence, Step 4.1b, and Step 4.6 item 1 (UCS2a) | high (contract, all shipments) | Careful mode; normal PR review and CI | revert the commit through a normal PR |
| PA2: edit P-002.6, the P-002.2 rows, and version history (UCS2b) | high (policy contract) | Careful mode; normal PR review and CI | revert the commit through a normal PR |
| PA3: edit the simulation model and fixture (UCS4) | medium (scheduler replay gate) | Careful mode; 21-scenario guard; `-VerifyAgainstQueue` | revert the commit |
| PA4: provenance build and fixture CLI runs under ignored `logs/` (UCS5a, UCS5b) | medium (containment) | the Proof Protocol checks; abort on any mismatch | none needed, because the live state is untouched; fixture removal only with operator approval |
| PA5: install a provenance-checked binary as the live MCP server (E1) | high (operator infrastructure) | operator only; never Stage or Ship | the operator restores the previous binary |
| PA6: claim `B` | high (bulk activation) | the dispatch contract block (E1, in-force check, E3, P) | the existing claim-recovery path; no admin fallback |
| PA7: append the informational exception comment on `B` (Stage, at harvest) | low (append-only) | already authorized | a revocation comment |

`ActionResult` for each executed action is recorded by its executor: Stage for PA7, Ship for
PA1–PA4 and PA6, and the operator for PA5.

### Added Verification

* The Proof Protocol prechecks: per-run ignored root, ignored-inclusive baseline and hash
  manifest, provenance build, fixture-init root confirmation, and the containment check.
* Blocked-path handling: any failed precheck aborts with P-005 and never falls back to the live
  workspace or the live MCP server.
* The `Preserved` subtests run in every wave from wave 1 onward.
* At harvest, Stage verifies the chore child count, the dependency edges, `B`'s manifest, the
  Harvest Record block, and zero `scripts/md-lint.ps1` and docs-lint violations on the Stage
  artifacts.

### Operational Closure

* **Monitoring:** the existing mechanism. Ship records every `WAVE_CLAIM_STATE_INDETERMINATE` and
  `TASK_START_NOT_RECORDED` halt through P-005 telemetry, in its session memory, and in the
  closure document.
* **Rollback trigger:** a legitimate claim-assigned member (correct marker, live membership,
  claim event, no record in the epoch) is halted.
* **Rollback procedure:** revert UCS2a and UCS2b through a normal PR. The prior contract fails
  closed, so a rollback is safe.
* **Owner:** Ship for execution and closure; the operator for E1, E3, and C.
* **Validation window:** from `B`'s merge until `B`'s post-merge closure is recorded.

### Review-Gate Capability

* Plan review must emit literal `dispatch_mode:` and `decision:` markers in a `## Plan Review`
  section.
* Multi-agent dispatch is available through the custom reviewer agents.
* Engram, attempts 1 to 3: degraded (the CLI bind failed twice; no third bind was attempted), so
  reviewers used targeted known-path reads. Attempt 4: after the approved daemon restart and a
  successful operator-directed `workspace-status` (bound path, scan complete, not stale), Engram
  CLI `query-memory` and `search` calls succeed. Reviewers may use Engram CLI for bounded context
  and still use known-path reads. Neither state degrades the dispatch mode.
* agent-intercom is not installed, so there are no remote broadcasts.

### Unresolved Operator Decisions

* **E3:** execution under the pre-repair contract, applying the plan's repaired text in place of
  exactly the `e3-wording` replacement list. This is new authority, not granted. It blocks Ship
  dispatch of `B`, not harvest.
* **E1:** the binary installation (operator infrastructure).
* **C for successors:** the operator alone decides, after `B`'s runtime closure.
* **Fixture removal:** optional, and needs operator approval.

## Attempt-2 Revision Note

This body supersedes the attempt-1 text that the review below assessed (SHA-256 prefix
`C5D823393DC3`). Every attempt-1 P1 finding is addressed as follows:

1. Token gaps and the policy statements: UCS1 token set `K`, the negative `ready_k`
   assertions, the `WAVE_NO_PROGRESS` row assertion, and the UCS2b scope.
2. Containment: Proof Protocol steps 2, 3, 5, and 8.
3. Proof granularity: the UCS5a/UCS5b split (P6).
4. Selector naming: the `UCS*` unit names.
5. The impossible row: removed; the case is proven in the simulation.
6. Binary provenance: Proof Protocol step 4, and E1.
7. Chore-root collision: RB12.
8. E3 soundness: the amended E3 in the source decision, and the dispatch contract item (3).
9. Exception binding: the Harvest Record binding, the revocation rules, the in-force check, and
   the dispatch contract block.

The P2 and P3 findings are folded into the units and sections they name.

## Attempt-3 Revision Note

This body supersedes the attempt-2 text that the second review below assessed (SHA-256 prefix
`18834BDED6F8`). Every attempt-2 P1 finding is addressed as follows:

1. Fixture storage root: Proof Protocol steps 3 and 5 (`.backlog` root, `.backlogit` absent,
   override unset and recorded, no parent-directory fallthrough).
2. Infeasible marker-mismatch row: UCS5b row rewritten as an exact-match fixture frontmatter edit
   plus `sync`, read back through the recipe's CLI `get`; RB13.
3. Green-maker classification: the harness classification table, P7, and full `covered-by`
   contract blocks on UCS2a, UCS2b, and UCS4, with marker-printing gate commands and explicit
   `--- PASS:` subtest checks.
4. E1 owner: the Orchestrator runs `backlogit_get_version` and passes the evidence (Correct-Binary
   Handoff, P8, dispatch item (1)); a CLI version never satisfies the server check.
5. Removed-sentence assertions: UCS1 `Ship` and `Policy` now `assert.NotContains` the exact
   pre-repair sentences, and Ship Step 2 item 1 joins the scope.
6. E3 self-attestation: the grant file path is fixed and must be reachable from `origin/main`;
   the `B` comment must match byte-for-byte (source decision E3, dispatch item (3)).

Folded P2 and P3 findings: private-clone provenance build (clean status including untracked
files, `vcs.modified=false` as the dirty check, evidence written last); one build and one
containment pair per unit with a fresh fixture per row; whole-storage-root hash baseline with a
single-session window; UCS3 robustness (`NO_COLOR`, ANSI stripping, `errors.As`, 24 outcome
lines, `require`d lookups); line-anchored, exactly-once slice anchors; the UCS4 default; the
log-fact pin; the UCS2a size justification; return-blocked expiry; the delimited Harvest Record
read from `origin/main` with an ordered member comparison; report lines for the `BOOTSTRAP_*` and
`PROOF_*` halts; revocation re-checks before pull request creation and merge; the scoped P-012
raw-log exception and served-root handoff; md-lint gates; RB11 CI paths-filter follow-up; RB12
queue and archive check and chore rationale; epoch edge cases; and the post-merge closure stated
as Ship's standard closure.

## Attempt-4 Revision Note

This body supersedes the attempt-3 text that the third review below assessed (SHA-256 prefix
`91305A6BBBF0`). It is the single targeted revision the operator authorized after the attempt-3
halt; it changes no start semantics, no DAG edge, no unit count, and no exception-matrix row.
Every attempt-3 P1 finding is addressed as follows:

1. `WAVE_NO_PROGRESS` first condition: UCS2b rewrites it to "no `queued` or claim-assigned member
   has all dependencies terminal"; UCS1 `Policy` asserts the new phrase in that row and
   `assert.NotContains` the old "no `queued` member has all dependencies terminal" (Scope, UCS1,
   UCS2b).
2. `B`'s own shipment log: the scoped P-012 exception now has two read sets, (a) member item logs
   at Step 4.0 and Step 4.1b and (b) `<served storage root>\logs\<B>.jsonl` at claim, every wave
   admission, before pull request creation, and before merge; the dispatch block states both
   (Start semantics, Exception section, dispatch block, Constitution Check).
3. E3 replacement list: one canonical `e3-wording` block, identical in the source decision and
   this plan, lists Ship Step 2 item 1, Step 4.0 items 4, 6, and 7, the Step 4.1a sentence, Step
   4.1b, Step 4.6 item 1, P-002.6 Definitions `ready_k`, "Active leftovers halt too", per-wave
   steps 1 and 2, and the `WAVE_NO_PROGRESS` row; it recognizes `WAVE_CLAIM_STATE_INDETERMINATE`
   and `TASK_START_NOT_RECORDED` as session halts. Same-contract completion: Step 4.6 item 1's
   `active` check becomes mandatory "no member of `ready_k` is `active`" in UCS2a and is asserted
   in UCS1 `Ship`, and the two pre-repair "Active leftovers" sentences are asserted absent in UCS1
   `Policy`, so the permanent text matches the replacement list.
4. `origin/main` trust: the Verified Main Read (fresh fetch, canonical URL, `rev-parse` equal to
   `ls-remote`, SHA-pinned `git show`, merged-pull-request provenance of every commit touching
   the read path via `gh api .../commits/<sha>/pulls`) governs both the Harvest Record read and
   the E3 grant read; the grant file must also carry `B=<B>`.

The dispatch block also passes the served workspace root, which the CLI fallback's `--cwd`
already required.

Not changed (carried P2/P3 items left for review judgment, not scope): exempt-command deliverable
probes, `len(scenarios)` instead of 24, evidence-marker regex hardening, fixture frontmatter
backup, containment volatility and `GIT_CEILING_DIRECTORIES`, UCS5a-before-UCS5b ordering, the
served-root Ship rule, R3 tool mapping, lenient revocation matching, and the remaining P3 items.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

* Attempt: 1, reviewing plan SHA-256 prefix `C5D823393DC3`.
* `TOOL_OK: reviewer-subagent-dispatch`.
* Personas dispatched: 7, and all 7 returned.
  * Always-on: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher.
  * Cross-model triggers: Architecture Strategist (always); Agent-Native Parity Reviewer (the
    plan changes the agent-facing Ship workflow); Security Lens Reviewer (authority records and
    the containment trust boundary).
* Plan hardening was required and is present. The Constitution Check verdict is present.
* Raw counts, before deduplication:

  | Persona | P0 | P1 | P2 | P3 |
  |---|---|---|---|---|
  | Constitution | 0 | 3 | 6 | 5 |
  | Go | 0 | 2 | 5 | 5 |
  | Scope | 0 | 1 | 6 | 4 |
  | Learnings | 0 | 3 | 6 | 4 |
  | Architecture | 0 | 1 | 7 | 3 |
  | Parity | 0 | 3 | 6 | 2 |
  | Security | 0 | 3 | 3 | 2 |

Rationale: there are no P0 findings. Nine deduplicated P1 findings block harvest, so the decision
is FAIL.

### P1 findings (deduplicated)

1. **Contract gaps the harness cannot see.** The harness does not pin R4 (the frontier), the H1
   ordering rule, live-manifest membership, manifest drift, or stale reads. The policy keeps
   three statements that contradict the repaired Ship contract:
   * the P-002.6 `ready_k` definition (~L645);
   * the per-wave step 2 text;
   * the `WAVE_NO_PROGRESS` P-002.2 row condition (~L291).

   Raised by Constitution, Scope, Parity, and Architecture.
2. **U5 containment is blind to ignored files.** `git status --porcelain` skips the git-ignored
   `.backlogit/logs/`. The `--cwd` path is relative. No rule maps MCP calls to the fixture CLI.
   Raised by Constitution, Learnings, Security (two findings), and Go.
3. **U5 has 7 scenario rows.** That exceeds the task granularity limit, and the Constitution
   Check misstated it. Raised by Constitution and Scope.
4. **Selector names do not follow `^TestU<unit>_`.** The build-feature Step 0.5 precondition 2
   requires that form, but the units are named U1/U3 while the tests use `TestUCS1_`/`TestUCS3_`.
   Raised by Go.
5. **The `second-active-shipment` row cannot happen.** `ensureShipmentActiveSlotAvailable`
   refuses a second claim. Raised by Go and Parity.
6. **Binary provenance is unproven.** A plain `go build` reports commit `unknown`. Recorded
   commits are self-declared. Dirty builds are not rejected. Raised by Parity, Learnings, Security,
   and Go.
7. **Chore-root child-ID collision trap.** The plan does not cover it
   (`docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md`). Raised by
   Learnings.
8. **Unsound E3 wording.** Under the pre-repair Step 4.1b, B writes no start records, so a crashed
   started member would be re-admitted. That fails open for B itself. Raised by Architecture,
   Learnings, Security, and Scope.
9. **The exception is not bound to anything authoritative.** A forgeable comment could be read as
   the waiver. The plan has no committed binding to B's ID, no exact revocation rule, no scope
   baseline, and no defined dispatch-check text. Raised by Security, Parity, and Learnings.

### P2 and P3 findings carried into the revision

* CLI `--comment` is a required flag.
* The Problem Frame cites the wrong producer path; it must use the decision's correction.
* Guards must run independently of red tokens: use a `Preserved` subtest, with `assert` for red
  tokens.
* After an MCP-to-CLI fallback, re-read before appending.
* Thread the new halt token through Step 4.0 item 7 and the replay paragraph. Report and P-005
  recording are needed for `TASK_START_NOT_RECORDED`.
* Make the harness mapping for the green makers explicit.
* Run the full simulation in Replay.
* Careful mode for the simulation unit and Freeze-scope for the proof.
* Documented deviations: Principle IX (local log), and E3 (conditional).
* Name the simulation functions and use a single overrides map.
* Resolve the log path from the storage root.
* Anchor the start record to the latest claim event.
* Read markers through the marker contract recipe and R3. The marker read is not
  "frontmatter-sourced".
* Clarify Step 4.6 item 1 to `ready_k`.
* Define non-shipment mode.
* Indeterminate takes precedence.
* Step 4.1a gets a one-sentence qualification.
* Drop the harness-manifest edits: unrequested, they caused same-wave overlap and pinned a
  bookkeeping token.
* Make the `pwsh` prerequisite an explicit, accepted consequence.
* Use an existing monitoring mechanism.
* Rename the E1 halt token.
* The closure records the operative binaries.
* `plugin/agents/ship.agent.md` is not a mirror.
* Define the log parse policy.
* Record deliberate log edits as an accepted residual.
* Add the full quality-gate list.
* Use per-run fixture directories.
* P-005 recording on a containment abort.
* Filtered log reads.

Recommendation: revise the plan and the source decision (D1 anchor, E3 wording, exception
binding) and re-run every persona.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

* Attempt: 2, reviewing plan SHA-256 prefix `18834BDED6F8` (the attempt-2 revision).
* `TOOL_OK: reviewer-subagent-dispatch`.
* Personas dispatched: 7, and all 7 returned. The persona set is the same as attempt 1.
* Every attempt-1 P1 finding is reported resolved by the personas that raised it.
* Raw counts, before deduplication:

  | Persona | Verdict | P0 | P1 | P2 | P3 |
  |---|---|---|---|---|---|
  | Constitution | ADVISORY | 0 | 0 | 2 | 6 |
  | Go | FAIL | 0 | 3 | 4 | 3 |
  | Scope | ADVISORY | 0 | 0 | 3 | 3 |
  | Learnings | ADVISORY | 0 | 0 | 4 | 3 |
  | Architecture | ADVISORY | 0 | 0 | 2 | 4 |
  | Parity | FAIL | 0 | 2 | 3 | 3 |
  | Security | FAIL | 0 | 2 | 3 | 2 |

Rationale: there are no P0 findings. Six deduplicated P1 findings block harvest, so the decision
is FAIL.

### P1 findings (deduplicated)

1. **Wrong fixture storage root.** `init` creates `.backlog`, not `.backlogit`, and the workspace
   resolver checks `.backlog` first. The protocol asserted `.backlogit` and an unset
   `BACKLOGIT_WORKSPACE_DIR` was not required. Raised by Go and Security.
2. **The `marker-mismatch-is-residual` row is infeasible as written.** `update --json` is a
   boolean output flag, and no CLI flag sets `custom_fields`. Raised by Go.
3. **Green makers have no harness classification.** UCS2a, UCS2b, and UCS4 named other units'
   selectors as their green commands, which breaks the task-scoped selector rule and would let
   harness-architect scaffold unplanned harnesses. They must be `covered-by` exempt tasks with
   full contract blocks. Raised by Go.
4. **Ship cannot run the E1 server check.** `backlogit_get_version` is not in Ship's tool set,
   and a CLI version check reports the CLI binary, not the served one. Raised by Parity.
5. **Old unconditional sentences survive a green harness.** The harness asserts new tokens but
   never asserts that the old "any `active` member halts" sentences are gone from Ship Step 4.0,
   Step 4.1b, the P-002.6 per-wave step 1, "Active leftovers", and the `WAVE_NO_PROGRESS` row.
   Raised by Parity (with related Constitution and Scope P2s).
6. **The E3 grant can be self-attested.** An unmerged memory file plus a comment from any actor
   satisfied the recognition rule. Raised by Security.

### P2 and P3 findings carried into the revision

* Clean-tree precheck must include untracked files (`vcs.modified` counts them); write evidence
  only after the last build; the `-dirty` string check proves nothing on its own.
* Green commands need `-v` and explicit `--- PASS:` lines for the named subtests.
* UCS4 default when `claim_overrides` is absent must leave `active_residual` unchanged.
* Ship Step 2 item 1 also defines `ready_k` as queued tasks.
* UCS3 robustness: `require` lookups, `NO_COLOR`, ANSI stripping, `errors.As` exit code, a
  24-outcome-line count.
* Slice anchors must be line-anchored, and each P-002.2 lookup must match exactly one row.
* UCS2a size: split or justify.
* Post-merge closure is Ship's standard closure, outside the tasks.
* One build and one containment check per proof unit, fresh fixture per row.
* Containment baseline must cover the whole live storage root except the index database.
* The dispatch block must copy the in-force conditions verbatim, read the Harvest Record from
  `origin/main`, use a delimited single block, and compare an ordered member list.
* Report templates and P-005 recording for the `BOOTSTRAP_*` halts and
  `PROOF_CONTAINMENT_BREACH`.
* Return-blocked removal expires the exception.
* Pin the `shipment claimed` reason literal and the comment delta shape.
* Re-added member and re-claim epoch behavior must be stated.
* The served storage root must be passed to Ship, with a scoped P-012.3 exception for the raw log
  read.
* Add `scripts/md-lint.ps1` to the harvest and pre-merge gates; evidence files need docline
  frontmatter and an H1.
* RB11: the UCS1 guard runs only on code-changing pull requests (CI paths filter).
* RB12: no stderr suppression, check `queue/` and `archive/`, and record why a chore root.
* Re-check revocation before pull request creation and before merge.
* Exact phrase tokens replace the loose `idempotent` and `re-read` tokens; `P-005` in Step 4.1b;
  pin the Step 4.1b heading.

Recommendation: revise the plan and the source decision (E1 owner, E3 recognition, return-blocked
expiry) and re-run every persona. Attempt 3 is the final allowed attempt.

<!-- plan-review-attempt: 2 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

* Attempt: 3, the final allowed attempt. Reviewed plan SHA-256 prefix: `91305A6BBBF0` (the
  attempt-3 revision).
* `TOOL_OK: reviewer-subagent-dispatch`.
* Personas: 7 dispatched, 7 returned. Same persona set as attempts 1 and 2.
* The personas that raised the attempt-2 P1 findings report all six resolved.
* Raw counts, before deduplication:

  | Persona | Verdict | P0 | P1 | P2 | P3 |
  |---|---|---|---|---|---|
  | Constitution | ADVISORY | 0 | 0 | 7 | 5 |
  | Go | ADVISORY | 0 | 0 | 2 | 5 |
  | Scope | ADVISORY | 0 | 0 | 3 | 3 |
  | Learnings | ADVISORY | 0 | 0 | 2 | 5 |
  | Architecture | FAIL | 0 | 2 | 3 | 3 |
  | Parity | FAIL | 0 | 1 | 4 | 2 |
  | Security | FAIL | 0 | 1 | 3 | 2 |

Rationale: no P0 findings. Four deduplicated P1 findings block harvest, so the decision is FAIL.
Each one is a contract-consistency gap. None changes the chosen start semantics, the DAG, or the
exception matrix.

### P1 findings (deduplicated)

1. **The first condition of the `WAVE_NO_PROGRESS` row still contradicts the new `ready_k`.**
   * The P-002.2 row still says "no `queued` member has all dependencies terminal".
   * UCS2b rewrites `ready_k` to include claim-assigned members, but leaves that first condition
     unchanged.
   * Required fix:
     * UCS2b rewrites the first condition to "no `queued` or claim-assigned member".
     * The UCS1 `Policy` subtest asserts the new phrase in that row.
     * The same subtest adds `assert.NotContains` for the old phrase.
   * Raised by Architecture.
2. **The scoped P-012 raw-log exception leaves out `B`'s own shipment log.**
   * The exception covers only the raw logs of the claimed member items.
   * The in-force check, the revocation check, and the E3 recognition check all need to read
     `<served storage root>\logs\<B>.jsonl`.
   * Required fix:
     * Extend the exception to that file at the four checkpoints: claim, each wave, pull
       request creation, and merge.
     * State the extension in dispatch item (2).
   * Raised by Architecture, with a related Parity P2.
3. **The E3 replacement list leaves the installed policy halts in force.**
   * Dispatch item (3) replaces only Ship text.
   * These policy rules would still halt the bootstrap session on its claim-assigned members:
     * the P-002.6 per-wave step 1;
     * "Active leftovers";
     * the Definitions entry for `ready_k`;
     * the `WAVE_NO_PROGRESS` row;
     * Ship Step 4.6 item 1;
     * Ship Step 4.0 item 7.
   * Required fix:
     * Add all of these to the E3 replacement list.
     * Declare `WAVE_CLAIM_STATE_INDETERMINATE` and `TASK_START_NOT_RECORDED` as recognized halts
       for the session.
     * Update the E3 wording in the source decision to match.
   * Raised by Parity.
4. **`origin/main` is a local ref that can be forged or stale.** Both the E3 grant check and the
   Harvest Record read trust it. Required fix:
   * Run `git fetch origin main`.
   * Require `git remote get-url origin` to be the canonical repository URL.
   * Require `git rev-parse origin/main` to equal the hash from
     `git ls-remote origin refs/heads/main`.
   * For the grant file, find the commit that added it and confirm that commit belongs to a
     merged pull request, with `gh api repos/softwaresalt/backlogit/commits/<sha>/pulls`.
   * Apply the same checks to the Harvest Record read.
   * Raised by Security.

### P2 and P3 findings carried forward

Carry these into any continuation the operator authorizes.

**Harness assertions and proof checks**

* `covered-by` exempt commands must also probe the task's own deliverable, not only the owner
  selector (P-002.3).
* Add `assert.NotContains` for two policy sentences:
  * "A member still carrying `active` at wave admission is a claim from a prior wave that never
    reached `done`".
  * "The scheduler never admits a new wave over an unfinished claim".
* Make "no member of `ready_k`" in Step 4.6 item 1 mandatory and asserted.
* Replace the fixed 24-line outcome count with `len(scenarios)` (at least 24).
* Use anchored regexes that match each evidence marker exactly once, and reject `FAIL` and
  `ABORT` lines.
* Back up fixture frontmatter before editing it (Principle VII).
* Containment checks:
  * handle volatile writer paths, or add an idle control pair;
  * set `GIT_CEILING_DIRECTORIES`;
  * take a whole-repository `git status` baseline.
* Run UCS5b only after UCS5a: concurrent windows collide.

**Permanent Ship text**

* Add a served-root rule: halt as indeterminate when the root is unknown, and name an
  Orchestrator follow-up.
* Remove the 6d233d21-specific E1 parenthetical.
* Pass both the workspace root and the storage root.

**Dispatch and grant checks**

* Map each R3 check to the tool that performs it.
* Make revocation matching lenient.
* Tie the E3 grant file to `B=<B>` and to the Harvest Record SHA.

**Other P2/P3 items**

* Constitution X/VI/IX deviation mapping.
* Quality-gate order.
* `QuoteMeta` on anchors.
* Scope the P-002.2 slice.
* Check `ctx.Err` before `errors.As`.
* Name the baseline scenario.
* Put excerpts in code fences.
* Add a closure-pending checkpoint.
* Pin only the literals.

### Gate outcome: HALT

* This was attempt 3, so both allowed re-entry cycles are used. The plan-review attempt counter
  now reads 3.
* Stage halts and requires operator intervention.
* No harvest runs, and no backlog items, dependency edges, or shipment are created.
* The operator's bootstrap authorization covers the bootstrap scope. It does not waive review
  findings.

Escalation (P-013.6). The route was resolved from a fresh read of `.autoharness/config.yaml`:

* The config is `schema_version` 1.1.0. Focused JSON Schema validation against the installed
  `harness-config.schema.json` returned 0 errors.
* There is no `model_routing.stage.escalation` route and no legacy flat `escalation` route, so
  the route falls back to `tier3`: `gpt-6.1-sol` / `openai` / `xhigh`.
* That route differs from Stage's own route (`claude-opus-5.5` / `anthropic` / `high`), so the
  same-route guard does not fire.
* The engram handoff target is unavailable: the CLI binding failed twice with the daemon not
  ready, and no third bind was attempted.
* Outcome: `ESCALATION_DEGRADED`, so Stage falls back to an operator halt. No fourth attempt is
  run.

A continuation needs new operator authority. The operator can either:

* authorize one more targeted revision and review cycle that fixes the four P1 findings above;
  or
* decide to rescope or abandon the work.

<!-- plan-review-attempt: 3 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: ADVISORY

* Attempt: 4, the single operator-authorized final cycle. Reviewed body: SHA-256 of the UTF-8
  text before the first `## Plan Review` line, prefix `ADBF9F245D84`. (The earlier
  `91305A6BBBF0` convention is not reproducible, so attempt 4 defines it explicitly.)
* `TOOL_OK: reviewer-subagent-dispatch`. 7 dispatched and 7 returned, the same persona set as
  attempts 1 to 3. Engram CLI was live for this attempt (bounded `query-memory` and `search`).
* Structural rows: plan hardening present, `Constitution Check: documented-deviations`, and the
  risky actions carry `ProposedAction` / `ActionRisk` (strict-safety).
* All four attempt-3 P1 findings are reported RESOLVED by every persona, including the three
  personas that raised them (Architecture: 1 and 2; Parity: 3; Security: 4).
* Raw counts, before deduplication:

  | Persona | Verdict | P0 | P1 | P2 | P3 |
  |---|---|---|---|---|---|
  | Constitution | ADVISORY | 0 | 0 | 4 | 4 |
  | Go | ADVISORY | 0 | 0 | 1 | 5 |
  | Scope | ADVISORY | 0 | 0 | 2 | 3 |
  | Learnings | ADVISORY | 0 | 0 | 1 | 4 |
  | Architecture | ADVISORY | 0 | 0 | 1 | 4 |
  | Parity | ADVISORY | 0 | 0 | 1 | 2 |
  | Security | ADVISORY | 0 | 0 | 1 | 3 |

Rationale: no P0 or P1 findings, and P2 findings remain, so the gate decision is ADVISORY. The
operator decides whether to proceed with these findings recorded as follow-ups, or to revise.

### P2 findings (deduplicated)

1. **UCS1 literal delimiters.** The Step 4.6 `Contains` literal ``no member of `ready_k` is
   `active` `` keeps a trailing space under CommonMark, while the mandated UCS2a sentence has a
   comma after the backtick, so a literal copied exactly cannot pass. The same pattern occurs at
   the other literals ending in a backtick. Fix: state that literals are trimmed. (Go)
2. **Leftover `ready_k` prose.** The P-002.6 `Wave k` bullet ends with "the queued members whose
   every dependency has reached a terminal-success status"; UCS2b respecifies only the formula.
   Fix: UCS2b rewrites the whole bullet, and UCS1 `Policy` asserts the old prose absent.
   Same-contract. (Scope, Architecture)
3. **Dispatch block shell forms.** `git show $m:<path>` and unquoted `origin/main^{commit}` fail
   in PowerShell; the VMR body already uses `"${m}:<path>"` and `'origin/main^{commit}'`. Fails
   closed, but halts wrongly. Same-contract. (Parity; Go P3)
4. **Grant provenance breadth.** VMR proves only that the grant file reached `main` through some
   merged pull request, which could be a large unrelated one, including the staging pull
   request. Fix: require the introducing pull request to change only the grant path and not be
   the staging pull request. (Security)
5. **VMR step 5 operability.** `commits/<sha>/pulls` may first report a pull request into an
   intermediate branch; requiring `base.ref` `main` for every commit could halt a legitimate
   read. Fix: dry-run VMR on the merged plan path before dispatch, or accept any merged pull
   request chain ending at `main`. (Learnings)
6. **Permanent P-012 declaration rule.** After merge, repaired Step 4.0 and Step 4.1b need member
   log reads for every shipment, but the declaration rule is scoped to 2A355F83. Fix: one UCS2a
   sentence requiring a session-record declaration until a configured tool returns the event
   stream. (Constitution)
7. **Principle VII.** UCS5b overwrites fixture frontmatter in place without a backup, and the
   Constitution Check VII line omits it. Extends the carried backup item. (Constitution)
8. **P-021 C2.** RB11 defers the CI paths-filter gap and the history-read tool without naming
   who captures the `DEFERRED SCOPE EXPANSION` stash entry, and when. (Constitution)
9. **P-021 C3/H14.** The carried P2/P3 items left "for review judgment" need an explicit operator
   disposition (fold in or accept the risk) before completion. (Constitution)
10. **UCS2a size.** Eight edit sites in one file; within the counts, but watch duration
    telemetry. No plan change required. (Scope)

### P3 findings (summary)

Name the full Step 4.0 token set `K` in UCS2a; define `active_ids` as residuals only; resolve the
"never treats an `active` member as satisfied" assertion against a still-true sentence; clarify
`Preserved` counts for not-yet-existing rows; add a token check on Step 4.0 item 7; name the
grant-file writer, merger, and `<yyyy-MM-dd>` resolution; clarify "claim" versus Step 4.1b and
"claim or start" in Step 4.0 item 6; say the H1 crash note applies to claim-assigned members
only; note the `WAVE_NO_PROGRESS` convergence scope and a separate claim-assigned census count;
qualify the circuit-breaker row as the D2 residual class; confirm merge commits pass VMR step 5;
pin `gh --hostname github.com`; check the URL before fetching; anchor Harvest Record delimiters
at line start; state non-shipment-mode classification is skipped; note UCS5a/UCS5b fixture-only
creation; record NotContains non-vacuity in UCS1 red evidence; UTF-8 without BOM for UCS5b
fixture edits; quote `#` in evidence frontmatter; the chore-root choice; the unrequested `B=<B>`
grant binding and extra dispatch field are harmless. Carried attempt-3 P2/P3 items are not
worse and were not re-raised.

### Gate outcome: HALT pending operator disposition

* ADVISORY needs explicit operator confirmation before harvest. The standing direction to keep
  working autonomously waives no gate, so no `operator_authorization` is recorded here.
* No harvest ran. No backlog items, dependency edges, or shipment exist, and the exception is not
  bound to any shipment ID.
* No fifth review cycle is authorized or needed for approval. On explicit operator approval of
  this ADVISORY, Stage appends `operator_authorization: approved` to this section and harvests
  this body unchanged, recording the P2 findings above as follow-ups. Revising the body instead
  would need new authority for a fifth cycle.
* P-013.6 escalation does not apply: no consecutive-failure threshold was crossed.

<!-- plan-review-attempt: 4 -->

### Operator authorization (attempt 4 ADVISORY)

operator_authorization: approved

* Recorded by Stage at harvest resumption from checkpoint `checkpoint-20261002-031116.json`.
  This applies to the attempt-4 ADVISORY record above. No fifth review cycle ran, and the
  reviewed body (SHA-256 prefix `ADBF9F245D84`) was re-verified unchanged before this record was
  appended.
* Exact preceding request, presented by the Orchestrator to the operator: "The remaining approval
  is specific: approve the ADVISORY result and resume checkpoint-20261002-031116.json for
  harvest. Stage can then create the seven-task bootstrap shipment, with the findings carried
  into implementation rather than silently ignored."
* Operator reply, received `2026-10-02T03:20:52.330Z`: the full continuation directive. The
  Orchestrator relayed these verbatim excerpts: "If you were planning, stop planning and start
  implementing" and "Keep working autonomously until the task is truly finished, then call
  task_complete." Stage did not receive the complete reply text, and quotes only these excerpts.
* Basis: the Orchestrator publicly interprets this direct reply to the single, specific approval
  and selected-checkpoint request as contextual approval of the attempt-4 ADVISORY and as
  direction to harvest. The reply does not contain the literal word "approved".
* Limits: this record is not a blanket advance approval, a gate waiver, a dark-mode trigger, a
  merge or admin approval, an E3 grant, a claim authorization, or a Condition B attestation.

## Harvest Attempt 1: HALT (RB12 chore-root collision)

* Time: `2026-10-02T03:29Z` to `03:31Z`. Stage resumed from `checkpoint-20261002-031116.json`
  after owner validation and a bounded Engram-backed prune. It refreshed the metadata catalog,
  the type list, and the chore and task WIT metadata, then started Step 5.
* Root created: chore `001-C`. The chore allocator numbers each type separately, so the next
  chore number is `001`, not the next global number. Stage checked `195` before creation instead
  of the actual candidate `001`, which departs from the RB12 order (check first, then create).
* Collision: the first task child of `001-C` resolves to `001.001-T`. That ID already exists as
  `.backlogit/archive/001.001-T.md` (parent `001-F`, status `done`), together with `001.002-T`
  to `001.011-T` and their subtasks. One task create under `001-C` failed at the pre-write
  uniqueness chokepoint with "artifact ID already exists on the canonical filesystem". Nothing
  was written.
* RB12 applies: any collision halts harvest. No task, dependency edge, link, shipment, Harvest
  Record, or `BOOTSTRAP_EXCEPTION_RECORDED` comment exists. The exception is not bound to any
  shipment ID.
* Partial root retired: `001-C` had zero children. Stage added a `HARVEST_HALT_RB12` comment
  (actor `stage`) and archived it from `queued` (`archived_status: queued`). This is reversible.
* Protected state is unchanged: `154-S`, `182-F`/`182.001-T`, and `183-S` to `194-S`, with an
  aggregate hash taken over 84 files before and after.
* The reviewed body is unchanged (SHA-256 prefix `ADBF9F245D84`). Re-rooting needs an explicit
  operator decision, because RB12 records the chore root as a reviewed choice. Two options:
  * use a `feature` root, as the compound learning recommends; feature numbering is free at
    `195`;
  * fix the allocator separately, which is out of this plan's scope.
* Disposition of the attempt-4 findings is prepared in
  `docs/memory/2026-10-02/stage-2a355f83-harvest-halt-rb12.md`. It applies when harvest
  resumes, and nothing is marked fixed.

## Harvest-Time Deviation: Feature Root (RB12, operator-approved)

This section records a packaging-only deviation made at harvest. It does not edit the reviewed
body above, does not change any reviewed design choice other than the root artifact type, and is
not a fifth plan review.

* Preceding scope presented to the operator (quoted as relayed to Stage, spacing as received):

  > Harvest hit a known ID collision: the chore allocator selected001-C, whose task IDs collide with archived tasks under001-F. No tasks were overwritten; the empty chore was archived, and no shipment exists yet. The documented workaround is a feature root for the same seven tasks, preserving the scope,DAG andexception without changing theGoallocator. Approve that packaging-only change and resume checkpoint-20261002-033403.json for harvest. The reviewedplan explicitlyspecifiedachore,soStagepausedratherthansilentlychangingit.

* Operator reply, exactly: `Approved`, at `2026-10-02T03:52:58.782Z`.
* What it approves: Option A only, a `feature` covering root for the same seven tasks, and the
  owner resume of `checkpoint-20261002-033403.json`. It is not E3 or any temporary session
  authority, not a merge or admin approval, not dark mode, not a scope expansion, not a claim
  authorization, and not a Condition B attestation.
* What changes: the root's artifact type, from the reviewed `chore` (D4, RB4, RB12) to `feature`
  `195-F`. The scope is still internal maintenance. The feature type is a packaging workaround
  for the per-type hierarchical ID collision recorded in Harvest Attempt 1; it does not claim a
  new user-facing capability. The reviewed chore rationale above is left as written and is
  superseded for packaging only. Units, DAG, waves, exception, gates, and findings are unchanged.
* RB12 pre-create check, run this time against the actual allocator: the feature allocator takes
  the highest root ordinal among `feature` items (`194`) plus one, so the candidate was `195-F`.
  A recursive scan of `.backlogit` (queue, archive, logs, and all other subdirectories) and an
  index query found no `195-F` and no `195.001-T` to `195.007-T` before creation. The shipment
  candidate `195-S` was likewise absent before creation. There was no allocator change, no
  counter manipulation, and no second chore attempt. `001-C` stays archived and untouched.
* Stash provenance: no stash entry `2A355F83` exists (operator-directed bootstrap ID), and the
  retired `001-C` carries no `source_stash_id`, so no provenance correction was needed and none
  was made.
* Reviewed body unchanged: SHA-256 prefix `ADBF9F245D84` over the bytes before the first
  `## Plan Review` heading, re-verified after this append.

## Harvest Record

The block below is the authoritative binding named by the Exception and Dispatch Preconditions
section. The member list is ordered and equals `195-S` `custom_fields.items` at harvest: the root
comes first (`feature` under the approved deviation, in place of the reviewed chore root), then
the tasks in harvest order.

<!-- BEGIN:harvest-record -->

```text
scope: 2A355F83
B: 195-S
members: 195-F, 195.001-T, 195.002-T, 195.003-T, 195.004-T, 195.005-T, 195.006-T, 195.007-T
root: 195-F (artifact_type feature; operator-approved RB12 packaging deviation, Approved 2026-10-02T03:52:58.782Z)
unit_map: UCS1=195.001-T, UCS3=195.002-T, UCS2a=195.003-T, UCS2b=195.004-T, UCS4=195.005-T, UCS5a=195.006-T, UCS5b=195.007-T
waves: 1={195.001-T, 195.002-T}; 2={195.003-T, 195.004-T, 195.005-T}; 3={195.006-T, 195.007-T}
task_blocks_edges: 195.003-T->195.001-T; 195.004-T->195.001-T; 195.005-T->195.002-T; 195.006-T->195.003-T,195.004-T,195.005-T; 195.007-T->195.003-T,195.004-T,195.005-T
shipment_blocks_edges: 195-S->154-S (P only; no edge onto any future Condition B shipment C)
links: 195-F related_to 182.001-T
reviewed_body_sha256_prefix: ADBF9F245D84
recorded_at: 2026-10-02T04:17Z
```

<!-- END:harvest-record -->

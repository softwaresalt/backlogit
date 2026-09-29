---
title: "Stage session memory: 173-F option A re-plan, attempt 6 FAIL, UR3 flake prerequisite"
doc_type: memory
date: 2026-09-28
agent: stage
branch: stage/173f-option-a-replan
base: a7462173
status: halted-part-1-circuit-breaker
---

# Stage: 173-F option A re-plan and UR3 flake prerequisite

## Session context

* Route (P-013.5): claude-opus-5.5 / anthropic / high.
* Base: `main` == `origin/main` at `a7462173`.
* Branch: `stage/173f-option-a-replan`. Single branch, no worktrees.
* Startup:
  * ALL_TOOLS_OK.
  * INDEX_SYNC_OK: 1769 items at start, 1770 after the session.
  * Stage checkpoints: none active and no anomalies, so no recovery was
    needed.
  * Hook poll: no events.
* Untracked files left untouched and not staged:
  * `.backlogit/checkpoints/checkpoint-20260929-033703.json`
  * `.backlogit/reconcile/155-S-*.md`
  * `.backlogit/telemetry.jsonl`
  * `docs/memory/2026-09-28/orchestrator-decisions-and-154-s-revalidation-memory.md`

## Operator decisions consumed (2026-09-28T22:22 -07:00)

1. **"Decision: option A."** The marker is kept through block, unblock and
   `ReturnBlockedItem`. An item is claim-activated only if all three hold:
   * its status is `active`;
   * its marker equals the currently active shipment's ID;
   * it is in that shipment's manifest.
2. **"Probably should fix a flaky test before a next task that could depend
   on it."**

## Part 1: 173-F re-plan under option A

**Artifacts:**

* Decision: `docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`.
  * Contents: the normative predicate, producer rules, residuals R1–R3 and
    the Consequences section.
  * P2-3 factual corrections from review applied: the `ReturnBlockedItem`
    row ends `blocked`, and R2 is restated.
* Plan: `docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md`.
  * The body was rewritten; `docline.status: revised`.
  * Units: U0a, U0b, U0c (new), U1b (new: claim-crash recovery accepts
    members carrying this shipment's marker), U1, U2, U3, U4, U5.
  * F1–F8 remediated under option A, and the plan was re-hardened.
  * The review history from attempts 1–5 is preserved.

**Plan-review attempt 6: `decision: FAIL`.**

* `dispatch_mode: multi-agent-dispatch`, six personas.

  | Persona | Verdict |
  |---|---|
  | Constitution | FAIL (P1) |
  | Go | ADVISORY |
  | Scope | ADVISORY |
  | Learnings | ADVISORY |
  | Architecture | ADVISORY |
  | Parity | ADVISORY |

* **P1-1:** U0b (`173.007-T`) must be a dependency of U1 (`173.001-T`),
  because U1 is what turns U0b green. U2 becomes verification-only.
* **P2-1..P2-10:**
  * U1b acceptance-criteria wording, plus stale-foreign-marker fixtures.
  * U4(2) claim-crash scenario cannot be executed. Claims emit no evidence
    events, recovery skips journals that are not intent journals
    (`shipment_recovery.go:128`), and the UR3 crash child only handles
    block/unblock. Re-specify it as an in-process failpoint on the Nth member
    plus a reopen, or drop it.
  * Revert guidance: reverting U1 withdraws the contract.
  * U5 must depend on U4.
  * Manifest JSON path `[].custom_fields.items[]`: 0 or more than 1 active
    shipments is fail-closed.
  * R3: the shipment list is index-backed on both transports. Split it into
    a consumer rule and `sync` remediation.
  * U3 must inject a plain error or `ErrWriteNotApplied`, never
    `ErrWriteIndeterminate`.
  * Mixed-binary rollout checkpoint.
  * Cite the atomic-multi-item-claim learning and waive its clear-on-exit
    rule.
* The full text is in the plan, `## Plan Review` attempt 6.

**Circuit breaker.** Attempts 5 and 6 are two consecutive FAILs, so the
breaker tripped and Part 1 **halted**:

* no harvest update: the `173.00x-T` contracts are still the pre-option-A
  text and are stale;
* U0c and U1b tasks were not created;
* the `154-S` manifest was not changed;
* the `do-not-claim-until-convergence` hold label was kept;
* `C29EBEE5` stays ACTIVE, with an attempt-6 note appended.

## Escalation payload (P-013.6)

* `threshold_kind: review_fix_cycles`, `count: 2` (consecutive plan-review
  FAILs: attempt 5 on 2026-09-28 at `ba303ee2`, attempt 6 on 2026-09-28 on
  this branch).
* **Failure summary.** The re-planned `173-F` contracts under option A still
  have one sequencing P1 (U0b→U1 edge) and ten precision/executability P2s.
  The option A semantics themselves were not rejected by any persona.
* **Last actions and observations:**
  * attempt-5 findings F1–F8;
  * operator option A decision;
  * plan rewrite;
  * attempt-6 persona verdicts (above).
* **Artifact refs:**
  * `docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md`
    (attempt 5 and attempt 6 `## Plan Review`);
  * `docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`;
  * stash `C29EBEE5`;
  * `.backlogit/queue/154-S.md` banner.
* **Telemetry evidence:** persona outputs are summarized in the attempt-6
  review section. No separate telemetry artifact.
* **`resolved_escalation_route`:** `gpt-6-sol` / `openai` / `xhigh`.
  * Source: `.autoharness/config.yaml` `model_routing.escalation`, a legacy
    flat field, read fresh this session.
  * It differs from the Stage route (claude-opus-5.5 / anthropic / high), so
    it is **not** same-route degraded.
* **Handoff medium.** Engram exposes no payload-accepting MCP tool. This
  memory file (engram-indexed) is the file-based handoff. If a file-based
  handoff is not accepted, treat it as `ESCALATION_DEGRADED` and use the
  operator-halt fallback. Either way, no third re-review was attempted.
* **Resumption checkpoint:** this memory file plus the plan's attempt-6
  section.
  * Next action: with operator authorization, run attempt 7 applying P1-1
    and P2-1..P2-10.
  * Then update the harvest: revise `173.001-T`..`173.007-T` via
    `backlogit update`; create U0c and U1b under `173-F` with blocks edges
    onto `181.001-T`; `add_to_shipment` `154-S`.
  * Then archive `C29EBEE5` and remove only the
    `do-not-claim-until-convergence` label from `154-S`.

## Part 2: UR3 flake prerequisite (completed)

**Triage.**

* `BDA56ED8` (captured 2026-09-26 by Ship during 174.078-T) is the
  **survivor**.
* `46A898B8` (captured 2026-09-28, v1.11.0 release gate) is the
  **duplicate**, with the same root cause:
  * the crash child writes `ReadyPath` with a non-atomic `os.WriteFile`;
  * the parent's `json.Unmarshal` runs on an empty or partial read.
* Both entries were edited with notes, then archived, not removed.
* Duplicate scan: 46A898B8 only.
* Late-ID reconciliation: no residual-risk record cites BDA56ED8, so the
  N/A values stand.

**Deliberation.** `docs/decisions/2026-09-28-ur3-crash-ready-handoff-flake-deliberation.md`.

* Chosen option: 2, reader-side tolerance.
* Rejected: the atomic temp-then-rename writer, and a sentinel-file
  protocol.

**Plan.** `docs/exec-plans/2026-09-28-ur3-crash-ready-handoff-flake-plan.md`
(status `reviewed`).

* Plan-review attempt 1: ADVISORY. One P2: the helper signature could not
  carry the decode error that AC5 needs.
* Attempt 2: **PASS** from all five personas (Constitution, Go, Scope,
  Learnings, Architecture). Parity was not triggered.
* Post-PASS P3 wording fixes were applied and recorded.

**Harvest.**

* Feature `181-F`, "Test-harness reliability: deterministic UR3 crash-ready
  handoff" (high).
* Task `181.001-T`, "Tolerate partial UR3 crash-ready marker in harness poll
  loop" (high, tests domain, reproduce-first ACs).
* Shipment `182-S`, "Flake prerequisite: deterministic UR3 crash-ready
  handoff (181-F)" (high, queued).
  * Items: `[181.001-T]`, a single member as the operator instructed (157-S
    precedent). `181-F` is NOT in the manifest; this is an open decision.
  * Not claimed.

**Dependency edges (verified with `get_dependencies`).**

* `154-S` → `182-S`, `blocks`. This is the shipment-to-shipment path.
  `154-S` deps are now [`155-S`, `182-S`].
* `173.001-T`, `173.002-T`, `173.003-T`, `173.004-T` and `173.006-T` each →
  `181.001-T`, `blocks`. These are every `173-F` task whose gate runs
  `./internal/core/...`.
* Excluded: `173.005-T` (docs) and `173.007-T` (cli/mcp).

## 154-S state after the session

* Status: `queued`. Deps: [`155-S` (shipped), `182-S` (new)].
* Items unchanged: `173-F`, `173.001-T`..`173.007-T`.
* Labels unchanged:
  * `bootstrap-exception`
  * `convergence-prerequisite`
  * `bootstrap-bypass-approved-conditional`
  * `do-not-claim-until-convergence`
* Banner "Remaining gate" rewritten to record:
  * the attempt-6 FAIL and the circuit breaker;
  * the option A decision link;
  * the `182-S` prerequisite and its task edges.

## Open operator decisions

1. **Authorize plan-review attempt 7 for `173-F`** after applying the
   attempt-6 remediation (P1-1, P2-1..P2-10).
2. **`182-S` composition.** The shipment is single-member (`181.001-T` only),
   so covering feature `181-F` is outside the manifest. Ship will need to
   close `181-F` manually, or `181-F` can be added.
3. **Carried from earlier sessions:** label-aware claim refusal (`AF1E5075`)
   and the 074-DL post-ship attestation.

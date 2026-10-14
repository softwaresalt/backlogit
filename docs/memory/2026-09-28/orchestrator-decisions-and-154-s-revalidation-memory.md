---
title: Orchestrator decisions, auto-tune, and 154-S re-validation session memory
doc_type: memory
source: docs/memory/2026-09-28/orchestrator-decisions-and-154-s-revalidation-memory.md
---

## Outcome

The operator accepted the Orchestrator's recommendations on 2026-09-28. Three
PRs merged with merge commits, and every merge passed the P-018 gate and CI.

| PR | Merge | Content |
|---|---|---|
| #455 | `920ee3eb` | 074-DL and 075-DL decided. Hold-only `blocks` edges from 141-S, 147-S, 152-S, 177-S, 178-S, and 179-S onto 154-S. 154-S waiver conditionally approved (label `bootstrap-bypass-approved-conditional`). Stash 513E62AB archived. |
| #456 | `ba303ee2` | Auto-tune pass (075-DL): operator config commit `b855cfc3` restored `alt_doc_review` as google / gemini-3.8-flash. The manifest ESCALATION_* values are now gpt-6-sol / openai / xhigh, and doc-review SKILL.md was re-rendered. |
| #457 | `3e25ade6` | Stash FB0A850B archived. The C29EBEE5 re-validation returned FAIL, which is recorded on 154-S. |

## Decisions Recorded

* 074-DL: no exemptions. The interim consumption signal is an operator
  attestation comment on 154-S after it ships. The long-term signal is the
  pipeline-topology pre_claim gate, after A592FC1C is fixed. The L1 check is
  folded into AF1E5075. Enforcement stays manual until AF1E5075 lands.
* 075-DL: restore `alt_doc_review`, fix it through auto-tune, and treat the
  removal in `2ac59314` as unintentional.
* 154-S waiver: approved only on the recorded conditions.

## Current Hold

154-S keeps `do-not-claim-until-convergence`. Plan-review attempt 5 of C29EBEE5
failed. Only 173.002-T passed. The P1 findings are:

* marker lifecycle across block, unblock, and return is undefined;
* claim-crash recovery rejects marked members with `ErrShipmentConflict`, and no task covers that path;
* a stale marker is misread as claim-activated;
* 173.006-T has no real RED test.

## Next Steps

1. Operator chooses the marker semantics: option (A) keep the marker and match
   it against the active shipment, or option (B) clear it on block and return,
   then re-mark it on unblock. Stage then revises the plan, runs review attempt
   6, and re-harvests the tasks, including U1b.
2. Optional: triage the flaky test stash 46A898B8 and its possible duplicate
   BDA56ED8 before 173.004-T runs.
3. Do not route 154-S, 153-S, or any shipment behind it.
4. The untracked checkpoint `checkpoint-20260929-033703.json` (resolved), the
   155-S reconcile files, and `telemetry.jsonl` are left for the operator to
   dispose of.

## Later Session Update (2026-09-28 22:22 -07:00)

* The operator chose option (A) for the marker semantics.
* The operator asked for `telemetry.jsonl` to stay untracked, so it is now
  listed in `.gitignore`.
* The operator asked for the flaky test to be fixed first.
* PR #458, the PR #455 review follow-up, merged at `a7462173`.
* Stage recorded option A in
  `docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`.
* Plan-review attempt 6 returned FAIL: 1 P1, which requires 173.001-T to
  depend on 173.007-T, plus 10 P2 findings. Attempts 5 and 6 are two
  consecutive failures, so the circuit breaker tripped. Attempt 7 needs
  operator authorization, and the 173-F task contracts are not yet updated.
* The flaky-test fix is task `181.001-T`, under feature `181-F`, in its own
  shipment `182-S`. Stash `46A898B8` was marked a duplicate of `BDA56ED8`.
  154-S now depends on 182-S through a `blocks` edge. The 173-F tasks that run
  `./internal/core/...` depend on 181.001-T.
* Staging PR #459 carries this work.
* 182-S has no unshipped predecessor and needs no waiver, so it is eligible for
  the normal Ship flow.

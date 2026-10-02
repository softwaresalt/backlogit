---
schema_version: "1.0"
doc_type: memory
title: Stage session memory for the 2A355F83 claimed-versus-started bootstrap
description: Verbatim bootstrap authorization, gate results, a plan-review halt after three attempts, the exact P1 fixes and dispatch preconditions, preserved paths, and counters.
chunk_strategy: h1-h2-h3
---

# Stage memory: 2A355F83 claimed-versus-started bootstrap

## Status

**HALTED at the plan-review gate.** The plan failed review three times, so both allowed
re-entry cycles are used. Escalation resolved to `ESCALATION_DEGRADED`, which means an operator
halt.

Nothing was harvested:

* no backlog items;
* no dependency edges;
* no shipment.

No bootstrap shipment ID exists yet, so the exception is not yet bound to one.

Stage did not:

* claim anything;
* attest anything;
* touch 154-S;
* remove any edge;
* repackage the frozen 46-task decomposition;
* modify source.

## Operator authorization (verbatim)

> Yes, I authorize that narrowly scoped bootstrap exception

* Received: 2026-10-01T17:29:30.253-07:00 (2026-10-02T00:29:30.253Z).
* What it answers: the Orchestrator's proposal for a small, separate prerequisite repair of
  claimed-versus-started tracking and Ship task selection.
* What it allows: repairing and verifying this consumer before truthful consumption attestation.
* What stays in force: the admission gate for every existing successor.
* What it is not:
  * attestation on the operator's behalf;
  * proof that consumption already happened;
  * a bypass of other gates;
  * a claim of 187-S;
  * a change to archived 154-S;
  * edge removal;
  * a ledger redesign;
  * dark mode;
  * preauthorization of a merge, or an admin fallback for merging;
  * a waiver of review findings.

## Artifacts

* Decision: `docs/decisions/2026-10-01-2a355f83-claimed-vs-started-bootstrap-deliberation.md`.
  * D1 to D5.
  * A one-shipment exception matrix.
  * Dispatch preconditions E1 to E4.
  * Risks RB1 to RB5.
  * Counters.
* Plan: `docs/exec-plans/2026-10-01-2a355f83-claimed-vs-started-bootstrap-plan.md`.
  * Hardened; reviewed body SHA prefix `91305A6BBBF0`.
  * Holds the three `## Plan Review` records.

## Gates passed

* Tools: backlogit MCP returned `TOOL_OK` (served version `v1.10.0-1023-g2c8759c3-dirty-debug`),
  and the index sync returned OK.
* Engram is degraded: there were 2 earlier CLI bind failures and no third bind was attempted.
  Fallback was targeted known-path lookups.
* Config: `.autoharness/config.yaml` is `schema_version` 1.1.0.
  * Focused JSON Schema validation against the installed `harness-config.schema.json` found
    0 errors. It was re-run before commit.
  * The full `verify-workspace` was not replayed; it failed twice earlier on unrelated
    portability warnings.
* Metadata: the `chore` type (prefix C) allows task children.
* 154-S: shipped with merge `6d233d21`. It has no `CONDITION_B_ATTESTED:` or
  `CONDITION_B_REVOKED:` line.
* Checkpoints: 85 enumerated, none anomalous, no active `stage` candidate.
* Hook events acknowledged through seq 3495.

## Design summary (not harvested)

* **Start semantics (D1/S4):** reuse the existing comment operation.
  * The record is a `comment` event with actor `ship` whose first line is exactly
    `WORK_STARTED: <S>`, where `<S>` is the session shipment. It must fall inside the current
    start epoch, which begins at the latest `status_changed` event to `active` with reason
    `shipment claimed`.
  * It is written once, through `backlogit_append_comment`. Ship re-reads the log before any CLI
    fallback, and after writing requires exactly one valid record.
  * A claim-assigned member is never moved again. No dispatch happens before the record is
    verified.
  * No new API, status, event, or schema is introduced. The served root is handed off, and a
    scoped P-012 raw-log read exception applies.
  * Bulk claim activation (M2) stays as it is.
* **Admission (D2):** a member counts as claim-assigned only if all of these hold:
  * it is `active`;
  * its marker equals the current active shipment;
  * it is an explicit member of the live manifest;
  * it has no start record.
  * It is admitted only when dependency-ready.
  * Genuine residuals, wrong or missing markers, ambiguity, stale reads, and drift all fail
    closed with `WAVE_CLAIM_STATE_INDETERMINATE` or `TASK_START_NOT_RECORDED`.
* **Planned DAG:**
  * One chore root.
  * Tests-first units:
    * UCS1 contract harness;
    * UCS2a (Ship Step 4.0/4.1b) and UCS2b (policy), both covered-by UCS1;
    * UCS3 simulator harness, with UCS4 covered-by UCS3;
    * UCS5a (positive proof) and UCS5b (fail-closed proof): current-HEAD producer/consumer proof
      in isolated private-clone fixtures.
  * Edges: UCS2a → UCS1, UCS2b → UCS1, UCS4 → UCS3, and UCS5a/UCS5b → {UCS2a, UCS2b, UCS4}.
    Shipment edge: `B` blocks-on `154-S` (P).
  * Waves: {UCS1, UCS3}, then {UCS2a, UCS2b, UCS4}, then {UCS5a, UCS5b}.
  * Estimate: 7 tasks, each under 2 hours, about 14 hours in total.
* **Exception matrix:** the exception applies only to bootstrap shipment `B`.
  * P (154-S shipped provenance) is still required.
  * Every existing successor keeps C.
  * Stop and expiry: merge, revocation, return-blocked, or scope drift.
  * Revocation is re-checked at claim, at each wave, before the PR, and before merge.

## Blocker and the exact fixes for a continuation

The four P1 findings from attempt 3 (the full P2/P3 list is in the plan's final review record):

1. UCS2b must rewrite the first `WAVE_NO_PROGRESS` condition to "no `queued` or claim-assigned
   member". UCS1 then asserts the new phrase and `NotContains` the old one.
2. Extend the scoped P-012 raw-log exception to `<served root>\logs\<B>.jsonl` at claim, at each
   wave, before the PR, and before merge.
3. Extend the E3 replacement list with:
   * P-002.6 per-wave step 1;
   * "Active leftovers";
   * the `ready_k` definition;
   * the `WAVE_NO_PROGRESS` row;
   * Ship Step 4.6 item 1;
   * Ship Step 4.0 item 7.

   Also declare `WAVE_CLAIM_STATE_INDETERMINATE` and `TASK_START_NOT_RECORDED` as recognized
   halts, and align the E3 wording in the decision.
4. Harden the `origin/main` trust used by the E3 grant check and the Harvest Record read:
   * fetch;
   * require the canonical remote URL;
   * require `rev-parse` to equal `ls-remote`;
   * require the grant commit to belong to a merged PR, checked via `gh api .../commits/<sha>/pulls`.

## Operator-only preconditions (unchanged)

* **E1:** the Orchestrator verifies that the served MCP binary carries the 154-S producer
  (`go version -m` evidence), or hands off to the correct binary. A CLI version never satisfies
  this.
* **E2:** the staging manifests are merged to remote `main`. Last known `origin/main` is
  `046c0130`.
* **E3:** an operator grant is needed to apply the replacement admission text for `B`'s own
  session. This is new authority and is not granted.
* **C for successors:** only a legitimate operator `CONDITION_B_ATTESTED:` line in
  `.backlogit/logs/154-S.jsonl`, written after the consumer is actually repaired and verified.

## Counters

| Counter | Value |
|---|---|
| Plan review | 3 of 3 attempts, all FAIL (P1 counts 9, 6, 4); halted |
| Engram bind failures | 2, carried forward |
| Full `verify-workspace` failures | 2, carried forward; not re-run |

## Disclosures

* `pwsh scripts/wave-scheduler-sim.ps1 -Scenario ...` was run twice as a read-only probe.
* `scripts/md-lint.ps1` was run over the whole repository, with 0 issues.

## Preserved paths (not staged)

* The five `.backlogit/checkpoints/checkpoint-20260930-{030939,042027,043858,044842,045011}.json`
  files.
* `.backlogit/stash.jsonl`
* `.backlogit/memories.json`
* `.autoharness/config.yaml`
* `docs/memory/2026-09-30-orchestrator-154s-closure-session.md`
* The existing git stashes.

## Next exact Orchestrator action

Present the halt to the operator and get one of these decisions:

* authorize exactly one more targeted revision and review cycle, limited to the four P1 fixes
  above; or
* rescope; or
* abandon.

Until then, no harvest, shipment, or Ship dispatch happens for 2A355F83.

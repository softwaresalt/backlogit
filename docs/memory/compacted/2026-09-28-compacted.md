---
title: "Compacted memory: 2026-09-28 gate decisions, 173-F option A re-plan, C29EBEE5 re-validation, and 513E62AB triage"
date_range: "2026-09-28 to 2026-09-28"
doc_type: compacted-memory
source_count: 4
---

# Compacted memory: 2026-09-28

Compacts 4 source files covering 154-S hold decisions, PR merges, 173-F marker lifecycle, and the UR3 flake prerequisite. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

Operator decisions and merges:
- PR 455 (merge 920ee3eb): 074-DL and 075-DL decided. Hold-only blocks edges were added from 141-S, 147-S, 152-S, 177-S, 178-S, and 179-S onto 154-S. The 154-S waiver was conditionally approved (label bootstrap-bypass-approved-conditional). Stash 513E62AB was archived.
- 074-DL: no exemptions. The interim consumption signal is an operator attestation comment on 154-S after it ships. The long-term signal is the pipeline-topology pre_claim gate, once A592FC1C is fixed. L1 was folded into AF1E5075. Enforcement stays manual until AF1E5075 lands.
- PR 456 (merge ba303ee2): auto-tune pass (075-DL). alt_doc_review was restored as google / gemini-3.8-flash. ESCALATION values in the manifest became gpt-6-sol / openai / xhigh. doc-review SKILL.md was re-rendered.
- PR 457 (merge 3e25ade6): stash FB0A850B archived. The C29EBEE5 re-validation FAIL was recorded on 154-S.
- PR 458 (merge a7462173): the PR 455 review follow-up.
- Marker semantics option A (operator): the scheduler-baseline marker is kept through block, unblock, and ReturnBlockedItem. An item counts as claim-activated only if all three hold: status is active; its marker equals the currently active shipment ID; it is in that shipment's manifest.
- Flaky-test prerequisite: the operator asked for it to be fixed first. The UR3 choice was option 2, reader-side tolerance. The atomic temp-then-rename writer and the sentinel-file protocol were rejected.
- 182-S composition: kept as a single member, [181.001-T]. Adding 181-F would make it multi-member, which condition (b) forbids before the 154-S marker exists, and 154-S depends on 182-S, which would deadlock. A governed post-ship close step for 181-F was recorded instead.

Stage re-plan and gate:
- The 173-F plan was rewritten under option A. Plan-review attempts 5 and 6 both FAILED, which tripped the circuit breaker. Attempt 6 raised a P1 (173.001-T must depend on 173.007-T) plus ten P2s. Stage halted and compiled an escalation payload (P-013.6). The resolved route was gpt-6-sol / openai / xhigh, which is not same-route. The memory file served as the file-based handoff medium. The source states that if that handoff is not accepted, the result is ESCALATION_DEGRADED with an operator halt.
- Attempt 7 required operator authorization. The re-plan was halted pending that authorization.

Triage of 513E62AB and FB0A850B:
- 513E62AB: six hold-only blocks edges were added to 154-S. Enforcement was described as manual policy, because the queue filter releases dependent shipments on any terminal status of 154-S.
- FB0A850B: routing drift was re-verified. Resolved by PR 454: escalation literals, the anchor family, and the stage and ship literals. Remaining: manifest ESCALATION values, stale config comments, and the alt_doc_review binding.

## Files modified or created

- docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md (normative predicate, producer rules, residuals R1 to R3)
- docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md (body rewritten; docline status revised; attempts 1 to 6 kept)
- docs/decisions/2026-09-28-ur3-crash-ready-handoff-flake-deliberation.md and docs/exec-plans/2026-09-28-ur3-crash-ready-handoff-flake-plan.md
- docs/decisions/2026-09-28-513e62ab-condition-b-enforcement-deliberation.md and 074-DL and 075-DL decision records
- .backlogit/queue/154-S.md banner and labels; 181.001-T and 181-F under flake plan; shipment 182-S; stash notes on AF1E5075, 6434A4D7, 513E62AB, FB0A850B, C29EBEE5, 46A898B8, and BDA56ED8
- .gitignore (telemetry.jsonl listed)
- Config: .autoharness/config.yaml (operator edit to restore alt_doc_review, committed in b855cfc3)

## Key learnings

- A shipment-to-shipment blocks edge releases on any of the six cascade terminal statuses (filterByResolvedDependencies and IsNoLongerBlockingStatus). ClaimShipment performs no dependency validation. A shipped-provenance guard is therefore needed.
- The C29EBEE5 re-validation found: a claim crash-recovery CAS failure (memberRecoveryCandidates compares full JSON, so a marked member fails with ErrShipmentConflict); an undefined marker lifecycle across block, unblock, and return; stale markers; a 173.006-T scenario that was not RED; and a 173.007-T scenario count above the limit.
- The UR3 flake root cause: the crash child writes ReadyPath with a non-atomic os.WriteFile, and the parent's json.Unmarshal can run on an empty or partial read. The entries were 46A898B8 (duplicate) and BDA56ED8 (survivor).
- The R3 indeterminate rule: CLI reads run recovery on open, but the MCP backlogit_sync_index alone does not. Operator remediation uses a fresh-process CLI sync.
- Engram exposes no payload-accepting MCP tool. The memory file was used as the file-based handoff.

## Failed or rejected approaches

- Option B (clear the marker on block and return, re-mark on unblock): not chosen.
- An atomic temp-then-rename writer for the UR3 marker, and a sentinel-file protocol (both rejected).
- A third plan re-review without new operator authorization (not attempted).
- Adding 181-F to 182-S (rejected, deadlock risk).

## Outcomes and open items

- 154-S remains queued with its hold label. It depends on 155-S (shipped) and on 182-S (new). Its banner records the attempt 6 FAIL and the option A decision.
- 182-S is queued and eligible under the normal Ship flow, with no waiver needed.
- Open: operator authorization for plan-review attempt 7 on 173-F; carried items AF1E5075 (label-aware claim refusal) and the 074-DL post-ship attestation.
- Untracked files left for the operator: checkpoint 033703, the 155-S reconcile files, and telemetry.jsonl.

## Sources

- docs/memory/2026-09-28/orchestrator-decisions-and-154-s-revalidation-memory.md
- docs/memory/2026-09-28/stage-173f-option-a-replan-memory.md
- docs/memory/2026-09-28/stage-c29ebee5-154s-revalidation-memory.md
- docs/memory/2026-09-28/stage-triage-513e62ab-fb0a850b-memory.md

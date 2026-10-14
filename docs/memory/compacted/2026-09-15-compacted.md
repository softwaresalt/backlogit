---
title: "Compacted memory: 2026-09-15 155-S current-HEAD P1 remediation and PR 444 follow-up"
date_range: "2026-09-15 to 2026-09-15"
doc_type: compacted-memory
source_count: 1
---

# Compacted memory: 2026-09-15

Compacts 1 source file (two remediation cycles in one record). The original was moved to docs/archive/memory/ (not deleted).

## Decisions made

- Five P1 findings from final review of 174-F / 155-S were resolved, scoped to Stage-owned planning and backlog artifacts. No source or test changes, no 154-S edit, no shipment claim, no PR.
- F1: canonical red-deliverable-contract blocks added to 174.039-T, 174.040-T, and 174.041-T. Selectors were validated with the wave-scheduler parser (3 of 3 clean).
- F2: a governed block commit must include the full output: the 154-S record, changed member artifacts, intent, preimage, snapshot recovery state, and per-item event logs.
- F3: 164.002-T retired. Its parked-era forward repair conflicts with exclusive activation under rev3. It is subsumed by R9 and R11.
- F4: removing 164.002-T decouples 146-S from 155-S, so no shipment-level blocks edge is needed.
- F5: 174.045-T was split into R7a (174.045-T, bypass write paths) and R7b (new 174.051-T, create-as-active refusal).
- 155-S manifest: 14 members, all queued.
- PR 444 follow-up (same date): unblock requires explicit --confirm for both blocked-to-queued and blocked-to-active. The block resume flag is --resume-checkpoint (mapped to BlockOptions.ResumeCheckpointRef). Stale --resume-ref wording was removed.

## Files modified or created

- .backlogit/queue/155-S.md, 164.002-T.md, 174.039-T.md, 174.040-T.md, 174.041-T.md, 174.045-T.md, 174.051-T.md
- .backlogit/queue/174.044-T.md, 174.046-T.md, 174.052-T.md, 174.053-T.md (PR 444 follow-up)
- docs/exec-plans, docs/product-specs, and docs/decisions: 2026-09-14-resumable-shipment-blocked-lifecycle-* files (plan, spec, and deliberation)

## Key learnings

- Validation for the first cycle: backlogit sync had 0 parse failures and indexed 1525 artifacts. doctor reported 23 pre-existing orphan findings, none on touched artifacts. wave-scheduler-sim -VerifyAgainstQueue passed 186 of 186. docs lint had no findings.
- Validation for the PR 444 follow-up: sync had 0 parse failures and indexed 1528 artifacts. wave-scheduler-sim passed 186 of 186 across 21 scenarios. Target-mode schema validation of the five edited files exited 0.
- Open residual classification: 146-S and 164-F have no live actionable task member. Retiring them entirely is a separate operator decision.

## Failed or rejected approaches

- A fixed claim-only unblock rule that did not cover blocked-to-active (replaced by the confirm-for-both rule).
- The stale --resume-ref flag (replaced by --resume-checkpoint).

## Outcomes and open items

- P0 = 0 and P1 = 0 in scope after both cycles.
- Open P2: whether to retire 146-S and 164-F. Flagged, not actioned, to avoid scope creep.
- Ship remains the owner of implementation (174.039-T and the following tasks).

## Sources

- docs/memory/2026-09-15/stage-155s-p1-remediation-cycle-memory.md

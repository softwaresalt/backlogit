---
title: "Compacted memory: 2026-09-16 trust-chain correction, 155-S hook trace repair, and non-member rollup elimination"
date_range: "2026-09-16 to 2026-09-16"
doc_type: compacted-memory
source_count: 4
---

# Compacted memory: 2026-09-16

Compacts 4 source files covering three threads: the 866FDC8C trust-chain decomposition correction (149-S, 150-S, 151-S), the 155-S hook-stream trace repair, and the 155-S rev6 non-member rollup elimination. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

Trust-chain correction (866FDC8C):
- Six deferred-scope stash entries were resolved (AC5346BC, 71F5C21F, 01D8515F, B9BA8751, 9BD58471, E8D4ED66). Work was amend-in-place, with no duplicate release units. Deliberation 066-DL linked the entries.
- The chain is serial: 149-S, then 150-S, then 151-S. A hard edge makes 151-S depend on 150-S, because the three shipments share reconcile and event surfaces. Downstream PRs must land as merge commits.
- Cycle-4 contract fixes (operator-authorized extra cycle): P-002.3 harness-exempt commands for five tasks, P-002.6 red-deliverable contracts for three tasks, and dependency-prose consistency.
- Final graph: 53 nodes, acyclic. Memberships: 149-S 20 items, 150-S 16, 151-S 17.

Hook trace repair (155-S, PR 445):
- No synthetic create_artifact rows were hand-appended and no retroactive creation was claimed.
- The gap was recorded with a comment. A truthful follow-up update event was emitted through a no-scope priority re-set.

Non-member rollup elimination (155-S rev6):
- Option B was chosen over re-locking the ancestor: eliminate the non-member status rollup entirely. This is authoritative flat-manifest rule SBLK-R28.

## Files modified or created

- .backlogit/queue/066-DL.md (deliberation); docs/exec-plans/2026-09-16-866fdc8c-decomposition-correction-plan.md (plan, with Plan Review cycles)
- New tasks: 168.011-T, 168.012-T, 168.013-T, 169.010-T, 170.011-T, 170.012-T; cycle-1 leaf splits 168.014-T to 168.018-T, 169.011-T, 169.012-T, 170.013-T, 170.014-T
- Withdrawn in a later cycle: 168.019-T, 170.015-T, 170.017-T
- .backlogit/hooks_queue.jsonl (repair events seq 2994 to 2996 for 174.059-T, 174.060-T, 174.063-T)
- New tasks for rev6: 174.061-T (RED harness) and 174.062-T (implementation). Coupled edits to 174.057-T, 174.058-T, and 174.059-T.
- docs/exec-plans and docs/product-specs for 2026-09-14 blocked lifecycle (plan section 0.4 rev6, spec SBLK-R28 and section 0.6), and the deliberation section 6j.

## Key learnings

- Cycle-0 plan review was BLOCKED by standard and adversarial review. Cycle 1 passed, with the mandatory Security persona covering confused-deputy, denial audit, nonce, algorithm pinning, not_before, and pin-set integrity.
- Hook stream: CreateArtifact normally emits a create_artifact event through the built-in post-hook. Three tasks had no create signal, so a follow-up event was the truthful repair.
- Lost-update defect: the non-member ancestor rollup in ShipShipment could overwrite a concurrent write. It was fixed by bounding cascade to explicit members rather than by adding a second lock.
- Validation (rev6): docs lint passed, sync indexed 1536 artifacts with 0 parse failures, doctor --target exited 0, and wave-scheduler simulation passed 164 of 164 and 186 of 186.
- Concurrency Reviewer PASS with high confidence, with no residual P0 or P1.

## Failed or rejected approaches

- Cycle-0 plan review: BLOCKED (superseded by cycle 1).
- Arithmetic error in cycle 1 ("5 resulting primary tasks"), corrected in cycle 2 to four primary tasks.
- Stash correct for plain-archived entries: inapplicable, since no harvested_artifact_id existed. Archive history was left untouched.
- Option A (protecting the ancestor with an extra lock or CAS): rejected in rev6.
- Synthetic create events: rejected.
- Trust-chain final review (third cycle) returned BLOCKED with two P1 findings. This was resolved in cycle 4.

## Outcomes and open items

- Trust-chain: operator approval required before any push or staging PR. Deferred items B633E9B9 and 6749D311 stayed open.
- 155-S: remains queued with the manifest unchanged after the hook repair. The GitHub thread was left unresolved and unreplied per instruction.
- rev6: 155-S has 25 members. Ship is to implement SBLK-R28 test-first, from RED 174.061-T to GREEN 174.062-T.

## Sources

- docs/memory/2026-09-16-stage-866fdc8c-decomposition-correction.md
- docs/memory/2026-09-16/orchestrator-trust-chain-stage-blocked-memory.md
- docs/memory/2026-09-16/stage-155s-hooks-trace-repair-memory.md
- docs/memory/2026-09-16/stage-155s-rev6-nonmember-rollup-elimination-memory.md

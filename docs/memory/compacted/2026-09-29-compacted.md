---
title: "Compacted memory: 2026-09-29 182-S UR3 flake closure, 173-F harvest, and plan review attempt 7"
date_range: "2026-09-29 to 2026-09-29"
doc_type: compacted-memory
source_count: 4
---

# Compacted memory: 2026-09-29

Compacts 4 source files covering the 182-S flake prerequisite and its post-merge closure, the attempt-7 plan review for 173-F, and the 173-F harvest after operator approval. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

UR3 flake prerequisite (182-S / 181-F):
- The deliberation chose option 2, reader-side tolerance of a partial crash-ready marker. The atomic temp-then-rename writer and sentinel-file protocol were rejected.
- The plan passed plan-review attempt 2 with all five personas. Parity was not triggered.
- Shipment 182-S was a single-member manifest, [181.001-T]. 181-F was deliberately not added.

Post-merge closure (182-S):
- PR 461 merged on 2026-09-29 with merge SHA 70d72044. The SHA is an ancestor of origin/main, and local main equals origin/main.
- shipment-reconcile ran pre, safe-close, and post under the 182-S lock, all passing. The shipment was shipped with archived_ids 181.001-T and 182-S, and no returns.
- The 181-F governed close step ran in this order: confirm shipped provenance from Markdown, record the commit (one commit_tracked event), move active to done (the hook rejects queued to done directly), then archive.
- Closure artifact: docs/closure/182-S-181-F-post-merge-closure.md (closure READY, compaction done). Runtime verification was N/A because the change was test-only.
- P-020 compact-context ran with target all. Eleven unreferenced 155-S Ship memories were compacted into docs/memory/compacted/2026-09-29-155s-ship-session-checkpoints-compacted.md, and their originals moved to docs/archive/memory/. Referenced memories were kept.

173-F plan review and harvest:
- Plan-review attempt 7 (decision ADVISORY, six personas). Every attempt-6 finding was confirmed resolved. Five merged P2s were remediated in the text.
- Size validation: the plan was decomposed. The bootstrap slice for 154-S became seven tasks in five waves (173.006-T, 173.007-T, new U0c, new U1b, 173.001-T, 173.003-T, 173.005-T). 173.002-T was retired into 173.007-T. 173.004-T was deferred to a follow-on feature with no new shipment.
- Operator gate (verbatim, 2026-09-29): "C29EBEE5 gate approved". This was recorded as operator_authorization approved, and the plan status moved from revised to approved.
- Harvest: created feature 182-F (option A regression net) with task 182.001-T (adopted from 173.004-T). Created 173.008-T (U0c) and 173.009-T (U1b). Added the edges 173.009-T to 173.008-T, 173.001-T to 173.009-T, and 173.001-T to 173.007-T. Set the 154-S manifest to 173-F, 173.006-T, 173.007-T, 173.008-T, 173.009-T, 173.001-T, 173.003-T, and 173.005-T.
- Archived C29EBEE5. Removed the do-not-claim-until-convergence label from 154-S. Kept bootstrap-exception, convergence-prerequisite, and bootstrap-bypass-approved-conditional.

## Files modified or created

- docs/closure/182-S-181-F-post-merge-closure.md (new closure record)
- docs/memory/compacted/2026-09-29-155s-ship-session-checkpoints-compacted.md (P-020 compaction output recorded in the 182-S closure; referenced, not written by this worker)
- docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md (cardinality guard, abandoned in R1, R3 transport side effects, mixed-binary caveat)
- docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md (attempt 7 review record; harvest checklist; size validation and decomposition)
- docs/exec-plans/2026-09-28-ur3-crash-ready-handoff-flake-plan.md and the flake deliberation
- Backlog: 181-F and 181.001-T; 182-S; 182-F and 182.001-T; 173.008-T and 173.009-T; 173.001-T, 173.003-T, 173.005-T, 173.006-T, 173.007-T rewritten; 173.002-T archived; 154-S manifest and banner; stash C29EBEE5 archived
- Test files: the UR3 harness (RED at af3d13ab, GREEN at eabfee9d)

## Key learnings

- The RED reproduced the flake signature: a 0-byte marker produced unexpected end of JSON input. GREEN passed with count 25 under -race, with 150 of 150 tests passing and no decode errors.
- Full governed go test -timeout=30m ./... passed for the flake change. golangci-lint reports 57 pre-existing findings on main, and --new-from-rev reported 0 for this change.
- A governed done transition archives the task record. Ship's governed close step must therefore confirm shipped provenance before archival.
- Plan-review ADVISORY with operator_authorization missing is not a passing gate. Stage must halt until the operator records approval in the final Plan Review section.
- Shipped provenance is archived with archived_status shipped, and a blocks edge alone releases on any terminal status. The 154-S banner therefore added a shipped-provenance guard for 182-S.

## Failed or rejected approaches

- The atomic temp-then-rename writer and the sentinel-file protocol for the UR3 flake (rejected).
- Adding 181-F to 182-S (rejected, because of condition b and the 154-S deadlock).
- Harvesting 173-F before operator approval (halted until approval arrived).
- Attempt 8 of the plan review (not proposed, because the remediation was text-only).

## Outcomes and open items

- 182-S: PR 461 merged. Ship stopped before merge approval for the closure PR, which was awaiting operator approval.
- 154-S: queued, with its hold label removed. It may be routed to Ship as an operator-supervised bootstrap under the conditional waiver, only after the staging PR merges. Nothing else was active.
- 182-F and 182.001-T: a later Stage session packages them after 154-S ships, under the 074-DL rule.
- Open operator items: the closure PR review, the staging PR merge, and carried items AF1E5075 and the 074-DL post-ship attestation.

## Sources

- docs/memory/2026-09-29/ship-182s-post-merge-closure-memory.md
- docs/memory/2026-09-29/ship-182s-ur3-crash-ready-flake-memory.md
- docs/memory/2026-09-29/stage-173f-harvest-memory.md
- docs/memory/2026-09-29/stage-173f-plan-review-attempt-7-memory.md

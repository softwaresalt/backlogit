---
title: "Compacted memory: 2026-09-26 shipment eligibility assessment and CB8887AF E1 to E5 ensemble harvest"
date_range: "2026-09-26 to 2026-09-26"
doc_type: compacted-memory
source_count: 2
---

# Compacted memory: 2026-09-26

Compacts 2 source files. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

Eligibility assessment (read-only; Orchestrator):
- 153-S and 141-S were corrected to not eligible. The shipment-claim convergence rule (condition b) forbids claiming unrelated multi-member shipments before the 154-S scheduler marker exists and is consumed. 152-S and 147-S were conditional. None was routed.
- 152-S was conditional on five open pre-claim plan corrections: AF6BFAC8, 4898A600, 33241CC0, D9E4EC6C, 97AB4978.
- 147-S was conditional: member 165.002-T was blocked on an upstream harness change.
- 154-S was held by the bootstrap-bypass-unapproved label, which needed an operator waiver and C29EBEE5 re-validation.
- Checkpoint 005049 (Stage-owned, E1 to E5) was valid. Stage needed operator confirmation before restoring it.

CB8887AF harvest (Stage, branch stage/cb8887af-test-budget-ensembles from main at c3f5e2ce):
- Five features and five shipments were harvested, with 22 tasks in total: 176-F / 177-S (E1 CI and release, 3 tasks); 177-F / 178-S (E2 local gates, 4); 178-F / 179-S (E5 render inputs, 4); 179-F / 180-S (E3 procedures, 5); 180-F / 181-S (E4 guidance, 6).
- Shipment edges: 177-S, 178-S, and 179-S blocks 155-S (provenance only, since 155-S had shipped). 180-S and 181-S blocks on 179-S.
- Constitution bump: task 180.002-T became a MINOR bump, 1.0.0 to 1.1.0, after PR 452 review. The earlier PATCH plan was superseded.
- Provenance: backlogit_harvest_stash was not used, because it maps one entry to one item and consumes it. Instead every feature and task cited CB8887AF, decision 073-DL O12, and plan sections 19R.10 and 19R.11. CB8887AF was annotated and then archived.
- Priorities stayed at the default (medium), and no shipment priority was set, to avoid reordering other queued work.

## Files modified or created

- Backlog: 176-F, 177-F, 178-F, 179-F, 180-F; tasks 176.001-T to 176.003-T, 177.001-T to 177.004-T, 178.001-T to 178.004-T, 179.001-T to 179.005-T, 180.001-T to 180.006-T; shipments 177-S to 181-S.
- Serialization edges (ordering only): 176.003-T after 176.002-T (ci.yml); 179.004-T after 179.002-T (build-feature/SKILL.md); 178.004-T after 178.002-T (manifest).
- Full ensemble-green checks sit on 178.004-T, 179.005-T, and 180.006-T.
- Stash CB8887AF annotated and archived. Still active: 95DF7CE9, 5F1A1873, 5A1C4D3F, D8EF5443, 7CE12EE6.
- Source record: docs/memory/2026-09-26/stage-cb8887af-e1-e5-harvest.md and the orchestrator eligibility record.

## Key learnings

- A pipeline-topology pre-claim gate existed for multi-member shipments. The eligibility assessment missed this because the rule was prose-only and the queue did not read it. It was corrected on 2026-09-27.
- Two parallel create_item calls raced the ID allocator. The second produced a self-edge (179.004-T to 179.004-T), which was compensated. After that, all creations ran sequentially.
- Doctor (read-only, no fix_orphans) reported 23 orphaned-artifact findings, all pre-existing (016.001-R and 106.012-T to 106.033-T). No duplicate IDs. Sync indexed 1766 artifacts.
- A bare go test ./... run enforces Go's default per-package 10-minute timeout. An explicit -timeout=30m loosens that gate, so the constitution change had to name the enforcement change.
- The P-011 clean-main gate halted on non-continuity edits (config.yaml, _orchestrator.agent.md, _stage.agent.md) that the operator had to classify.

## Failed or rejected approaches

- Treating the 153-S and 141-S Ready verdicts as correct (rejected by the 2026-09-27 correction).
- Using backlogit_harvest_stash for a stash entry that feeds five features (rejected).
- The PATCH version bump for the constitution (superseded by MINOR).
- Routing anything behind 154-S before it passes both P and C checks.

## Outcomes and open items

- Stage checkpoint 005049 resumed and harvested CB8887AF. Stage handed off to Ship, with no claim and no build.
- Ship may take 177-S, 178-S, and 179-S in any order once the staging PR merges. 180-S and 181-S become eligible after 179-S ships.
- Orchestrator: commit its own routing and memory files separately, then run the Step 1.5 staging PR.
- Open: the non-continuity dirty files on main, and the 154-S hold (C29EBEE5 re-validation pending).

## Sources

- docs/memory/2026-09-26/orchestrator-eligibility-assessment-memory.md
- docs/memory/2026-09-26/stage-cb8887af-e1-e5-harvest.md

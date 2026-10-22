---
schema_version: "1.0"
doc_type: memory
title: "Compacted memory: 153-S and corrective staging sessions (2026-10-07 to 2026-10-08)"
description: "Dense extract of seven Orchestrator, Ship, and Stage notes for the 153-S claim halt, build and review, and the corrective 197-S to 201-S staging; originals archived."
chunk_strategy: h1-h2-h3
---

# Compacted memory: 153-S and corrective staging sessions

P-020 compaction (2026-10-22). Originals are preserved under `docs/archive/memory/` at the same date-relative path (listed at the end).

## 153-S claim contention halt and correction (2026-10-07)

* Orchestrator dark run for scope `153-S` (feature 172-F, 16 members) halted with `DARK_MODE_HALTED`. Before the halt: the operator attested the 154-S closure condition (b) (`CONDITION_B_ATTESTED`, PR #484, merge `0201e770`, three Copilot rounds); the `pre_claim` topology gate passed; the served-root handoff passed (workspace `.`, storage `.backlogit`, catalog and `pragma_database_list` attested, manifest matched MCP); Ship created `feat/153-s-concurrency-safety-hardening-for-artifact-mutation-writers` at `0201e770`.
* Halt chain: MCP `backlogit_claim_shipment(153-S)` timed out; a single bounded CLI re-claim at 07:23:33Z failed with `lock shipment lifecycle recovery ... shipment-lifecycle-global: gate in progress for item`; the post-claim gate returned `CLAIM_NOT_OBSERVED` (exit 3); later shipment reads also failed on the same lock. Bounded claim convergence required a terminal halt (no further claim attempt, no gate bypass).
* Correction (2026-10-07T20:14Z): the original multi-server contention diagnosis was wrong. Process working directories showed only PID 37184 serves this workspace. The timed-out MCP claim kept running and succeeded (`153-S.jsonl` `shipment_status_changed` 07:34:13Z; members active through 07:36:11Z). Root cause: Ship's CLI re-claim collided with the session's own in-flight MCP claim on the global lifecycle lock; the post-claim gate sampled before the claim committed.
* Resume: do not re-claim. A resumed Ship observed 153-S and all 16 members `active` (`scheduler_baseline_claim: 153-S`); `pipeline-topology --phase post_claim` exit 0 (`CLAIM_VERIFY_OK`, `BRANCH_OK`, `WORKTREE_TOPOLOGY_OK`); `SERVED_ROOT_ATTESTATION_OK`; index sync 1918 artifacts. Claim state is committed on the feat branch.
* Learning / follow-up: after an MCP client timeout the server-side operation may still complete; retrying races it. Cancel the server-side operation on timeout or make the lifecycle-gate error identify the in-flight operation, and make claim convergence poll for a delayed result before declaring `CLAIM_NOT_OBSERVED`.

## 153-S build and review (2026-10-07 to 2026-10-08)

* All 15 tasks (172.001-T to 172.015-T) implemented across 7 waves and `done`. Harness order: source-shape AST RED harness (172.014-T), declarations (172.011-T, 172.013-T), behavior harnesses, then GREEN implementation. Base `0201e770`.
* Review cycle 1 (persona): `BLOCKED`; findings A-J fixed in `5feb4248`, `dafc93b2`, `1b17b672`, `ce868001`. Cycle 2 (three-reviewer adversarial): `READY_WITH_FOLLOWUPS`, F1-F6; in-scope F1 (production-unreachability AST guard for the item-log barrier setter), F2 (lock-order test cleanup drains archive goroutine), F4 (CAS unarchive of a vanished item returns `ErrShipmentConflict`), F6 (hook-placement comment), F3/F5 documentation corrections fixed in `c5e3a1df`, `e6c963a2`; out-of-scope F3 code `E4908F30` and F5 ordering `BACD94FC` captured.
* Deferred stash entries: `A0C92478` (`BulkUpdateConflict.Err` serializes to `{}`), `E4908F30` (ShipShipment holds global and B locks across item-log C during archival), `BACD94FC` (deferred archive event ordering), `517BAC76` (C-before-B lock hierarchy redesign), `DBF89C9C` (post-archive hook/stash side effects unlocked), `36D86223` (core test runtime near the CI race timeout), `92AB6879` (`TestClientLatestRespectsContextTimeout` flake); `A0C733C6` (findArtifact TOCTOU) reused.
* Decisions: archive events from standalone and cascade `ArchiveItem` go to a deferred sink drained after B is released; reconcile and unarchive use CAS on expected status and surface `ErrShipmentConflict`; ShipShipment's governed path was left unchanged and recorded as a residual in `docs/design-docs/artifact-mutation-lock-order.md`.
* Copilot round 2 (review on `c89227e9`): Reconcile Step 12 re-archive returned nil on concurrent deletion; fix translates not-found into `ErrShipmentConflict` (wrapping `ErrNotFound`) only when an expected-status CAS is set; RED-first test `TestU172_ReconcileArchivedLifecycleReArchiveStepDetectsConcurrentDeletion`.
* Race watchdog flake: `TestU172_Race_ReconcileArchivedLifecycleBatchAndSameItemWriterConverge` hit the 10s watchdog under load (goroutines finished after cleanup, `database is closed`); isolated 3/3 pass. Fix (P-021 C1, own tests): `u172RaceDeadline = 45s` for the three U172 race tests; real deadlocks never complete, so detection is preserved.
* Merge and closure: round 3 two threads fixed in `b7a06d53`, track-commit `1655bf2f`, round 4 clean; `autoharness gate copilot-review 485` SATISFIED, CI green; PR #485 merged by merge commit `271c916e` (07:05:39Z); `POST_MERGE_SYNC_OK`. Closure branch `post-merge/153-s-concurrency-safety-hardening`; pre-reconcile PROCEED; MCP `ship_shipment` timed out client-side and was not retried (shipped 07:14:45Z, archived 07:30:33Z); P-007 clean; archive commit `c5d6bb06`; closure `docs/closure/153-S-172-F-concurrency-safety-hardening-post-merge-closure.md` (READY, compaction degraded); compound-refresh kept all four candidates.

## Corrective staging 197-S to 201-S (2026-10-08)

* Stage (gpt-6.1-sol, `ROUTING_DEGRADED`) created shipments 197-S to 201-S and 61 tasks; staging PR #487 merged as `98950cf5` (CI green, no threads); main fast-forwarded and equal to origin.
* Approved plans (four 2026-10-08 plans, ADVISORY reviews with operator authorization, "FOLLOW THE APPROVED PLANS UNCHANGED") were the decomposition source. Registry harvest model is feature -> task, no sub-epics.

| Wave | Feature | Tasks | Shipment | Members | Queue pos |
|---|---|---|---|---|---|
| CX closure protocol and gate correctness | 197-F (reused) | 197.001-T..197.019-T (U1, U1b, U2-U15, U16a, U16b, U17) | 197-S | 20 | 100 |
| CT test-suite health and diagnostics | 198-F | 198.001-T..198.005-T (T1-T5; T5 depends on T1-T3) | 198-S | 6 | 150 |
| CY-A | 199-F | 199.001-T..199.009-T | 199-S | 10 | 450 |
| CY-B | 200-F | 200.001-T..200.013-T | 200-S | 14 | 1400 |
| CY-C | 201-F | 201.001-T..201.010-T | 201-S | 11 | 1500 |

* Fold-ins: `97AB4978` -> 171.007-T/171.008-T into 152-S (9 members); `E52607F5` -> 189.006-T/189.007-T into 189-S (8); `95379DBA` -> 183.004-T into 183-S (5), review edge 183.002-T -> 183.004-T.
* Shipment edges (dependent blocks on prerequisite): 152-S->197-S, 141-S->197-S, 152-S->198-S, 199-S->197-S, 200-S->199-S, 201-S->200-S, 189-S->199-S. Context link 072-DL informs 198-F; 185-S informs 156-S.
* Constraints: U7 (197.008-T) is additive 140-S closure registration only (do not rename or edit narrative closure files); U15 requires U6 CONFIRMED; 152-S blocks on both CX and CT; CT T5 verification runs after CX merges; 156-S stays queued, empty manifest, DO-NOT-CLAIM until 185-S ships a governed abandon path.
* 34 stash entries archived with provenance (10 CX, 3 CT, 3 CY-A, 6 CY-B, 4 CY-C, 3 fold-ins plus scope correction `8CCB29CF` folded into `DE3E7B67`; duplicates `8B52A5F1`, `E45E6D65` supersedes `37699341`, `92F79833`). Rejected fold-ins `7AA35A39`, `9CA03F5D`, `F88FE051`, `6EB55AE6`, `E6EE8944` stay active; `DB48A817`, `FF1F3AC7`, `885263D2` active pending CT green verification.
* Decisions: pre-claim rule for `198-S blocks-on 197-S` is effective shipped provenance from canonical Markdown (live `shipped`, or `archived` with `archived_status: shipped`), because `ShipShipment` archives (corrected `13e9ef67`); keep it manual until queued feature 184-F ships a native guard. Stash `9F67362D` (native shipped-only guard) was captured then retracted as a duplicate of 184-F; lesson: search queue and stash for existing coverage before capturing. "Decisions 1-3 approved" was treated as authorization for the staging merge. The PR #489 review-fix limit (3) was extended once by explicit operator re-instruction.
* Config: one invalid `context_tier: "default"` under `model_routing.alt_doc_review` was removed locally (backup `logs/config.yaml.operator-edit-2026-10-08.bak`); the operator's routing edit (`89990d65`) was restored out of the docs-only PR by `41e182f0` and needs its own reviewed PR. Later fresh validation against the autoharness `harness-config.schema.json` failed (`context_tier` additional property); Stage stayed fail-closed (config untouched, no fallback route) until the config passed. The full test run failed only on the `TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters` lock-contention flake, which justified 198-S.
* Stage blocked-continuation state: checkpoint `checkpoint-20261009-005935.json` was operator-selected for CX criteria repair (19 sections, audit 42 more); HEAD `d2f3ff97` on main; nothing committed or resolved at the time.
* Open items: stale Ship checkpoint `checkpoint-20261008-073934.json` (153-S) can be resolved by Ship; archived 057/121/122/123-S carry `status: done` and need a Ship `reconcile-shipped` dry-run; Ship routing blocked by dirty `.autoharness/config.yaml` (commit via its own PR or stash); committed Ship route is gpt-6-luna/openai/xhigh vs operator's uncommitted claude-haiku-5.5/anthropic edit (confirm); 194-F `context_tier` edits conflict with the installed schema.
* Learnings: queue-position edits to shipment frontmatter need an index sync before queue reads; a native dependency-add timeout was verified unapplied before a longer bounded retry; Stage checkpoints sequence (harvest/assembly/stash-archival) ended at `checkpoint-20261009-004513.json`.

## Archived originals

* `docs/memory/2026-10-07/orchestrator-153-s-dark-run-claim-contention-halt-memory.md` -> `docs/archive/memory/2026-10-07/orchestrator-153-s-dark-run-claim-contention-halt-memory.md`
* `docs/memory/2026-10-07/ship-153-s-claim-verification-halt-memory.md` -> `docs/archive/memory/2026-10-07/ship-153-s-claim-verification-halt-memory.md`
* `docs/memory/2026-10-07/ship-153-s-served-root-handoff-memory.md` -> `docs/archive/memory/2026-10-07/ship-153-s-served-root-handoff-memory.md`
* `docs/memory/2026-10-08/orchestrator-corrective-staging-memory.md` -> `docs/archive/memory/2026-10-08/orchestrator-corrective-staging-memory.md`
* `docs/memory/2026-10-08/ship-153-s-build-review-memory.md` -> `docs/archive/memory/2026-10-08/ship-153-s-build-review-memory.md`
* `docs/memory/2026-10-08/stage-corrective-config-validation-blocked-memory.md` -> `docs/archive/memory/2026-10-08/stage-corrective-config-validation-blocked-memory.md`
* `docs/memory/2026-10-08/stage-corrective-harvest-memory.md` -> `docs/archive/memory/2026-10-08/stage-corrective-harvest-memory.md`

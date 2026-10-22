---
schema_version: "1.0"
doc_type: memory
title: "Compacted memory: 196-S Stage and Ship sessions (2026-10-05 to 2026-10-06)"
description: "Dense extract of seventeen Stage and Ship session notes for shipment 196-S (Amendment 3, Erratum E1, build waves, review); originals archived."
chunk_strategy: h1-h2-h3
---

# Compacted memory: 196-S Stage and Ship sessions

P-020 compaction (2026-10-22). Originals are preserved under `docs/archive/memory/` at the same date-relative path (listed at the end). `ship-196-S-merge-and-closure.md` was skipped because `docs/memory/compacted/2026-10-06-196-S-compacted.md` still names it, so it stays in place.

## Shared context

* Shipment `196-S` (feature `196-F`, nine tasks 196.001-T to 196.009-T), P-017 dark mode, scope `[196-S]`, operator AFK, `merge_approval_pre_authorized=true`, `admin_fallback_pre_authorized=false`. Both agents ran `ROUTING_DEGRADED` and `ESCALATION_DEGRADED` (same-route Tier 3, no engram intake), so halts went to the operator.
* Governing plan: `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md` (Amendment 3, then Erratum E1).

## Stage: Amendment 3 plan review (2026-10-05)

* Amendment 3 (appended to the plan) holds: A3.1 per-finding P-021 C1 table, A3.2 exact Go assertion specs with expected red strings, A3.3 TDD conditions, A3.4 governed-reopen options A/B/C, A3.5 ordered Ship phases, A3.6 records/telemetry/risks. The Objective was rewritten so P-001 no longer "blocks the next autonomous pipeline cycle".
* Governed reopen was blocked: the lifecycle allows only `done -> archived` (`internal/config/defaults.go`, `internal/hooks/builtin_pre.go`), so no governed operation reopens a `done` task. Operator ruling A (recommended, operator-executed only) was adopted, R02 ratified.
* R02: single-key hand edit of `resume_checkpoint_ref` to `checkpoint-20261005-052350.json` (no governed setter; `member_status_snapshot` left stale). MCP `backlogit_sync_index` missed the hand edit; CLI `backlogit sync` picked it up (CLI v1.11.0 `131577c` vs MCP `7c805f9`).
* Finding dispositions: R01 resolved (PR #472 `1ccecd94`); R04/R07 (U8 harness 196.008-T), R06 (U1 196.001-T), R08 (U3 196.003-T) fixed as same-surface; R03/R10 `CDBCB258`, R05 `F6F3AA0E`, R09 `147BD825`, R12 `3B25D37F` deferred as P-021 C2; R11 dropped (already pinned by `TestUCS1_ClaimStartContract`).
* Review cycles: 1-3 FAIL (cap), cycle 4 FAIL (operator exception), cycle 5 PASS (operator exception, Resolution 3, `8db905ae`). Cycle-4 P1s: tracked runtime logs (`hooks_queue.jsonl`, telemetry) dirty the tree and break Option A step 8; porcelain-shape assumption; Ship Session End writes after the G-A3 commit; no Orchestrator committer. Cycle 5 verified all four remediations after a second targeted pass (P2/P3 fixed in text, not re-dispatched).
* Mechanism decisions: Option A step 8 is `git add -A` then an exact cached name-status set (two `R` rows archive to queue plus `M` rows in `RUNTIME_LOGS`; `hooks_queue.jsonl` append-only; `git reset -q` on halt). The scratch rehearsal was dropped (code evidence, residual in A3.6). The Orchestrator commits nothing on the 196-S branch; the receiving party carries Orchestrator notes and `RUNTIME_LOGS`. G-A3 item 13: events recorded before the final commit, Session End steps 3-4 deferred, the G-A3 checkpoint is the single final one, no backlogit call follows the commit.
* Spike `002-SP` harvested from stash `DB071B5D` (queued, outside any shipment). Stash `69B0B3F0` has no harvested item, so it is referenced in text instead of linked.
* Ordered Ship phases: Phase 0 restore checkpoint and resume 008 at Step 4.4 (no second `WORK_STARTED`); Phase 1 U8 (harness `^TestUSR8_`, red, `test(harness)` commit, red-path done, wave-1 Step 4.6); gate G-A3 halt unless ruling B/C or A with committed record; operator runs Option A steps 1-10 and commits the ruling file (`A3_RESUME_CHECKPOINT`, `A3_R02_RATIFIED`); Phase 0' restore and route (A to Phase 2, B halt for Amendment 4, C to Phase 3); Phase 2 (001 and 003, one harness-architect call, per-owner `test(harness)` commits); Phase 3 waves 3 {002,005}, 4 {009}, 5 {006}. Closing-wave drift under A (declared 2/2/3, effective 3/3/4) is accepted and recorded.
* Local state: `checkpoint-20261005-052350.json` was added to the 196-S continuity block in `.git/info/exclude` (never committed).

## Stage: Erratum E1 (2026-10-06)

* Ship halted at Phase 2 wave 2 (checkpoint `checkpoint-20261006-073956.json`); Stage added Erratum E1 to the plan on Ship's branch.
* E1.1: A3.3 item 4 ordered Step 4.2 dispatch before the traceability commit, contradicting clean-tree/baseline/zero-delta order; corrected (Step 4.1b/4.1c and Step 4.2 pre-dispatch actions first; dispatch only after `red_baseline_sha`; precedent `f18fcb11`). The lifecycle `TOPOLOGY_GATE` is a pre-dispatch action "when installed".
* E1.2: item 2 "no green yet" evidence for U1/U3 was not recorded in time; re-run against pinned SHAs (`origin/main` `c4c458b9`; parents `e7454f15`, `d2248d7c`) was empty and accepted as equivalent with a forward rule.
* E1 effective by operator approval (reply 2026-10-06T12:31-07:00 about the clean worktree; interpreted by the Orchestrator, so approval is inferred, a residual risk). Cycle-6 review was not run.
* E1.4 (operator instruction): same order for every task (pre-dispatch actions, clean tree by commit or stash, empty porcelain, fresh `red_baseline_sha`, dispatch); stash-to-clean is legal (`push --include-untracked` with a 196-S message, SHA/paths recorded, no drop/clear/`reset --hard`/clean except a post-verified-apply drop); plan-required content must be committed (stashing it is a halt); no restore during the build window; no 196-S stash left at PR time.

## Ship: resume and build (2026-10-06)

* Resume (E1.3): the Orchestrator selected `checkpoint-20261006-073956.json` (81 Ship checkpoints, 0 anomalies); preconditions passed (E1 effective, scope, clean tree, `d9e00a6d` ancestor, `hooks.yaml` absent, R14 served-root attestation including a single `main` pragma row). The checkpoint was resolved after resume; head `9e023324`, resume commit `c7b7c57f`. All red deliverables used E1.4 with no stash.

| Task | Result | Key commits / evidence | Deferrals |
|---|---|---|---|
| 196.001-T (U1, red) | RED `^TestUSR1_` (5 A3.2.1 messages), READY P0/P1/P2=0, P3=15 | baseline `c7b7c57f`, harness `d2248d7c`; green-maker 196.002-T | `180AA2C2` |
| 196.003-T (U3, red) | RED `^TestUSR3_`, READY_WITH_FOLLOWUPS P2=1, P3=13 | baseline `24469868`, harness `e36ac003`; green-maker 196.005-T | `2D682258` (discharged when 005 removed the Step 6 item a clause) |
| 196.002-T (U2) | exempt `EXEMPT_VERIFY_OK`; USR1 green; READY_WITH_FOLLOWUPS P2=1, P3=6 | `cdda97f9`, `231d008d`, baseline `72a074f1`; `_orchestrator.agent.md` | `CB0F604E` (R8 `docs/memory/` record vs P-011 clean-tree gate, fails closed) |
| 196.005-T (U5) | exempt OK; USR3/USR4 green; P2=1, P3=6 | `d64afe6a`, `f39fd009`, baseline `a1effc4a`; shipment-reconcile SKILL plus `_ship.agent.md` Step 6 item a | reuse `52D18E44`, new `6387A6A2` (also U2 P3s) |
| 196.009-T (U9) | exempt OK; USR8 4/4; P2=1, P3=6 | `0cdb4f0a`, `3a8a7744`, baseline `6539216f`; `_ship.agent.md` | `7C9340AC` |
| 196.006-T (U6, harness) | `TestUSR6_HarnessManifestDriftRecords` red 3/3 then PASS; P3=2 | harness `abcffb63`, label `865d3b7c`, deliverable `091a074e`, `b2768e69`; manifest checksums for orchestrator/SKILL/ship match recipe | `0F428816` |

* AC4 dry-run (196.002-T): read-only served-root checks passed for the active 196-S (queue path) and archived 195-S; a candidate root one level up fails step b with expected `SERVED_ROOTS_UNRESOLVED`.
* Wave-2 Step 4.6 (HEAD `e64d07d9`): P-005 closing-wave drift emitted before the gate; compile, vet, golangci-lint v1.64.8 (`GOTOOLCHAIN=local --concurrency=1`), LF-gofmt (880 blobs) passed; open-red selectors `^TestUSR1_`, `^TestUSR3_`, `^TestUSR8_` each observed assertion-RED; `FULL_SUITE_DEFERRED: wave 2`; budget not exceeded.
* Wave 3: 002 and 005 done; USR1/USR3 newly closed GREEN; open red `^TestUSR8_` (green-maker 009, declared 3 / effective 4); full suite deferred again.
* Wave 4: 009 done; USR8 GREEN; open-red set empty; unfiltered `go test -timeout=30m ./...` exit 0 (40 packages); `WAVE_OPEN_RED_UNCLOSED` not triggered.
* Wave 5: 006 done (`b2768e69`, bookkeeping `423ae694`); census 9/9 done; `go build ./cmd/backlogit` exit 0; full suite exit 0 at `423ae694`.

## Ship: pre-PR adversarial review and PR

* Report-only review at `423ae694` vs `origin/main` `c4c458b9`: Anchor (gpt-6-sol, high) 2xP1, Correctness (claude-opus-5) P2=5/P3=6, Go Reviewer none, Coupling (claude-haiku-4.5) 1xP0/2xP1/2xP2/1xP3. Final triage P0=P1=0, `READY_WITH_FOLLOWUPS`; no code fix needed (1 of 3 cycles).
* Triage: anchor P1 "Safe-Close rejects active" was pre-existing and plan-verbatim, so deferred (reuse `6387A6A2`); anchor P1 "E1 approval inferred" kept as governance residual (Ship cannot edit plans, P-010). All coupling claims refuted with code (SKILL mandates the under-lock re-check; `pragma_database_list` valid and exercised by `TestUSR7_`; `--cwd` is a persistent flag; frontmatter read is not a checkpoint dump; checksum CI pin belongs to autoharness). Correctness P2 on gate-disabled U4 refuted (`validateMemberGateEvidence` skips non-task members); U6 checksum currency verified.
* New deferrals: `5B2F5EC1` attestation trust wording, `4E0F44CE` pre-archived `archived_status`, `5E3FBAC7` non-shipment recovery disposition, `CC86D2E1` U4/U8 harness hardening; reused `7C9340AC`, `6387A6A2`, `52D18E44`.
* Step 5 summary: tasks 001/003/002/005/009/006 done this session (004/007/008 earlier); next push, PR with Local Review Readiness, Copilot loop, merge commit, main sync, closure PR. Decision: plan-verbatim defects are deferred under P-021 C1, never edited.

## Learnings

* Resuming from a checkpoint after an E1-type ordering error: fix the order in an erratum rather than patching evidence; record late evidence by deterministic recomputation against pinned SHAs.
* Clean-tree-before-baseline is the invariant for red deliverables; stash-to-clean is acceptable only for non-plan content.
* Plan-review cycle caps need explicit operator authorization per exception cycle; each exception should be recorded in the plan.
* Hand edits to Markdown need CLI `backlogit sync`; the MCP sync may miss them.

## Archived originals

* `docs/memory/2026-10-05/stage-196-S-amendment3-cycle4-review.md` -> `docs/archive/memory/2026-10-05/stage-196-S-amendment3-cycle4-review.md`
* `docs/memory/2026-10-05/stage-196-S-amendment3-cycle5-review.md` -> `docs/archive/memory/2026-10-05/stage-196-S-amendment3-cycle5-review.md`
* `docs/memory/2026-10-05/stage-196-S-amendment3-reopen-blocker.md` -> `docs/archive/memory/2026-10-05/stage-196-S-amendment3-reopen-blocker.md`
* `docs/memory/2026-10-06/ship-196-S-001-complete.md` -> `docs/archive/memory/2026-10-06/ship-196-S-001-complete.md`
* `docs/memory/2026-10-06/ship-196-S-002-complete.md` -> `docs/archive/memory/2026-10-06/ship-196-S-002-complete.md`
* `docs/memory/2026-10-06/ship-196-S-003-complete.md` -> `docs/archive/memory/2026-10-06/ship-196-S-003-complete.md`
* `docs/memory/2026-10-06/ship-196-S-005-complete.md` -> `docs/archive/memory/2026-10-06/ship-196-S-005-complete.md`
* `docs/memory/2026-10-06/ship-196-S-006-complete.md` -> `docs/archive/memory/2026-10-06/ship-196-S-006-complete.md`
* `docs/memory/2026-10-06/ship-196-S-009-complete.md` -> `docs/archive/memory/2026-10-06/ship-196-S-009-complete.md`
* `docs/memory/2026-10-06/ship-196-S-adversarial-review.md` -> `docs/archive/memory/2026-10-06/ship-196-S-adversarial-review.md`
* `docs/memory/2026-10-06/ship-196-S-e1-resume.md` -> `docs/archive/memory/2026-10-06/ship-196-S-e1-resume.md`
* `docs/memory/2026-10-06/ship-196-S-step5-summary.md` -> `docs/archive/memory/2026-10-06/ship-196-S-step5-summary.md`
* `docs/memory/2026-10-06/ship-196-S-wave2-step46.md` -> `docs/archive/memory/2026-10-06/ship-196-S-wave2-step46.md`
* `docs/memory/2026-10-06/ship-196-S-wave3-step46.md` -> `docs/archive/memory/2026-10-06/ship-196-S-wave3-step46.md`
* `docs/memory/2026-10-06/ship-196-S-wave4-step46.md` -> `docs/archive/memory/2026-10-06/ship-196-S-wave4-step46.md`
* `docs/memory/2026-10-06/ship-196-S-wave5-step46.md` -> `docs/archive/memory/2026-10-06/ship-196-S-wave5-step46.md`
* `docs/memory/2026-10-06/stage-196-S-a3-erratum-e1.md` -> `docs/archive/memory/2026-10-06/stage-196-S-a3-erratum-e1.md`

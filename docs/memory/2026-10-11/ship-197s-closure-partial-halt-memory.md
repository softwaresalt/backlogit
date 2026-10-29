---
schema_version: "1.0"
doc_type: memory
title: Ship 197-S DARK_MODE_HALTED at post-merge closure: governed safe-close timed out after partial mutation; operator reconciliation required
description: PR #491 for 197-S merged at f8d35936 with CI and Copilot P-018 gates passing. The post-merge closure halted in backlogit_ship_shipment after an MCP timeout left the shipment control record shipped but unarchived, with a held global shipment lifecycle lock. Ship did not retry, delete locks, or commit the partial mutations. Resume steps recorded.
timestamp: "2026-10-11T01:35:00Z"
---

# Ship 197-S: DARK_MODE_HALTED at post-merge closure (partial governed mutation)

## Status

- **Feature delivery: complete.** PR #491 merged as a merge commit at `f8d35936deae71a468ccba6d769a3dfa2bdb3e20`.
- **Release closure: INCOMPLETE** (P-001). Shipment `197-S` remains awaiting required post-merge closure, so the release unit is still active.
- Halt class: `DARK_MODE_HALTED` (P-017 dark-mode halt on an ambiguous governed-mutation state and an unavailable required tool with no fallback). No admin fallback was used.
- The operator's 10000-AIC rule was respected: the shipment was not stopped before PR or merge. Closure stopped on the safety gate described below.

## Completed (Ship, this invocation)

| Step | Result |
|---|---|
| Startup and attestation | Index sync OK. Served roots re-attested (`C:\Source\GitHub\backlogit`, storage `.backlogit`, single `main` row). Post-claim topology gate exit 0. M = 18 tasks all `done`, 197.016-T `archived`. |
| Pending docs | Plan Erratum E3, Stage note, and Orchestrator decisions committed `46d6eaad` (explicit paths). Markdownlint 0 issues. |
| Drift records (AC3) | `_ship.agent.md` and `shipment-reconcile/SKILL.md` checksums refreshed `43c2bbea`. `TestUCXS17_RefreshedDriftRecords` and `TestUSR6_HarnessManifestDriftRecords` PASS. |
| Captures (P-021 C2, discovery first) | New: `B2B4BE2A` (legacy lock sidecars), `0D1B5D5C` (unchecked stderr write), `8E484B5D` (human-thread resolution wording), `40F841E6` (stale section pointers), `DBD92867` (shipment-ID grammar). Reused by discovery: `2C8615A5` (P1-4 side-effect paths), `63233C1C` (P-007), `405C8574` (vacuous assert), `8F45E676` (140-S gate keys), `2F152179` (WAVE_SNAPSHOT). |
| Confirming review at `af46609f` | Adversarial (anchor `gpt-6.1-sol` high, dispatched; runtime identity unexposed) and review-skill pass (`claude-sonnet-5.5`). **0 P0, 0 P1**, P2 4, P3 6. No fix cycle used. Follow-ups: stash `3386D498`, `FECEA4B0`, `3F058389`, `0365F693`, `5EE1CABB`. |
| Integration contract package | `go test ./tests/integration/` exit 0 at `b0f611ea`. |
| Unfiltered full suite at final HEAD `b0f611ea` | exit 0. 40 packages ok, 1 without tests. Log `logs/full-suite-197s-b0f611ea.txt` (gitignored). |
| Local build | `go build ./cmd/backlogit` exit 0. |
| Lock runtime round-trip (PowerShell) | Pass: sidecar name `.<file>.agent-lock`, wrong-token refusal, correct release, `-Force` release, out-of-root refusal. Bash round-trip not run on this host. |
| Feature PR | #491 opened with `## Local Review Readiness` (reviewed HEAD `b0f611ea`, `READY_WITH_FOLLOWUPS`, P0/P1 0). |
| Copilot P-018 | Requested. Reviewed `b0f611ea`, 0 inline comments, 0 threads. Gate verdict `SATISFIED` (pass 1 and last-mile). |
| CI | 7 of 7 required checks pass at `b0f611ea`. |
| Merge | Merge commit `f8d35936`. Preconditions: `merge_approval_pre_authorized` true, P-009 merge-only (squash and rebase disabled), last-mile head match. Confirmed `MERGED`, and `f8d35936` is an ancestor of `origin/main`. |
| Post-merge main sync | `POST_MERGE_SYNC_OK: main == origin/main @ f8d35936`. Worktree clean. |
| Closure branch | `post-merge/197-s-ship-closure-protocol` created from synced `main`. Lifecycle topology gate (a0) exit 0. Closure baseline clean. |
| Closure lock | `.backlogit/queue/.197-S.md.agent-lock` acquired, later released on halt (exit 0). |
| Pre-mode reconcile | Report `.backlogit/reconcile/197-S-pre-20261011T011730Z.md`: PROCEED. |

## Halt: what happened in `backlogit_ship_shipment`

**Correction (01:36Z, later in this invocation).** The steps below describe the state at the time of the MCP timeout. The server then completed the governed operation: the item log records `archived` at 18:34:48 PDT with `archive_path .backlogit/archive/197-S.md`. The control record is now `archived_status: shipped`, `queue/197-S.md` is gone, and all 19 explicit members are archived with `commit: f8d35936`. Where this section says the control record is "NOT archived" or "absent", that was true only before 18:34:48 PDT. The current halt reason is `SAFE_CLOSE_REPORT_UNAVAILABLE`: the safe-close result envelope and report were never returned to Ship, so the closure allowlist and commit cannot be built. See `.backlogit/reconcile/197-S-post-20261011T013645Z.md`.

1. Ship invoked `backlogit_ship_shipment(197-S, sha=f8d35936…)` (safe-close). The MCP call returned `McpError -32001: Request timed out`. The envelope was never returned, so the outcome was unknown.
2. Ship did **not** retry. It checked state read-only.
3. Observed state after the timeout:
   - `.backlogit/logs/197-S.jsonl`: `shipment_status_changed` to `shipped` at 18:21:45 PDT, then `commit_tracked` `f8d35936` at 18:21:59 PDT. Nothing after that. The shipped-event append **succeeded**. The failure point is not `shipped-event-append`.
   - `queue/197-S.md`: `status: shipped`, modified, **not archived**. `archive/197-S.md` is absent.
   - `197-F`: queue file deleted (tracked `D`); `archive/197-F.md` created (untracked).
   - 18 explicit task archive files: all modified (`M`). 5 carry `archived_from` provenance (197.001 to 197.005). The other 13 carry the same `commit` provenance through their modified content; details are in the git diff.
   - Tracked archive deletions: **0**. The P-007 restore guard does not apply.
   - Total changed entries in `queue/` and `archive/`: 21 (1 shipment control record, 1 feature queue deletion, 1 feature archive addition, 18 task archive modifications). These are **uncommitted** on the closure branch.
4. Subsequent MCP reads (`get_shipment`, SQL) timed out. The stash write `backlogit_stash` timed out and its marker was absent from `stash.jsonl`, so it was retried once. The retry succeeded (`B53056C0`).
5. The CLI `backlogit get 197-S` fails with `gate in progress for item` on the global shipment lifecycle lock `.backlogit/.locks/shipment-lifecycle-global`. That lock is held by the wedged governed operation. Ship did **not** delete or force it: manual lock removal is outside Ship's contract.
6. `backlogit doctor --check-shipped-event-completeness` (read-only, exit 0): 197-S's own shipped event is present. Its `missing_shipped_event` warnings concern historical shipments 060-S to 124-S and are unrelated.

Per safe-close step 10 and the P-017 halt rules, the closure lock was released on halt. Safe-close retry is **not** allowed now: safe-close rejects any state other than `active`, and 197-S is `shipped`. A retry would be refused, and any retry could also duplicate governed writes.

## Captured and recorded

- Operator follow-up stash `B53056C0` (priority high): reconciliation required. Confirmed present.
- Checkpoint `.backlogit/checkpoints/checkpoint-20261011-013448.json` (agent `ship`, phase `post-merge-closure-safe-close-partial`, `resume_hint` says not to retry).
- Halt record: this file.

## Operator actions to resume (in order)

1. Confirm no `backlogit` process is mid-operation. Restart the wedged backlogit MCP server.
2. Clear the global shipment lifecycle lock through the supported recovery path (`backlogit doctor`, or the supported lifecycle reconcile). Do not delete `.backlogit/.locks/` by hand.
3. Reconcile the 197-S control record under P-007 so that `archive/197-S.md` exists with `archived_status: shipped` and `queue/197-S.md` is removed. Use the governed path, not a manual move.
4. Verify the 18 task archive files and `archive/197-F.md` carry the expected closure provenance (`git --no-pager diff -- .backlogit/archive/`). Confirm the change set is exactly the 21 entries listed above.
5. Run a fresh `shipment-reconcile` post-mode with merge SHA `f8d35936deae71a468ccba6d769a3dfa2bdb3e20`. Expect `PROCEED`. If it returns `HALT — shipped-event reconciliation required`, stop and escalate per P-007.
6. Commit the closure by explicit path only, using the safe-close allowlist (no `git add .`). Stage the 21 entries, the post-mode report, and the reconcile reports. Use the Ship commit format.
7. Resume Ship at Step 6 item 2: runtime verification (the lock scripts are a runtime surface; the PowerShell round-trip is recorded above and the Bash round-trip is still open), operational closure (`closure_status` and `compaction_status` top-level), follow-up stash, P-020 compact-context, index resync, then the closure PR with the same P-014, P-018, CI, and merge-commit gates. Keep merges merge-commit only.

## Open items not done in this invocation

- Runtime verification and operational closure artifact (`docs/closure/197-S-*-post-merge-closure.md`).
- P-020 compact-context and backlog index resync (`backlogit_sync_index`).
- Closure PR and its gates.
- Bash lock round-trip on a Bash host.
- Operator ratification of the cycle-cap note (plan E3 item (c), stash `3F058389`). The authorized extra review-fix cycle was not used.

## Scope and boundaries kept

- No backlog item was created or triaged. No shipment membership changed. No lock was deleted. No archive was restored. No commit includes the partial closure mutations.
- The closure branch carries this memory note and its checkpoint and stash records only. The 21 partial mutations stay uncommitted until reconciled.
- Commit `45dea2f8` (`.autoharness/config.yaml`) is the operator's intentional routing edit and is not a 197-S scope item.

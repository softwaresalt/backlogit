# Stage memory: 196-S Amendment 3 (reopen route): review FAIL at cycle cap, reopen blocked

* **Session.** Stage, P-017 dark mode. Scope: `[196-S]`; `merge_approval_pre_authorized=true`;
  `admin_fallback_pre_authorized=false`.
* **Branch.** `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`,
  base HEAD `e380ff3b` (not pushed).
* **Routing.** `ROUTING_DEGRADED`: the intended route was claude-opus-5.5 / anthropic / high,
  but this session did not verifiably run on it.
* **Status.** `HALTED`. Plan-review decision FAIL at the 3-cycle cap. The governed reopen
  is BLOCKED. An operator ruling is required.

## Tool and index state

* MCP tools: OK. Index: `INDEX_SYNC_OK`.
* **MCP sync anomaly.** `backlogit_sync_index` did not pick up the manual edit to
  `.backlogit/queue/196-S.md`. CLI `backlogit sync` did.
* **Version skew.**
  * MCP: `7c805f9` (1.11.1-0.20261002061024).
  * CLI `C:\Tools\backlogit.exe`: v1.11.0 at `131577c`, an ancestor of the MCP commit.
  * The two differ only in `internal/core` claim-marker files and in `internal/cli` and
    `internal/mcp` tests.
* **Checkpoints and hooks.**
  * The Stage checkpoint scan found zero candidates (normal startup).
  * Hook events were polled through seq 3557 and NOT acked. They are outside the 196-S
    scope. The Orchestrator or a later Stage session owns them.

## Deliverables

| Item | Result |
|---|---|
| A. Amendment 3 | Appended to `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md`. Contents: A3.1 per-finding C1 table; A3.2 exact Go assertion specs with expected red strings; A3.3 TDD conditions; A3.4 reopen BLOCKED with operator options A, B, and C; A3.5 ordered Ship phases; A3.6 records, telemetry, and risks. The Objective is rewritten so P-001 no longer "blocks the next autonomous pipeline cycle". Each of U1, U2, U3, U5, U8, and U9 carries an "Amended by Amendment 3" line. |
| B. R02 | Single-key hand edit of `resume_checkpoint_ref` to `checkpoint-20261005-052350.json`, then CLI sync. MCP `backlogit_get_shipment` confirms. No governed setter exists, so the operator must ratify the edit (`A3_R02_RATIFIED`). `member_status_snapshot` is stale and was NOT hand-edited. |
| C. Plan review | Cycle 1 FAIL, cycle 2 FAIL, cycle 3 FAIL. The final decision is **FAIL**. The cycle-3 P1 fixes are applied but not re-reviewed, because the cap was reached. |
| D. P-021 C2 stash | See the mapping below. |
| E. Commit | One commit on the current branch. Not pushed. |
| F. Memory note | This file. |

## Finding dispositions (P-021 C1)

| Finding | Disposition | Record |
|---|---|---|
| R01 | Resolved | Under ratified P-001: PR #472 merged as `1ccecd94`, an ancestor of HEAD |
| R02 | Fixed | By Stage, per B above |
| R03 | Deferred | `CDBCB258`, normalized in place to C2 |
| R04, R07 | Same surface; C3 fix | U8 harness, 196.008-T (A3.2.3) |
| R05 | Deferred | `F6F3AA0E` (new C2 entry; requires deliberation) |
| R06 | Same surface; C3 fix | U1 harness, 196.001-T (A3.2.1). Requires the reopen. |
| R08 | Same surface; C3 fix | U3 harness, 196.003-T (A3.2.2). Requires the reopen. |
| R09 | Deferred | `147BD825` (new C2 entry; requires deliberation) |
| R10 | Deferred | `CDBCB258`, normalized in place to C2 |
| R11 | Dropped | Already pinned by `tests/integration/claim_start_admission_contract_test.go:80-86` (`TestUCS1_ClaimStartContract`) |
| R12 | Deferred | `3B25D37F`, verified C2-compliant |

* `CDBCB258` was normalized in place. It carries the `DEFERRED SCOPE EXPANSION` token,
  the source refs, and `requires deliberation: yes`. This keeps the earliest-captured
  identity, so no second entry was created.
* The unconditional duplicate scan (P-021 C5(A)) was clean for all four entries.
* No source ref was `N/A`, so late-identifier reconciliation C5(B) was a no-op.

## Halt reasons

1. **Governed reopen is blocked.**
   * The lifecycle allows only `done -> {archived}` (`internal/config/defaults.go`
     DefaultHooksConfig; `internal/hooks/builtin_pre.go`).
   * No governed operation reopens a `done` task.
   * Decision 2's "via governed transition" cannot be met. The operator must rule A, B,
     or C (A3.4); A is recommended and operator-executed.
   * Emits `DARK_MODE_HALTED` (P-017). Scope item: 196-S. Gate: G-A3 reopen. Next
     action: operator ruling.
2. **Plan-review FAIL at the 3-cycle cap.**
   * Ship must not execute any part of Amendment 3 until a later Plan Review record
     shows PASS, or ADVISORY with `operator_authorization: approved`.
   * Escalation status: `ESCALATION_DEGRADED`. No engram escalation intake is available,
     so the fallback is the operator halt.
   * Escalation payload:
     * Threshold: plan-review attempt 3.
     * Failure summary: the cycle-3 P1s were clean-tree semantics vs build-feature Step
       0.5b, missing committers for continuity files, and a shared red baseline.
     * Artifacts: the plan path and this note.
     * Resumption: the operator authorizes cycle 4 or records approval.

## Ordered Ship instructions (effective only after review approval)

1. **Phase 0 (no Step 4.0).**
   * The Orchestrator re-supplies the payload: served roots, plan path, R14 and A3
     citations, and decision 1 as the restore confirmation.
   * Ship restores `checkpoint-20261005-052350.json`.
   * Preconditions:
     * the manifest ref and items match
     * the R14 attestation passes
     * `hooks.yaml` is absent
     * the tree is clean
     * no green commits exist
     * live statuses are as expected
   * Ship compares the contract keys without re-freezing, resolves the checkpoint, and
     resumes 008 at Step 4.4.
   * Never append another `WORK_STARTED` to 008.
2. **Phase 1 (U8).**
   * Emit the P-005 event, then remove `harness-ready`.
   * Run harness-architect with `feature=196-F` and `tasks=196.008-T`, adding the A3.2.3
     assertions only.
   * Observe red: `DefinitionLiterals` FAIL, `CallSites` FAIL, `PreservedInvariants` PASS,
     and all 5 expected strings present.
   * Commit as `test(harness)`.
   * Make the traceability commit and confirm the tree is clean. Set
     `red_baseline_sha` := HEAD and run build-feature Step 0.5.
   * Run fresh gates and review, then red-path `done`.
   * Commit the completion bookkeeping, then run wave-1 Step 4.6 (open red {001, 003, 008}).
3. **Gate G-A3.** Unless the ruling is B or C, or is A with a committed Option A record:
   * write and commit a checkpoint and halt note
   * emit `DARK_MODE_HALTED`
   * halt
4. **Operator (ruling A).** Execute A3.4 Option A steps 1–10. Then commit the ruling file
   with `A3_RESUME_CHECKPOINT` and `A3_R02_RATIFIED`.
5. **Phase 0′.**
   * Restore the named checkpoint.
   * Preconditions: R14, committed ruling, `hooks.yaml` absent, 008 `done`, clean tree,
     and under A, 001 and 003 `queued` with exactly one valid `WORK_STARTED` each.
   * Route by ruling: A → Phase 2; B → halt for Amendment 4; C → Phase 3.
6. **Phase 2 (A only).**
   * Remove `harness-ready` from 001 and 003.
   * Step 4.0 wave 2 must give `ready_k = {001, 003}`.
   * Run one harness-architect call with `tasks=196.001-T,196.003-T`; expected red
     strings are U1 ×5 and U3 ×2. Make one `test(harness)` commit per owner.
   * Run Step 4.1b claims.
   * For each task in turn: baseline, gates, `done`, then a bookkeeping commit.
   * Run wave-2 Step 4.6.
7. **Phase 3.**
   * Wave 3: {002, 005}; 001 and 003 must turn GREEN.
   * Wave 4: {009}; 008 must turn GREEN.
   * Wave 5: {006}.
   * Under ruling A, closing-wave drift (declared 2/2/3, effective 3/3/4) is accepted and
     recorded.

## Local state notes

* Stage added `.backlogit/checkpoints/checkpoint-20261005-052350.json` to the 196-S
  continuity block in `.git/info/exclude`, so the file is never committed. This change
  is local only.

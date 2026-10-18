# Ship session: 198-S (CT test-suite health), wave 1 halt

Recorded: 2026-10-12T05:05Z (UTC). Branch: `feat/ct-restore-test-suite-health-and-runtime-diagnostics`.
Outcome: **DARK_MODE_HALTED** at Step 2 item 6 (P-002 harness gap on 198.004-T). No task was dispatched to build-feature. No PR was opened. No merge authorization was issued. Shipment 198-S remains `active` (claim-assigned), as the P-017 violation action requires.

## Served-root handoff outcome

`SERVED_ROOTS_OK shipment=198-S workspace_root=C:\Source\GitHub\backlogit storage_root=.backlogit attestation=metadata_catalog+pragma_database_list static_check=pass at 2026-10-12T05:01Z`

Attestation evidence: `backlogit_get_metadata_catalog` `workspace.root_path` and `storage_root` match the served roots. `pragma_database_list` (name `main`) returns exactly one row, `C:\Source\GitHub\backlogit\.backlogit\backlogit.db`, a direct child of the served storage root.

## Gates executed

| Gate | Result |
|---|---|
| P-012 tool probes | `backlogit_get_metadata_catalog` OK; `backlogit_sync_index` OK (`INDEX_SYNC_OK`); `backlogit_get_shipment` OK; `backlogit_query_sql` OK. `autoharness`, `backlogit`, `gh`, `go`, `golangci-lint`, `gofmt` present on PATH. engram and graphtor-docs not required this session. |
| Predecessor 197-S (CX) | Canonical `.backlogit/archive/197-S.md`: `status: archived`, `archived_status: shipped`. Satisfies the T5 after-CX-merge prerequisite. |
| TOPOLOGY pre_claim (pre-branch) | exit 0, `BRANCH_CREATE_ELIGIBLE` (on `main`, clean tree) |
| Branch creation (P-011) | `git checkout -b feat/ct-restore-test-suite-health-and-runtime-diagnostics` from `main` == `origin/main` == `c5758918` |
| TOPOLOGY pre_claim (pre-claim) | exit 0, `BRANCH_OK`, `WORKTREE_TOPOLOGY_OK`, `WORKTREE_CLEAN` |
| Claim | `backlogit_claim_shipment` returned an MCP timeout; the shipment was then observed `active` (`updated_at` 2026-10-12T04:26:19Z). The claim took effect. |
| TOPOLOGY post_claim | exit 0, `CLAIM_VERIFY_OK` (sole active shipment is 198-S) |
| wave scheduler replay | `scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue`: `WAVE_SIM_OK` (200/200 assertions, 26 scenarios) |

Frozen wave set M (Step 3): `{198.001-T, 198.002-T, 198.003-T, 198.004-T, 198.005-T}`, count 5. Excluded: `198-F` (feature, covering). No `red-deliverable-contract` or `green-regression-contract` blocks exist in M. Dependency graph is acyclic. Wave 1 = `{001, 002, 003, 004}`. Wave 2 = `{005}`, which depends on 001, 002, 003.

## Wave 1 harness scaffolding (commit `9762e75f`, test-only, 3 files, no production change)

| Task | Status | Evidence |
|---|---|---|
| 198.001-T (T1) | HARNESS_READY | `TestUT1_` wrapper over `TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters`. Passes on main by design (it is a stabilization). Red is the mutation-kill: reverting the 106-F unlock-after-append fix fails `duplicate counter 1 durably persisted` in 10/10 runs. Mutation reverted; production files identical to pre-mutation SHA256. |
| 198.002-T (T2) | HARNESS_READY | `internal/core/log_state_restore_test.go`: `TestUT2_CaptureSitesRestoreStdlibLogState`. Red by assertion on main at `gate_baseref_shadowed`, `gate_baseref_no_shadow`, `shipped_event_append_failure` (`log.Writer() still redirects to the capture buffer`). |
| 198.003-T (T3) | HARNESS_READY | `TestUT3_` wrapper over `TestClientLatestRespectsContextTimeout`. Passes on main. Red is the mutation-kill: removing context propagation in `Client.Latest` fails at `release_test.go:102` (`An error is expected but got nil.`). |
| 198.004-T (T4) | **P_002_GAP** | Unlabelled docs-only spike. No `harness-exempt` label, no `harness-exemption-contract` block, and the governing plan (`docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md`) enumerates no closed exempt set. A `docs-only` exemption is therefore not admissible. P-002.1 forbids a fabricated test for docs-only work, so harness-architect declined to scaffold a file-existence check. A gap comment is recorded on the task. |

Latent gap for wave 2: `198.005-T` (verification-only) has the same missing contract (no label, no block, not enumerated in the plan). It would fail the same static intake at wave 2 admission.

## Why the run halted

Step 2 item 6 (and Step 4.0 item 10): after scaffolding, every task in `ready_k` must be harness-satisfied. A gap halts the run and must not produce a partial wave. Ship may not author an exemption contract or edit planning fields (Role Boundary), and may not drop 198.004-T from M (only a Stage manifest amendment may change M). No build-feature dispatch occurred, and no `WORK_STARTED` record was written, because no task was dispatched.

## Required to resume (operator or Stage decision; Ship cannot author these)

1. Stage amendment to the governing plan: add `198.004-T` (`docs-only`) and `198.005-T` (`verification-only`) to a closed exempt set, each with a five-key `harness-exemption-contract` block (`exempt_precondition: must-fail-before-deliverable`; a runnable verification command that must fail before the deliverable exists), **or**
2. An operator/Stage decision to change M (for example, descope the spike), which is a Stage manifest amendment.
3. Then re-run Ship from wave 1 admission on this branch. The scaffold commit `9762e75f` and the `harness-ready` labels on 001-003 are reusable. Re-run the Step 4.0 attestation first.

## Residual state and notes

* The `.backlogit/queue` Markdown changes (claim-state and label writes) are uncommitted in the working tree, as the `.backlogit/` carry-forward prefix allows.
* Stale active checkpoint `checkpoint-20261008-073934.json` (ship, 153-S, post-merge closure; 153-S is already shipped and archived) was NOT resumed, restored, resolved, abandoned, or quarantined. It needs an operator decision. Stash 84E54F92 tracks the orphan-checkpoint gap.
* Plan AC selectors are unanchored. P-002.6 requires `^TestU<unit>_`, so `TestUT1_` and `TestUT3_` wrappers were added. The mutation-kill red for 001 and 003 is a documented deviation from P-004's literal wording, because the stabilized tests already pass on main. Stage should confirm.
* `agent-intercom` is not installed. Dark-mode events are recorded here and in the session summary (degraded remote visibility).
* gofmt on this Windows tree reports false positives on CRLF working copies. Formatting must be judged on LF-normalized content.

## Dark-mode events (P-017)

* `DARK_MODE_START` / `DARK_MODE_SCOPE`: recorded by the Orchestrator (scope 198-S only, `merge_approval_pre_authorized: true`, `admin_fallback_pre_authorized: false`).
* `DARK_MODE_HALTED`: P-002 harness gap on `198.004-T`, shipment 198-S, wave 1 admission. No PR, no merge.
* `LOCAL_REVIEW_READY`, `DARK_MODE_MERGE_AUTHORIZED`, `ADMIN_FALLBACK_ATTEMPTED`, `DARK_MODE_COMPLETE`: not reached.

---
title: "198-S CT full-suite green evidence"
source: docs/closure/198-S-ct-full-suite-green-evidence.md
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

# 198-S CT full-suite green evidence

This artifact is the T5 deliverable (task 198.005-T, verification-only) for shipment 198-S, CT test-suite health. It records one full-suite run at HEAD 0d1ee370293505857c5a4acdf65dccae58ae518b and one rerun of the single package that failed in that run. The first run was not clean, so the result is GREEN-AFTER-RERUN under the red-run policy in the plan (A1.3), not GREEN.

CT_RUN_COMMAND: go test ./... -count=1 -timeout 40m
CT_RUN_RESULT: GREEN-AFTER-RERUN
CT_RUN_EXIT: 1
CT_RUN_HEAD_SHA: 0d1ee370293505857c5a4acdf65dccae58ae518b
CT_RUN_EVIDENCE_PATH: logs/ct-verify/t5-full-suite-2026-10-12.log
CT_INTERNAL_CORE_SECONDS: 678.504
CT_ISOLATION_FIXES_HELD: DB48A817, FF1F3AC7, 885263D2
CT_RERUN_PACKAGE: github.com/softwaresalt/backlogit/internal/core
CT_RERUN_COMMAND: go test github.com/softwaresalt/backlogit/internal/core -count=1 -timeout 40m
CT_RERUN_EXIT: 0
CT_RERUN_EVIDENCE_PATH: logs/ct-verify/t5-rerun-core.log
CT_FLAKE_DESCRIPTION: github.com/softwaresalt/backlogit/internal/core failed TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog on the first run and passed on the rerun

## Run record

The full-suite run started at 2026-10-13T00:02:43Z and ended at 2026-10-13T00:20:16Z with exit code 1, about 17.5 minutes. The rerun of internal/core started at 2026-10-13T00:20:53Z, ended at 2026-10-13T00:32:17Z, and exited 0. The full run's complete output (no -v, no -json, no tail, no summary) is saved at logs/ct-verify/t5-full-suite-2026-10-12.log. That path is git-ignored and session-local, so the table below is the durable record of the per-package times.

## First run

Of the packages in the first run, 39 printed an ok line, cmd/backlogit reported no test files, and internal/core failed in 1000.746s. Exactly one package failed. The log has one named failure, `--- FAIL: TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog (30.75s)`, with an assertion that expected "archived" and got "queued" at artifact_mutation_lock_order_test.go:88. The log has no panic, no test timeout, and no other `--- FAIL:` line.

## Rerun

`go test github.com/softwaresalt/backlogit/internal/core -count=1 -timeout 40m` printed `ok github.com/softwaresalt/backlogit/internal/core 678.504s` and exited 0, saved at logs/ct-verify/t5-rerun-core.log. That is the internal/core time recorded above. The first run's internal/core time of 1000.746s is not used.

## Per-package times

The rows are the ok lines of the first run, plus the rerun's internal/core line. Seconds are as printed.

| Package | Seconds | Result |
| --- | ---: | --- |
| github.com/softwaresalt/backlogit/cmd/faultline-analyze | 1.910 | ok |
| github.com/softwaresalt/backlogit/cmd/gen-docs | 74.887 | ok |
| github.com/softwaresalt/backlogit/internal/atomicfile | 2.390 | ok |
| github.com/softwaresalt/backlogit/internal/canonical | 0.419 | ok |
| github.com/softwaresalt/backlogit/internal/cli | 236.052 | ok |
| github.com/softwaresalt/backlogit/internal/cli/format | 2.469 | ok |
| github.com/softwaresalt/backlogit/internal/config | 16.753 | ok |
| github.com/softwaresalt/backlogit/internal/core/gate | 3.937 | ok |
| github.com/softwaresalt/backlogit/internal/core/templates | 21.588 | ok |
| github.com/softwaresalt/backlogit/internal/db | 55.052 | ok |
| github.com/softwaresalt/backlogit/internal/docline | 5.943 | ok |
| github.com/softwaresalt/backlogit/internal/errors | 1.820 | ok |
| github.com/softwaresalt/backlogit/internal/events | 50.405 | ok |
| github.com/softwaresalt/backlogit/internal/faultline | 2.032 | ok |
| github.com/softwaresalt/backlogit/internal/faultline/analyzer/auditsuccess | 17.394 | ok |
| github.com/softwaresalt/backlogit/internal/faultline/analyzer/errwrap | 15.244 | ok |
| github.com/softwaresalt/backlogit/internal/faultline/analyzer/failopen | 14.826 | ok |
| github.com/softwaresalt/backlogit/internal/faultline/analyzer/locktimeout | 15.230 | ok |
| github.com/softwaresalt/backlogit/internal/faultline/analyzer/scannerdiscipline | 20.431 | ok |
| github.com/softwaresalt/backlogit/internal/faultline/compatcorpus | 2.511 | ok |
| github.com/softwaresalt/backlogit/internal/faultline/mutation | 1.488 | ok |
| github.com/softwaresalt/backlogit/internal/faultline/parity | 89.640 | ok |
| github.com/softwaresalt/backlogit/internal/fsutil | 0.478 | ok |
| github.com/softwaresalt/backlogit/internal/gateevidence | 1.735 | ok |
| github.com/softwaresalt/backlogit/internal/gateproof | 2.173 | ok |
| github.com/softwaresalt/backlogit/internal/hooks | 6.800 | ok |
| github.com/softwaresalt/backlogit/internal/jsonutil | 2.979 | ok |
| github.com/softwaresalt/backlogit/internal/mcp | 139.617 | ok |
| github.com/softwaresalt/backlogit/internal/mdfront | 0.259 | ok |
| github.com/softwaresalt/backlogit/internal/models | 3.623 | ok |
| github.com/softwaresalt/backlogit/internal/parser | 6.881 | ok |
| github.com/softwaresalt/backlogit/internal/release | 4.024 | ok |
| github.com/softwaresalt/backlogit/internal/stash | 3.859 | ok |
| github.com/softwaresalt/backlogit/internal/telemetry | 22.400 | ok |
| github.com/softwaresalt/backlogit/internal/version | 0.502 | ok |
| github.com/softwaresalt/backlogit/scripts | 3.222 | ok |
| github.com/softwaresalt/backlogit/tests | 149.668 | ok |
| github.com/softwaresalt/backlogit/tests/contract | 101.594 | ok |
| github.com/softwaresalt/backlogit/tests/integration | 116.968 | ok |
| github.com/softwaresalt/backlogit/internal/core | 678.504 | ok |

## Isolation fixes

The DB48A817, FF1F3AC7, and 885263D2 isolation fixes held on this evidence. No test in the CT-scope set failed in the first run: the set is TestUT1_ and TestUT2_ and TestAppendGateEvidence_ConcurrentSameItem in internal/core, and TestUT3_ and TestClientLatestRespectsContextTimeout in internal/release. The first run printed no `--- FAIL:` line for any of them, and the rerun of internal/core passed in full. This run is not the clean GREEN run the policy describes, because the first run had one failure outside that set. Stage should read "held" with that qualification.

## Host activity during the run

This session ran no other go command, build, lint, scan, or heavy process during either run. Before the first run it ran one short exempt-gate probe that exits at a file-existence check and never reaches a go command. Between the two runs it read the saved log and ran git read commands, all of which finished before the rerun began. Host-wide load and processes from other users or tools were not sampled, so the load during the run is not known.

## Flake classification

The failed test is TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog. It is defined in internal/core/artifact_mutation_lock_order_test.go, which is not one of the five files T1 to T3 edited (internal/core/gate_baseref_warn_test.go, internal/core/gate_evidence_formal_test.go, internal/core/log_state_restore_test.go, internal/core/shipment_shipped_event_durability_test.go, and internal/release/release_test.go). The test is also outside the CT-scope test names. This is the red-run policy's rerun case: one package failed on a test outside CT's scope, and the rerun passed.

This is a load-sensitivity finding: the test failed under full-suite load and passed when the package ran alone. It is not a waiver. CT does not widen to fix it. This task captures no stash entry.

## Ship annotation (2026-10-13, per plan Amendment A1.8 E-8 and the delta review)

This annotation supersedes the statements above that the isolation fixes held and that Ship's CT-scope-file check is still to be run. It does not change the gated scalars.

1. The sentences above that say the DB48A817, FF1F3AC7, and 885263D2 isolation fixes held do not apply to this result. The run is GREEN-AFTER-RERUN, not a clean green, so this artifact does not show that those fixes held. The CT_ISOLATION_FIXES_HELD line names the three stash IDs that the gate requires; it is not a claim that they held.
2. Ship's CT-scope-file check (A1.5, step 2) has been run. The failing test is defined in internal/core/artifact_mutation_lock_order_test.go. The branch diff against the merge base c5758918 changes only five Go test files, none of which is that file, so the overlap is zero and the failure is not CT-scope. The result is recorded on task 198.005-T.
3. The first-run failure recurs the load-sensitivity flake already tracked by stash 63909DAE. No new stash entry is created for it in this task.

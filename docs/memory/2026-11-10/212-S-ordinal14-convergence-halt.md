---
title: "212-S Ordinal 14 convergence timeout and integration assertion failure"
shipment_id: "212-S"
feature_id: "211-F"
phase: ordinal14-convergence-halted
status: blocked
---

## Outcome

HALTED, not convergence PASS. Ordinal 14 consumed one failed test cycle.
Historical failed count advances from 11 to 12, with no counter reset.
No retry, runner repair, implementation fix, new ordinal, push, PR update,
or merge is authorized by this failed attempt.

## Completed recovery and custody

* Verified all supplied Stage custody digests and pinned runner digest
* Independently attested served workspace `.`, storage `.backlogit`, and
  main database `.backlogit/backlogit.db` through MCP
* Validated live model-routing configuration; lifecycle topology PASS
* Restored the explicitly selected, conforming Ship checkpoint
  `checkpoint-20261110-040706.json`; completed bounded recovery with reachable,
  bound Engram and resolved only that selected checkpoint
* Preserved protected historical `checkpoint-20261106-210317.json` at SHA-256
  `235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`
* Vet and lint PASS under pinned Go environment before custody commit
* Committed Stage custody and recovery as
  `5b446cc903e8be1aea24f6a97e5c4577b7739fde`
* Required completely empty `git status --short` before the single runner launch;
  status remained empty after the runner returned

## Native evidence

```powershell
& bin/tmp/212-go-env.ps1 python bin/tmp/212-full-suite.py --authorization-token-file bin/tmp/212-ordinal14-coordinator.token --authorization-receipt bin/tmp/212-ordinal14-authorization.json --counter-receipt bin/tmp/212-ordinal14-counter.json --output-dir bin/_rr14a6e920/wave12-ordinal14
```

The explicitly delegated coordinator issuance bound Ordinal 14, failed count 11,
2700 seconds, scope `212-S`, operation `unfiltered-convergence`, and exact
clean HEAD `5b446cc903e8be1aea24f6a97e5c4577b7739fde`.
The pinned runner SHA-256 remained unchanged before and after execution:
`9aead967085d5a9f45a2705ba4d2035e15b4cf71e160f8dacd4ecf1b500b957e`.
Issuance receipts remain historical evidence only. The separate
`bin/tmp/212-ordinal14-outcome.json` records consumption and forbids reuse.

Discovery succeeded in 4.393 seconds with 46 packages. LF preflight passed.
The unfiltered `go test -count=1 -p=1 -timeout=30m -json ./...` child started
at `2026-11-10T04:33:12Z` and timed out at `2026-11-10T05:18:12Z`
after 2700.043 seconds.

| Field | Observation |
|---|---|
| Recorder exit | 124 |
| `native_exit` / `suite_native_exit` | 124 / 124 |
| `evidence_valid` | false |
| `test_child_launched` / `test_cycle_consumed` | true / true |
| `prelaunch_only` | false |
| Passing package terminals | 45 of 46 |
| Missing terminal | `github.com/softwaresalt/backlogit/tests/integration` |
| `reported_failure_count` | 2, parent and subtest failure events |
| Top-level test passes | 3827 |
| Critical receipt errors | None |
| HEAD before / after | Same exact bound HEAD |
| Supervisor cleanup errors | None |
| Poll gaps greater than 5 seconds | 0 |
| Execution-state hold | Cleared on owning thread |

Result: `bin/_rr14a6e920/wave12-ordinal14/result.json`, SHA-256
`3910203002a2dbe9cb3d99dcbb26b67705e8e35c6ca822a7a47386beae128ed8`.
Driver snapshots, discovery/preflight evidence, and bounded raw streams remain
in the same directory. The raw-stream cap omitted bytes; the recorder reports
zero omitted failure records, unfinished-test entries, or package results.

## Observed integration failures and unfinished work

The recorded failures are `TestUCS1_ClaimStartContract/Ship` and its parent
`TestUCS1_ClaimStartContract`. The subtest reports missing literal assertions
in Ship Step 4.0:

* `ready_k = { t in queued or claim-assigned : every dependency of t is terminal_success }`
* `` `active_ids` contains only active residuals, never claim-assigned or indeterminate members. ``

This is observed contract-text drift, not a proven cause of the suite timeout.
No instruction, test, source, or planning artifact was repaired.

At timeout the unfinished chain was
`TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract` /
`leaf_mutates_caller_modified_path_refused` / `real_git_capture_compare` /
`zero`. Its retained output includes `WAVE_SIM_OK: 285/285 assertions PASS
across 38 scenario(s)` and three native cleanup receipts. These partial
observations do not establish integration completion or a timeout root cause.
No panic candidate was recorded.

## Resumption boundary

All 22 frozen task members remain terminal; open RED remains empty.
Shipment and covering feature remain active on the existing shipment branch.
No task was reclaimed, restarted, or re-completed.
Existing residuals `0D073C21`, `D2FCA0FB`, native POSIX validation, and incomplete
graph coverage remain unresolved.

Require Orchestrator/operator/Stage disposition of the failed convergence,
contract-text assertions, and unfinished integration chain before further
execution. Do not retry the suite, issue replacement receipts, change planning
artifacts, or infer Ordinal 15 authority.
Full local build, current-HEAD standard/adversarial review, PR creation,
CI/Copilot/runtime readiness, and merge approval remain pending.
No PR URL is available from this invocation.

Leave one current-session recovery checkpoint at this halt. Recovery requires
explicit selection and confirmation; the protected historical checkpoint
remains untouched and must never be resolved by this workflow.

Final current-session checkpoint:
`.backlogit/checkpoints/checkpoint-20261110-051913.json`.

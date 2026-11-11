---
title: U2j C1 local fix evidence
task: 211.016-T
reviewed_head: ffb9203ab52014772cf76b4fc5df752e38a01b5c
historical_red_baseline_sha: 30908ecb8438c0b3e7fe8bd514fa3680e35d90e5
---

## RED before source changes

Pinned Go 1.24.0, offline, readonly modules, fresh owned ignored TEMP.
Whole compile `go test -run=^$ -count=1 ./...`: native 0.

Same U2j selector, anchored C1 subcases, race, count 1: native 1.
Two subcases ran and failed on assertions (no skip or compile failure).
Simulation observed five concrete outcome failures:

* Identical conflicted `U` accepted instead of mapping refusal
* Identical tracked `??` accepted instead of mapping refusal
* Three production deltas masked by compile failure instead of production refusal

The positive closed-census assertion separately found 40 missing emissions in
the tracked 20-control run, despite native simulation exit 0 and 182/182 PASS.
Removing all control emissions was also rejected by negative replay.
Native evidence: completed tool session 1130.

## Carried custody

Prior five scoped invocations and their native proofs remain unchanged.
Prior two distinct paused-replay failures are retained; this C1 assertion RED
adds one counted failed observation, not a reset. Prior six full-suite failures
are untouched. No full suite, CI, reviewer Git execution, push or PR is claimed.
Standard review reported P1=3/P2=6; three distinct adversarial reviews reported
major bugs. This is local-gate remediation, not an external denying actor claim.

Only the six task-authorized source files and this owned proof are writable.
Caller hook/task016/memory remain unstaged. Task015, its original start and
historical baseline are unchanged. Parent lifecycle continuation remains outside
this leaf's authority.

## Stop checkpoint

Timestamp: 2026-11-07T20:40:46.5208589-08:00.
Current HEAD remains `ffb9203ab52014772cf76b4fc5df752e38a01b5c`.
No commit, staging, task status change, continuation dispatch or review was performed.
The carried five-failure build budget is exhausted; no further fix or harness
retry is authorized within this loop. This checkpoint does not reset any counter.

| Counted harness failure | Native | Observation |
|---|---|---|
| Prior 1 | 1 | Full U2j paused replay compile-order violations, prior GREEN manifest |
| Prior 2 | 1 | Anchored paused diagnostic: expected 0 vs actual 4, prior GREEN manifest |
| C1 3, session 1130 | 1 | Assertion RED: invalid statuses, production precedence, missing positive census |
| C1 4, session 1137 | 1 | Exact scoped race: all real captures failed; concrete error transport was incomplete |
| C1 5, session 1143 | 1 | Exact scoped race: capture runtime result lacks `Text` property |

The standalone simulation also failed once (session 1134, native 1):
the NEW tracked paused fixture had nine members and nine waves, so its final
admission hit `WAVE_BUDGET_EXCEEDED`. Adding the actual already-done prefix members
fixed that NEW fixture. Session 1136 returned native 0, 202/202 assertions across
27 scenarios, with 44 positive emissions for 22 branch controls.
This distinct standalone-gate failure is retained, not hidden or used to reset
the carried harness budget.

For session 1137 the missing concrete capture text remained provisional.
Read-only inspection observed the `GIT_CONFIG_COUNT` override name; command-scope
config inputs were subsequently bound instead of categorically refused.
That inspection did not prove the first runtime error differed. Session 1143
established the concrete remaining error:
`WAVE_RED_MAPPING_UNRESOLVED: capture refused: The property 'Text' cannot be found
on this object.` No post-trip probe or further source edit followed.

## Actual scoped census and native gates

The exact acceptance-criteria race command ran twice in C1, both native 1:

```text
go test -json -v -timeout=30m -count=1 -race -run '^(TestUCS3_WaveSimClaimStart|TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract)$' ./tests/integration
```

Latest census: 58 RUN, 22 PASS, 36 FAIL, zero SKIP.
U2j: 54 RUN, 18 PASS, 36 FAIL. UCS3: 4 RUN/PASS.
All 33 real-Git cases ran and failed at capture, before their outcome oracles.
They are NOT native GREEN proofs. Pure controls and all ten paused negative
bindings passed, including duplicate/missing actual starts.
The negative removed-control fixture produced expected native 1 with
`FAIL red_census :: closed positive control census`; its Go guard passed.

| Other gate | Native/result | Session |
|---|---|---|
| Whole compile, no tests | 0, baseline and post-edit | 1129, 1135 |
| Whole vet, Go 1.24 offline/readonly | 0 | 1140 |
| Pinned v1.64.8 lint | 0; binary reports built with Go 1.26.5 | 1141 |
| All 1,390 tracked Go files, readonly gofmt batches | 0, zero findings | 1139 |
| PowerShell parser | 0, zero errors | 1142 |
| JSON, UTF-8 LF, prompt YAML frontmatter | 0 | 1144 |
| Readonly diff check | 0, no whitespace findings | 1145 |

Session 1144 compared every original field of all 26 historical scenarios and
20 pre-C1 controls (including the original 18). All remain exactly unchanged.
Prompt frontmatter is byte-identical to HEAD and parses as YAML.
All six source worktrees are UTF-8 LF. Git's autocrlf warnings concern a future
checkout conversion; no config or exclusions were changed.

## Six source fingerprints at the stop window

These are uncommitted worktree SHA-256 values, not reviewed committed identity.

| Source | Worktree SHA-256 |
|---|---|
| `tests/integration/claim_start_wave_sim_contract_test.go` | `31ff808e04ac4cfc0463425b8faa751b0a432f4c44be2e44f1c0926e1737a413` |
| `scripts/wave-scheduler-sim.ps1` | `4bc6ec7813f951fa17f54046272e957bbc5f21fd84a14aed537c8b92a41c5f4b` |
| `tests/simulation/wave-scheduler-contract.json` | `2f09d4b9025b08c45c140ed7769d919d3738dab1ecef0d311dfeccfc8a0dc0e3` |
| `.github/skills/build-feature/SKILL.md` | `64eec36afe07f06ba51a7cc3d01ef6d78d0f7e1ee1c1612e189011a63efcc340` |
| `.github/agents/_ship.agent.md` | `03a54c6061dd1e3bf036308fe3cc1b11447d93389033b501469fba1105bc3adc` |
| `.github/policies/workflow-policies.md` | `7aeda106f84868086c5c5b6aa4ff8ddd8ec241a329d34c5eaed51b19f251238e` |

At HEAD above, corresponding committed Git blob OIDs are:

```text
Go     1e67d546fa6584b21c5bdb18631debbd0aba093b
PS     2e05036df0e739e9d6113e055034bdde13e3876d
JSON   1a84036953d4ed584e328977264b149b1af87cc7
Skill  f359ae96ddc76db07f2002ea0e233004c3dd1c41
Ship   cceaa7ce3b31b53bff69a6ddc54fabab3dd7680f
Policy b5cf28136514412a13556c448aba0c5e2cfd9ec5
```

All six committed-blob/worktree equality checks are FALSE at this stop window.
No new committed review-window attestation is claimed. Existing committed
fingerprints remain in the untouched prior GREEN manifest. No reviewer is
represented as having independently run Git.

## Unresolved C1 completion

* C1.5/C1.8: the public capture path fails because the native runner's result
  lacks the expected `Text` property
* C1.2/C1.3/C1.4/C1.8: real policy hiding/change, dirty-byte, index/mode,
  assume-unchanged/skip-worktree, digest tamper, ignored scratch, legitimate
  post-baseline HEAD and independent-binding outcomes remain unverified:
  every one of the 33 real cases stops at that capture failure
* C1.7/C1.9: supported caller resume/acceptance remains unexecuted; task016 is
  still active, no coherent GREEN commit exists, and no task015 lifecycle
  transition or open-red registration was performed by this leaf
* Overall C1 remains RED. No finding was deferred or described as completed

## Operator-approved focused R5 iteration6

Approval: 2026-11-08T04:56:44Z, exactly one focused iteration on 211.016-T.
Only the awaited write result at scripts/wave-scheduler-sim.ps1:521 was discarded.
No exception, budget, cap, drain, cleanup, test, or policy change was made.

Sole scoped native exit: 1. RUN 24 / PASS 0 / FAIL 0 / SKIP 0.
Real-Git functional cases: RUN 9 / PASS 0 of 33. No GREEN claimed.
Timing UTC: 2026-11-08T05:02:08.3345891+00:00 to 2026-11-08T06:50:48.4823317+00:00; 6517.811507 seconds.

```text
go test -json -v -timeout=30m -count=1 -race -run '^(TestUCS3_WaveSimClaimStart|TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract)$' ./tests/integration
```

| Named census | RUN | PASS | FAIL | SKIP |
|---|---|---|---|---|
| TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract | 24 | 0 | 0 | 0 |
| TestUCS3_WaveSimClaimStart | 0 | 0 | 0 | 0 |

| Real-Git case | Outcome |
|---|---|
| zero | RUN 1 / PASS 0 |
| task-byte | RUN 1 / PASS 0 |
| hook-byte | RUN 1 / PASS 0 |
| task-stage | RUN 1 / PASS 0 |
| index-mode | RUN 1 / PASS 0 |
| hook-delete | RUN 1 / PASS 0 |
| untracked-delete | RUN 1 / PASS 0 |
| new-test | RUN 1 / PASS 0 |
| new-config | RUN 1 / PASS 0 |
| new-docs | RUN 0 / PASS 0 |
| production-leaf | RUN 0 / PASS 0 |
| production-predispatch | RUN 0 / PASS 0 |
| clean-tracked-byte | RUN 0 / PASS 0 |
| ignored-scratch | RUN 0 / PASS 0 |
| head-change | RUN 0 / PASS 0 |
| raw-binary-byte | RUN 0 / PASS 0 |
| reparse-escape | RUN 0 / PASS 0 |
| policy-hidden-production | RUN 0 / PASS 0 |
| policy-only | RUN 0 / PASS 0 |
| assume-unchanged | RUN 0 / PASS 0 |
| skip-worktree | RUN 0 / PASS 0 |
| post-baseline-zero | RUN 0 / PASS 0 |
| post-baseline-hook | RUN 0 / PASS 0 |
| post-baseline-task | RUN 0 / PASS 0 |
| post-baseline-doc | RUN 0 / PASS 0 |
| pre-tamper | RUN 0 / PASS 0 |
| post-tamper | RUN 0 / PASS 0 |
| missing-bindings | RUN 0 / PASS 0 |
| no-clobber | RUN 0 / PASS 0 |
| nonignored-output | RUN 0 / PASS 0 |
| config-policy-change | RUN 0 / PASS 0 |
| unknown-exclusion | RUN 0 / PASS 0 |
| utf8-path | RUN 0 / PASS 0 |

Original seven raw byte backups and hashes: bin/tmp/212-r5i6-private/before/
and before-sha256.raw. Original manifest SHA-256:
`bed42a97231322a5b26d4f99ea4881694441df322a71c7dad5a22b4b5513396d`.
PS before SHA-256:
`4bc6ec7813f951fa17f54046272e957bbc5f21fd84a14aed537c8b92a41c5f4b`.

| Current source | SHA-256 |
|---|---|
| `tests/integration/claim_start_wave_sim_contract_test.go` | `31ff808e04ac4cfc0463425b8faa751b0a432f4c44be2e44f1c0926e1737a413` |
| `scripts/wave-scheduler-sim.ps1` | `ef1a6eda84ba610f03a3aa8b9e5fd830f99b7f5b63afedff5253f76e2021c509` |
| `tests/simulation/wave-scheduler-contract.json` | `2f09d4b9025b08c45c140ed7769d919d3738dab1ecef0d311dfeccfc8a0dc0e3` |
| `.github/skills/build-feature/SKILL.md` | `64eec36afe07f06ba51a7cc3d01ef6d78d0f7e1ee1c1612e189011a63efcc340` |
| `.github/agents/_ship.agent.md` | `03a54c6061dd1e3bf036308fe3cc1b11447d93389033b501469fba1105bc3adc` |
| `.github/policies/workflow-policies.md` | `7aeda106f84868086c5c5b6aa4ff8ddd8ec241a329d34c5eaed51b19f251238e` |

Raw native artifacts: bin/tmp/212-r5i6-private/go.stdout.jsonl.raw,
go.stderr.raw, go.timing-census.raw.
Prior five counted failures (including intended C1 RED), standalone failure,
and six historical full-suite failures remain unchanged.
Native FAIL adds counted failure 6. STOP: no seventh command or repair.
HEAD remains ffb9203ab52014772cf76b4fc5df752e38a01b5c.
No full suite, staging, commit, review, task lifecycle, hook, or archive mutation.

## Assertion-RED-authorized owned-command source fix

Timestamp: 2026-11-08T08:07:06Z. Source implementation only; native runs: zero.
The actual controlled assertion RED is preserved at
`bin\_r5compile_23a17318\controlled-red\result.json`
(native 1, 16.82997790025547s, no panic, skip, or timeout).
Its expected RED remains separate from the seven local and six historical
full-suite failures. No failure counter was reset or additional agent counter
introduced.

The shared test seam now owns its combined-output pipe, drains for one second,
disposes its owned tree, closes the pipe, joins its copier, and calls `Cmd.Wait`
exactly once. Clean disposal preserves the original native error; incomplete
drain explicitly returns `exec.ErrWaitDelay`. Disposal errors are joined and
fail the Go test independently of a native exit error. PID, native exit, drain
state, and disposal proof are logged through Go testing, not child JSON.

The two test-only adapters are necessarily coupled to this seam:

* `u2j_process_windows_test.go`: create kill-on-close Job, start suspended,
  assign before any child code, resume only the owned primary thread, retain
  parent handle, cancel idempotently, and check Job active count within 500ms
* `u2j_process_linux_test.go`: create owned process group, observe parent exit
  with non-reaping `waitid`, kill the reserved group before parent reap, and
  serialize cancellation/disposal

Filename OS selection covers Windows/Linux test CI without build tags or skips.
Windows ABI pointers are test-only, explicitly justified and pinned; the existing
cached `x/sys` dependency is used. No production source or dependency changed.

| Source | SHA-256 |
|---|---|
| `tests\integration\claim_start_wave_sim_contract_test.go` | `06e5ee22481e26a29ef992b434087e68eb0543a70970a12d172b5a5a534a44e7` |
| `tests\integration\u2j_process_windows_test.go` | `3393b05f270683d1e44d4a8b76efa4aa4df51ac5f4c8ccd6dc8d54c4ac452a3d` |
| `tests\integration\u2j_process_linux_test.go` | `2e21ef1e5ba13ed5628a2d83a3ed3a6388ad924cf1637811b2e667822d6fadda` |

Exclusive ignored byte backups and new-helper absence/post-edit evidence:
`bin\_u2j_owned_source_20261108_01\`. Existing RED proofs are untouched.
Static source inspection confirms all five seam calls, unchanged controlled
fixture bytes and 33-case declaration, and no zero WaitDelay. The controlled
oracle retains successful parent exit, START/END discrimination, explicit
ErrWaitDelay and no-END assertions, with an added exact-error assertion for
successful disposal. Historical scenario JSON and the four other C1 sources
retain their recorded hashes; PowerShell remains `ef1a6eda84ba610f03a3aa8b9e5fd830f99b7f5b63afedff5253f76e2021c509`.
One static byte-format check rejected newly created CRLF helper files; their
bytes were backed up exclusively and only line endings normalized to LF.
An append-only byte check detected normalization of earlier mixed manifest
line endings by the patch tool; its original byte prefix was restored from
the exclusive backup while retaining this new source-only appendix.
No native process experiment, Go build/test, syntax runner, or formatter ran.

Safety: freeze-scope to this seam/adapters; moderate test-contract change,
conditionally operator-authorized, applied as source only. Static self-review
covered ownership-before-resume, cancellation serialization, reserved identity,
single parent reap, copier join and error accounting. Runtime success is not
claimed. Parent exclusively owns one rebuild followed by A180/B600 and must
stop on an unexpected failure. No full suite, metadata/status change, Stage,
RCA, review dispatch, push, commit, or original task015/start mutation occurred.

Compact-context invocation is bounded to this active task's memory: preserve
the active cursor, all failure/RED evidence, and parent rebuild/A/B ownership;
no completed or stale in-scope candidate is eligible for archival.

## Narrow scheduler C1 review cycle 3 of 3

Timestamp: 2026-11-08T08:47:15Z. Task `211.016-T`; HEAD unchanged.
Freeze-scope: scheduler logic/help, tracked scenario additions, coupled P-002.6
wording and Ship pending-gate alignment only. No Go/helper change, native
capture, Go/build/test-binary invocation, review delegation, backlog mutation,
commit or push. Moderate, operator-authorized source completion; applied.

Before writes, exact affected bytes were preserved exclusively in ignored
`bin\_r5scheduler_c3_be983c4c\`; `before.json` records their SHA-256 values.
The existing manifest's raw prefix is retained, not rewritten as new history.

### Intended simulation RED and final GREEN

* Session 1178: declared read-only PowerShell simulation, exit 1,
  229/254 assertions passed. Foreign and multiple active shipments incorrectly
  completed; the marker-without-claim control reported residual, not indeterminate.
  Initial new transition controls also needed their 016 prerequisite reset to
  queued after simulated shipment claim; that fixture correction is explicit.
* Session 1179: corrected focused transition fixture, exit 1, 51/55 passed.
  Wave 3 was absent from compile gates (`1,2,4,5,6,7,8,9`); retained-command and
  discharge observables were absent. This preceded the scheduler logic fix.
* Session 1180: full tracked simulation, exit 0, 257/257 passed across 38 scenarios.
* Session 1184: final current-source full tracked simulation, exit 0,
  257/257 passed across 38 scenarios after final bounded guard/command-set checks.

These are intended new PowerShell contract RED/GREEN observations, not additional
native Go failures or rearmed counters. Seven local failed observations, six
historical full-suite failures and expected cleanup RED 1 remain carried intact.

### Bounded outcomes and preserved authority

Indeterminate classification now precedes amendment recognition. Foreign,
multiple-active and marker-without-claim controls halt before scheduling.
Unsupported, blocked, duplicate-start, archived-prerequisite and other active
residual controls still refuse; no old expectation was weakened.

Queued 016 completes, Wave 3 stays PENDING with closed command identity
`211.016-T`, and only the same-started 015 cursor may resume. Reload/fresh proof
and assertion RED precede registration. Wave 3 and Wave 4 closed command sets
(`211.016-T`, `211.015-T`) discharge at execution cursor 4, with compile/static
and open/closed-selector obligations, before U3 admission. Wave 3 is not FULL
PASS or full-suite deferral before registration. A simulated Wave 3 gate failure
leaves both 3 and 4 pending and halts at cursor 4; early GREEN refuses registration
and leaves 3 pending without executing U3.

P-002.6's residual/frontier/census and convergence lines now cross-reference only
the exact Stage `089-DL/M16`, `212-S`, 015/016 exception and existing zero-delta
paragraph. Ship's pending-gate wording aligns. No P-002.4, exemption, status
vocabulary, baseline, start history or generic active authority changed.
Help limits the no-process/no-write guarantee to simulation mode.

### Equivalence and handoff

Session 1183 wrote `bin\_r5scheduler_c3_be983c4c\equivalence.json`.
All original 27 scenarios (including the original 26), all 22 branch controls,
and every pre-existing top-level fixture field are identical; 11 scenarios were
added. The whole capture/compare/Git-helper/branch segment is byte-identical
before/after, SHA-256
`c8981bcaa1977a51e5ef57788d7796281798b4401db95a49b03135441e12b369`.
The pre-edit whole-script hash matches the compiled B source fingerprint
`ef1a6eda84ba610f03a3aa8b9e5fd830f99b7f5b63afedff5253f76e2021c509`.

All three Go test/helper files and B binary are unchanged from
`bin\_r5compile_23a17318\compile-after\result.json`; binary SHA-256 remains
`eba8a2b05f4712ea1cb403c4f3b6a9dd91131ba968d145e26801b504da44b901`.
Parent supplied A/B GREEN, 33 real-Git cases and 58/58 census; this leaf neither
reran B nor claims native B executed against the changed full scheduler source.
The new full-scheduler PowerShell GREEN and unchanged capture projection are
separate evidence.

| Current source | SHA-256 |
|---|---|
| `scripts\wave-scheduler-sim.ps1` | `9b1a4efaaefa4a8a771afaf56d22a8cae844d7c7bf5dcd0755d42f07235a30b7` |
| `tests\simulation\wave-scheduler-contract.json` | `21257b9540be46f07ef77abfcf1de4a16ef69d12ce117b39a3cf2482f7de9d76` |
| `.github\policies\workflow-policies.md` | `812bd9341e2f977e5eb5001d5366b3aa97f9bd82f6a5f19a89955193e71cdbdb` |
| `.github\agents\_ship.agent.md` | `73626e4da2f4fdc72b16507a40ee11f8af56018e09fe2aefc9693298699f9f39` |

PowerShell parse, JSON/UTF-8 LF, byte-identical YAML frontmatter with parser
validation, and read-only diff whitespace checks passed. No native platform
validation was added. Other-GOOS adapter support remains outside this exact
contract for parent C2 capture/Stage disposition, not expanded here.

Compact-context remains bounded to this active C1 evidence: no completed or
stale in-scope candidate is eligible for archival. Preserve the active cursor,
all carried gates/counters and pending parent authority. STOP at source fix and
PSSIM_GREEN; parent owns FINAL standard/adversarial review and lifecycle decisions.

## Final current-source completion evidence

{
  "utc": "2026-11-08T09:15:32.206095+00:00",
  "identity": "ffb9203ab52014772cf76b4fc5df752e38a01b5c plus eight source hashes",
  "source_hashes": {
    "scripts/wave-scheduler-sim.ps1": "9b1a4efaaefa4a8a771afaf56d22a8cae844d7c7bf5dcd0755d42f07235a30b7",
    "tests/simulation/wave-scheduler-contract.json": "21257b9540be46f07ef77abfcf1de4a16ef69d12ce117b39a3cf2482f7de9d76",
    "tests/integration/claim_start_wave_sim_contract_test.go": "c107d6066b01405514020b745c94039b2a2ffdfa40595b841ceaf1d39c01850a",
    "tests/integration/u2j_process_windows_test.go": "3393b05f270683d1e44d4a8b76efa4aa4df51ac5f4c8ccd6dc8d54c4ac452a3d",
    "tests/integration/u2j_process_linux_test.go": "2e21ef1e5ba13ed5628a2d83a3ed3a6388ad924cf1637811b2e667822d6fadda",
    ".github/skills/build-feature/SKILL.md": "64eec36afe07f06ba51a7cc3d01ef6d78d0f7e1ee1c1612e189011a63efcc340",
    ".github/agents/_ship.agent.md": "73626e4da2f4fdc72b16507a40ee11f8af56018e09fe2aefc9693298699f9f39",
    ".github/policies/workflow-policies.md": "812bd9341e2f977e5eb5001d5366b3aa97f9bd82f6a5f19a89955193e71cdbdb"
  },
  "standard": "READY_WITH_FOLLOWUPS; P0=0 P1=0",
  "three_model_routing_registry": "bin/tmp/1794128658918-copilot-tool-output-4d593e006b4b4eb7b16f4cf3acd56667.txt",
  "review_models": [
    "gpt-6.1-sol",
    "claude-haiku-5.5",
    "claude-sonnet-5.5"
  ],
  "anchor_coverage_completion": "R5-C3-Anchor-coverage-completion, explicit gpt-6.1-sol/high; all eight sources read",
  "adversarial_disposition": "No current R5 P0/P1 established; original missing routing/coverage provenance recovered without new pool. Unique panic-table concern is pre-existing P3 advisory, not demonstrated R5 regression.",
  "followup_ids": [
    "FB04387F",
    "1CA4D253",
    "6D46EDAD"
  ],
  "local_failed_observations": 7,
  "expected_controlled_cleanup_red": 1,
  "historical_full_failures": 6,
  "A": "compiled native0; 4/4 PASS; 26.67s",
  "B": "compiled native0; 58/58 PASS and 33/33 real-Git; 547.54s; no omissions/skips/cleanup errors",
  "C3": "read-only simulation257/257 across38; native helper function bytes unchanged from B, no fresh B after C3",
  "static": "Go vet and pinned v1.64.8 lint PASS; changed Go formatting, PS syntax and diff checks PASS",
  "raw_backup_relocation": "bin/tmp/212-r5i6-private/before/go-backup-path-relocation.json; old Go backup suffix .raw, exact31ff SHA preserved",
  "previous_manifest_raw_sha256": "e329e691e17a489e319e52baa69430188c34e5b267eea0e9f4984c1bd40e7de5",
  "previous_manifest_raw_path": "bin\\_r5compile_23a17318\\c1-manifest-before-final-canonical.raw"
}

Historical manifest bytes are preserved at the raw backup above; this working manifest is canonical UTF8LF. Model bindings come from caller registry metadata, not model self-identification. Source reviews were report-only. These are task-scoped/projection and read-only-simulation proofs, not a new unfiltered or post-C3 B execution.

## Live Task015 pre-dispatch boundary

Task016 is done and its reviewed source commit is b519164ef365fefe27d21ef26644d9ac9733826c; owned completion bookkeeping HEAD is 7f3fd6694e67e962bec21544d344f10f4949e6fc. Task015 retains its single start and original30908 history baseline. Wave3/4 gate obligations remain pending until its RED acceptance.

The corrected public Capture entrypoint failed before leaf dispatch: native1, 22.157s, owned PID42172, no timeout or cleanup error, no predispatch JSON created. Error: WAVE_RED_MAPPING_UNRESOLVED: capture refused: unsafe path alias. Raw proof is bin/_r5compile_23a17318/task015-resume/capture-pre-result.json and capture-pre.stderr.raw.

Bounded read-only Git metadata inspection found no alias-shaped tracked or ordinary untracked paths. The ignored-policy listing returns collapsed nested-repository DIRECTORY summary records ending in '/', including .copilot/session-state/f7052ee9-365f-4592-9568-5f8264c008bd/files/canonical-suite/ and the retained r5i6-temp new-config fixture directory. Get-RedSnapshotPolicySHA256 currently sends these non-file records to Get-RedSnapshotSafePath, whose empty final segment correctly refuses them at line564. This is a collector input-kind mismatch, not permission to weaken traversal/alias validation or delete ignored evidence.

No Task015 leaf, new start/claim, new Go test, unfiltered suite or code correction followed. Seven local failed test observations and six full failures remain; this separate pre-dispatch capture failure is retained. Source review cycle3 is complete, so a further bounded C1 collector repair requires disposition rather than an automatic fourth review/fix cycle. All existing source, raw proofs and deferred-entry payloads remain preserved.
## Operator-approved collector cycle4 result

Approval2026-11-08T09:50:17Z covered the exact input-kind correction and review extension, with first-unexpected-error stop. Compilation-before native0; actual named collector assertion RED/native1 in13.16s; compilation-after native0; focused corrected native0 in25.93s; compiled scoped suite native0, 59/59 named tests and34/34 real-Git cases in554.33s. Original33 cases preserved; new case covers ignored nested-repository directory summaries and strict trailing-separator FILE negatives.

The sole live whole-workspace Capture slot then timed out: supervisor-forced native124 after55.120s (60s authority with cleanup reservation), owned PID3844; zero stdout/stderr bytes and no snapshot JSON. Job was assigned before resume; idle hold/clear occurred on the same thread3152; max poll gap0.011s, no cleanup errors. This does not establish a capture hang, filesystem cause or live correctness. Temporary fixture GREEN is not a live-root PASS.

Evidence: bin/_r5collector_ce30568e/compile-red/result.json, assertion-red/result.json, compile-green/result.json, focused-green/result.json, scoped-green/result.json and live-capture/result.json. Original bytes are preserved by source backup wave-scheduler-sim.before-directory-summary.ps1 and Go scaffold backup. Current source hashes: Go df5fbe18b792e37936c8a4dbf91d99f5d0c5276597a27b802f4ed343c22b0cc2; Script bf235e224828398dc19359edd69363d7234f0c1b9a0bc3b12f938ee12d81c7fd. Generic file-path validators and other source fields are unchanged.

STOP after this first unexpected live-capture timeout. No retry, source edit, reviewcycle4 execution or commit followed. Task016 remains done; Task015 retains its original one start and30908 baseline, with no leaf dispatch/open-red acceptance. Local test-failure ledger remains7 and historical full-failure ledger6; this additional live-capture failure is recorded separately from the earlier unsafe-alias refusal. Wave3/4 convergence remains pending; U3-U7 remain unstarted.

## Approved observation-local collector optimization

The 2026-11-08 follow-up is limited to the existing dirty script, integration
test and this manifest. Task016 was already done. No backlog, start, checkpoint,
branch, index, commit, reviewer or lifecycle operation was performed.

Careful and freeze-scope modes remained active. ProposedAction: replace redundant
filesystem cmdlets with .NET metadata and independently scoped directory memos.
ActionRisk: moderate; ActionResult: applied under the supplied approval.
Original bytes remain in `bin\_u2j_perf_20261108_211219\*.before.raw`;
`before.json` records their hashes. No conflicting file locks were present.

`Get-RedSnapshotSafePath` now combines segments with .NET, checks attributes,
refuses reparse/device/8.3 aliases, and memoizes only verified existing ancestor
directories. Missing directories are not cached. Each worktree observation
creates its own memo; every policy fingerprint creates another fresh memo.
Leaf attributes are reused without skipping raw file-byte hashing. Native Git
NUL parsing, baseline blobs, two-observation equality and all policy checks
remain intact. DIRSUMMARY strips exactly one native `/`, validates an existing
nonreparse directory and excludes that record from policy-file hashing.
FILE trailing `/` and backslash negatives remain unchanged.

Optional `-SnapshotDiagnostics` emits bounded phase timings to stderr only.
Default snapshot/schema and simulation semantics remain unchanged.
Three device/short-directory/short-file controls were appended to the existing
metadata-refusal test; no old assertion or named case was removed.

### Scoped observations and retained failures

* Focused test-first native RED: two 8.3 alias outcome assertions failed; the
  device control already passed
* First post-edit focused/simulation runs exposed an optional `[ref]` default
  binding error; the argument was corrected without weakening any refusal
* Final focused race command: native0, 2/2 named PASS, 12.566s
* Initial full scoped race: native1, 731.380s, 59 RUN / 54 PASS / 5 FAIL /
  zero SKIP; 34 real-Git RUN / 32 PASS
* Both failed real-Git fixtures stopped at fixture `git add`, before Capture,
  with Windows `Filename too long`; three enclosing tests also failed
* The unchanged scoped selector passed with exclusive ignored TEMP
  `bin\_p86` after the required Go environment setup: native0, 811.401s,
  59 distinct named RUN/PASS, all 34 real-Git RUN/PASS, zero FAIL/SKIP
* Final simulation: native0, 257/257 assertions across 38 scenarios, 7.840s
* Final script AST parser: zero errors; changed Go formatting and owned diff
  whitespace checks passed

The scoped command retained `-timeout=30m -count=1 -race` and the exact
`^(TestUCS3_WaveSimClaimStart|TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract)$`
selector on `./tests/integration`. `-json` was diagnostic transport only.
No Git exclusion/config or dependency changed. These new follow-up observations
do not reset or reinterpret any historical failure ledger.

### Sole live public Capture

The supported public Capture ran from 2026-11-08T21:46:04.7505310Z to
2026-11-08T21:48:50.0705614Z: native0, 165.322s, no timeout.
Owned PID17132 was assigned to a private kill-on-close job before its thread
resumed. A 280-second monotonic/UTC deadline and five-second cleanup reservation
bounded the invocation; the parent exited and the job closed.
No AST extraction or dot-sourcing of the simulation main occurred.

| Capture phase | Seconds |
|---|---|
| Public bindings and observer storage | 1.068 |
| Baseline metadata / raw blobs | 1.719 / 6.193 |
| Observation1 policy before / Git metadata | 20.076 / 3.079 |
| Observation1 worktree hashing / policy after | 76.972 / 14.508 |
| Observation2 policy before / Git metadata | 14.935 / 2.726 |
| Observation2 worktree hashing / policy after | 7.494 / 13.041 |
| Independent equality / exclusive snapshot write | 0.355 / 0.244 |

Both observations measured 5,804 paths and independently verified 345 ancestor
directories. Snapshot size is 2,010,736 bytes, within the existing witness cap;
stdout was suppressed except for the success marker. Production baseline delta
is false. Root/task/baseline/HEAD bindings are the supplied repository root,
`211.015-T`, original `30908ecb8438c0b3e7fe8bd514fa3680e35d90e5` and unchanged
`7f3fd6694e67e962bec21544d344f10f4949e6fc`.
No before/after speedup claim is made: the older live observations were timeouts,
not completed comparable measurements.

### Immutable witnesses and stop boundary

Diagnostics below are exclusive ignored assets; their bytes are not rewritten.

| Witness | SHA-256 |
|---|---|
| `bin\tmp\u2j-perf-live-20261108-211219.json` | `a13653189eb3e06acd04bebcb5a876ecca60b9e41bc24ef7251251738bb2962b` |
| `bin\_u2j_perf_20261108_211219\verification.json` | `e6d5b45109f370a60e65327a449b477e02a36956b3f7404748e72fe8a28d6fd2` |
| `bin\_u2j_perf_20261108_211219\live-result.json` | `47ef631bb49e23ad4d23aa8d04d1d6bb2f65cf5d73e8e45c9337b0320d095ba7` |
| `bin\_u2j_perf_20261108_211219\live-stderr.raw` | `a6d4acd52fdd4e68c256f31cd80be3cd71f9d2c2617d0e2774eb471c8db66e2f` |
| `bin\_u2j_perf_20261108_211219\scoped-race-short-temp.jsonl` | `ee9f67e2f117c025e79c3b64ee34afcda362729982ff6e4966a794b60387f502` |
| `bin\_u2j_perf_20261108_211219\simulation-final.json` | `95734ca6abb5cce666eda5ddcbfc4cd8952176c4dca1ebd682b8bef71c92d4f1` |

`verification.json` contains the full closed named/real-Git census, every phase,
failed-observation paths/hashes, original backups and preserved-state hashes.
Final script SHA-256 is
`a03d88b1c90f1e50c075bd01599b7ad5af851379a531b3826d9078fac290659e`;
test SHA-256 is
`e296dd9bedd09d019f73df220c24606f6d415a2c35c922222d7d3e396aa2b086`.

The protected checkpoint remains byte-identical, SHA-256
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
The index remains byte-identical, SHA-256
`4d426e2769e81bea8e3fb55768bc1d7449ab66ef1a02da4c97a5ed7c1de894f1`.
Caller memories, the new caller checkpoint and final handoff are also unchanged.

STOP at implementation, scoped proof and diagnostic completion. No current
technical blocker remains for this bounded follow-up. This live capture is not
Task015 RED acceptance or a leaf post-state comparison. Its single start,
pending Wave3/4 gates and parent review/lifecycle authority remain preserved.
No compaction/archive candidate within this authorized active manifest qualifies.

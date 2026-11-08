---
title: U2j compiling assertion RED evidence
task: 211.016-T
feature: 211-F
shipment: 212-S
head: 465365263c1b2b1f87ad284342f31d957507498a
compilation: PASS
red_phase: CONFIRMED
workflow_updates: none
---

## Boundary

Harness-only execution under careful and freeze-scope posture. Only
`tests/integration/claim_start_wave_sim_contract_test.go` and this evidence
manifest were edited. The harness adds 583 lines and removes zero lines.
The existing test, evaluator, tracked fixture, skill, agent, and policy remain
unchanged. No implementation, stubs, child agents, full-suite test execution,
claim/start/status updates, labels, commits, branch/worktree changes, or pushes.
The parent owns subsequent labeling, commit, and build-feature dispatch.

The pre-existing caller changes to `.backlogit/hooks_queue.jsonl` and
`docs/memory/2026-11-08/autoharness-gsd-requirements-draft-memory.md` were left
untouched. Active `211.015-T`, its single historical start, and original
`red_baseline_sha = 30908ecb8438c0b3e7fe8bd514fa3680e35d90e5` were not changed.
No prior done task was reopened or restarted.

## Environment and commands

Every Go command dot-sourced `bin/tmp/212-go-env.ps1`: cached Go 1.24.0
windows/amd64, `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`,
`GOPROXY=off`, `GOSUMDB=off`, workspace-contained cached modules/build cache.
TEMP and TMP were overridden with fresh short owned ignored directories:

* Baseline compile: `bin/_u2j246fb0a4`
* Post-harness compile: `bin/_u2je665a0ee`
* Scoped race RED: `bin/_u2j326dfa14`

| Command | Native exit | Observation |
| --- | --- | --- |
| `go test -run=^$ -count=1 ./...` before edits | 0 | Baseline compiles |
| `go test -run=^$ -count=1 ./...` after edits | 0 | Whole repository including harness compiles |
| `go test -timeout=30m -count=1 -race -run '^(TestUCS3_WaveSimClaimStart\|TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract)$' ./tests/integration` | 1 | Genuine U2j assertion RED, no skips |
| `./bin/golangci-lint.exe run` | 0 | Pinned v1.64.8, no findings |
| `gofmt -l .` | 2 | Ignored cached Go/toolchain testdata contains intentional syntax errors; no repair attempted |
| `gofmt -l tests/integration/claim_start_wave_sim_contract_test.go` | 0 | Empty output, harness formatted |
| `git --no-pager diff --check` | 0 | No whitespace errors |

The race command executed natively on Windows, not a static-only fallback.
Exact scoped selector for downstream execution:

```powershell
go test -timeout=30m -count=1 -race -run '^(TestUCS3_WaveSimClaimStart|TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract)$' ./tests/integration
```

The scoped run reported zero `--- SKIP:` lines and zero `TestUCS3_` failure
lines. Existing replay still passes; RED belongs to U2j.
The full compile also discovers four pre-existing ignored scratch packages
under `bin/tmp/gotmp`; those reported `[no test files]`.

## Coverage

Five grouped subcases use temporary JSON passed through `-Fixture` to the
unchanged `scripts/wave-scheduler-sim.ps1`. Each replay retains all existing
fixture sections and all original 26 scenarios and 18 branch controls.
No tracked control was removed or weakened.

* Caller task/hook writes from the recorded `30908ecb` counterexample, caller
  documentation, and a clean tree with identical independent pre/post records
  expect `RED_DELIVERABLE_ACCEPTED` and no generic loop
* On both caller-modified paths: hash changes, worktree/index mode changes,
  index blob changes, staging/unstaged status changes, and deletions refuse
* New untracked or staged config/docs/test files and removal of an already
  caller-untracked file refuse; current status alone cannot hide a deletion
* Tracked/untracked leaf production Go and pre-dispatch production Go refuse,
  including a mixed production/docs delta without an old baseline path hint
* Missing clean/dirty snapshots, bad schema/root/task/baseline/HEAD, short or
  nonhex SHA-256, invalid index metadata, path escapes, duplicate paths, and
  absent/malformed/mismatched post-leaf metadata fail closed

Real isolated Git fixtures commit a local baseline, change task/hook bytes
as caller writes, add caller-untracked docs, and verify index OID/mode using
Git independently. Twelve cases exercise capture/compare once helpers land:
zero writes, task/hook one-byte changes, staging, index chmod, tracked and
untracked deletions, new test/config/docs files, and leaf/pre-dispatch production
Go. Fixture evidence resides inside the fixture's `.git`, not the measured
surface. No repository-root Git mutation is performed.

The missing helper declarations currently fail named assertions through
PowerShell function-AST discovery, not missing Go symbols, command-not-found
errors, artificial failures, or skips. After declarations land the same cases
execute actual capture and compare bodies. The test invokes:

```powershell
Get-RedDeliverablePreDispatchSnapshot -WorkspaceRoot $root -TaskID '211.015-T' -RedBaselineSHA $baseline
Compare-RedDeliverablePreDispatchSnapshot -PreDispatchSnapshot $pre -PostLeafSnapshot $post
```

The snapshot wire record is schema
`red-deliverable-predispatch-snapshot/v1`, workspace/task/baseline/HEAD metadata,
and `paths` containing `path`, `worktree_exists`, `worktree_sha256`,
`worktree_mode`, `index_mode`, `index_blob_oid`, `unstaged_state`,
`staged_state`, and `untracked`. Branch observations use
`predispatch_snapshot` and independently captured `post_leaf_state`.
Compare returns an object with `Outcome`.

A bounded amendment-resume scenario preserves nine done members and the
started flag on active `211.015-T`, keeps `211.016-T` queued at admission,
and expects prerequisite completion in Wave 3, RED completion/registration
in Wave 4, deferral through Wave 8, and GREEN closure in Wave 9. Two serial
prefix fixture members place the transition at its approved wave numbers;
they are not new backlog tasks. Source-boundary assertions cover the coupled
Step 0.5, caller admission/dispatch/return, and P-002.6 alignment.

## Raw RED outcomes

These are decisive native stdout excerpts, not synthetic failure markers:

```text
--- FAIL: TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract (59.96s)
    --- FAIL: TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract/caller_claim_bookkeeping_unchanged_by_leaf_accepted (7.79s)
            native_exit=1
            FAIL red_branch/caller-claim-bookkeeping-zero-leaf-write :: branch outcome
                 expected: RED_DELIVERABLE_ACCEPTED
                 actual  : RED_DELIVERABLE_DELTA_OUT_OF_SURFACE
            FAIL red_branch/caller-docs-zero-leaf-write :: branch outcome
                 expected: RED_DELIVERABLE_ACCEPTED
                 actual  : RED_DELIVERABLE_DELTA_OUT_OF_SURFACE
            WAVE_SIM_FAIL: 182/184 assertions PASS, 2 FAIL
        --- FAIL: TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract/caller_claim_bookkeeping_unchanged_by_leaf_accepted/paused_amendment_resume_and_coupled_sources (3.03s)
                WAVE_SIM_FAIL: 42/52 assertions PASS, 10 FAIL
    --- FAIL: TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract/leaf_mutates_caller_modified_path_refused (41.58s)
            native_exit=0
            WAVE_SIM_OK: 206/206 assertions PASS across 26 scenario(s)
        --- FAIL: TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract/leaf_mutates_caller_modified_path_refused/real_git_capture_compare (32.77s)
    --- FAIL: TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract/leaf_mutates_production_go_refused (3.42s)
            FAIL red_branch/predispatch :: branch outcome
                 expected: RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED
                 actual  : RED_DELIVERABLE_ACCEPTED
            FAIL red_branch/mixed :: branch outcome
            WAVE_SIM_FAIL: 184/186 assertions PASS, 2 FAIL
    --- FAIL: TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract/missing_or_mismatched_predispatch_snapshot_refused (3.57s)
            FAIL red_branch/missing-clean :: branch outcome
                 expected: WAVE_RED_MAPPING_UNRESOLVED
                 actual  : RED_DELIVERABLE_ACCEPTED
            FAIL red_branch/missing-dirty :: branch outcome
                 expected: WAVE_RED_MAPPING_UNRESOLVED
                 actual  : RED_DELIVERABLE_DELTA_OUT_OF_SURFACE
            WAVE_SIM_FAIL: 197/216 assertions PASS, 19 FAIL
RED_NATIVE_EXIT=1
```

The real-Git failure names append `/zero`, `/task-byte`, `/hook-byte`,
`/task-stage`, `/index-mode`, `/hook-delete`, `/untracked-delete`,
`/new-test`, `/new-config`, `/new-docs`, `/production-leaf`, and
`/production-predispatch` to the full `real_git_capture_compare` path above.
Each reports the actual absent helper declarations.
The outside-surface fixture-only group passes against the old evaluator;
it remains a regression boundary, not an invented RED assertion.

## Artifact hashes and raw log retention

Harness SHA-256:

```text
4ca9c35d2585bf84108e58350bc48d3494d7b7b4910103b7302cc905affd8804
```

Full unabridged native output was automatically retained by the tool in ignored
workspace files. These are evidence references, not additional deliverables:

| Raw artifact | SHA-256 |
| --- | --- |
| `bin/tmp/1794108513412-copilot-tool-output-7128-daa64bee-b1c3-4d52-8e7d-cdcab17df241.txt` | `3a6d0f9ba8dff80359ad4d4f9df908abeb6424f34aa792dbe31182995e37d784` |
| `bin/tmp/1794108460136-copilot-tool-output-7128-0f342ec2-27a1-4cb1-90e8-f1d3ccc2487e.txt` | `1da8ce71dff0e7b6803916f7e6d3265cce6aeec3634f8201a1feca9891da8fda` |

Protected/caller artifact SHA-256 values:

```text
677aac3b994af3e3290546e1e1761efbe1961951adc10cc43975114ffac3b928 scripts/wave-scheduler-sim.ps1
7207e66846063a852f061d214309d285c905801ec07d23930a94c57e0a269a29 tests/simulation/wave-scheduler-contract.json
2b4d48ecd31b6e4565e2ccd5503bb91f5f97c45e4b1e9a6cd3cdbac03aca3cb2 .github/skills/build-feature/SKILL.md
d7ec11e499ca061c2bf6e02e0459e24db2262bdb9b80a7e92e6c63c9048014cd .github/agents/_ship.agent.md
59c96e1cde24865c8d7f008c79e8718e127988cf908754ef19394f321d6b0b4d .github/policies/workflow-policies.md
ad7a4f7c381f2ed9bc7ab2a7a144b1de9b9bb033d8480843034e6070dd034ef5 .backlogit/hooks_queue.jsonl
70570ab0b0bd50696a2172e1eddd8e8498dc921ec4cf82784baa41ff71edfb7a docs/memory/2026-11-08/autoharness-gsd-requirements-draft-memory.md
1e8e996963b87f15867ece9a30fd1996cda7fa98aedd6feaf0c03aafd50a6119 .backlogit/queue/211.015-T.md
5769286c7b4c355ebb293eed7aa1c354582d5831701a0980e6e283dc50d8a1fb .backlogit/logs/211.015-T.jsonl
```

Harness encoding was verified as UTF-8 with zero CR bytes. This manifest is
UTF-8 LF. Repository-wide formatting is not claimed clean; native exit 2 and
the raw diagnostic artifact are retained. Stop point: compiling functional
RED, with no evaluator or workflow-state changes.

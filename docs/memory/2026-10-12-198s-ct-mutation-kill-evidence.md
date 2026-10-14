# 198-S (CT) R6 mutation-kill evidence: T1 (198.001-T) and T3 (198.003-T)

Recorded: 2026-10-12 (UTC-7 run window 15:24 to 15:27 local, i.e. 22:24 to 22:27 UTC). Branch: `feat/ct-restore-test-suite-health-and-runtime-diagnostics`. HEAD at run: `80114f4d`. Delivering commit under test: `9762e75f`. Governing text: plan Amendment A1.4 (ruling `a1_4_operator_ruling: T1=A; T3=A`), conditions R1 through R6.

Scratch evidence (git-ignored): `logs/_ct-mutation/evidence-r6.md`, `t1-mutant.log`, `t3-mutant.log`, `t1-control.log`, `t3-control.log`, `golist-after.txt`. No tracked file was edited and nothing was deleted.

## Executor

* Executor: GitHub Copilot CLI general-purpose sub-agent dispatched by Ship (R6 step 1). Its model was not exposed to it in-session, and no dispatch pin was supplied, so the model is not recorded here. A reviewer or the operator should spot-check the `--- FAIL:` lines below, because the same model family wrote the tests.
* Ship verified the executor's scratch evidence independently: mutant log counts (10 and 3 `--- FAIL:` lines), that each mutant diff is exactly one removed and one added line, and that `git status --short` is empty.

## Safety posture (R6 step 1; safety-modes careful, then freeze-scope)

| Risk | Containment |
|---|---|
| A mutant leaks into a tracked production file | Mutants are copies under `logs/_ct-mutation/`; originals only read |
| Overlay key mismatch, so the overlay silently does not apply | Keys use `go list -f {{.Dir}}` casing; zero intended failures would halt (none occurred) |
| A stray `.go` file enters the build | `_ct-mutation` starts with `_`, so `./...` skips it; `go list` identical before and after |
| Deletion of scratch artifacts (Principle VII) | None run; no `Remove-Item`, `git clean`, `reset`, `checkout`, or `stash` |
| Full-suite run and circuit-breaker timeout | Only the two named tests ran, one invocation per task |

Freeze-scope: write access `logs/_ct-mutation/` only; read access to `internal/core/gate_transition.go` and `internal/release/release.go`.

## Mutations (R6 step 2)

T1, `appendGateEvidence` in `internal/core/gate_transition.go` (line 629): `defer unlock()` replaced by `unlock()` immediately before `return ws.appendGateEvent(ctx, id, eventType, delta)`. Diff: one removal, one addition. Go accepted the `.mutant` replacement name, so no rename was needed.

T3, `Client.Latest` in `internal/release/release.go` (line 55): `c.LatestRelease(ctx)` replaced by `c.LatestRelease(context.Background())`. Diff: one changed line.

Overlay files: `t1-overlay.json` and `t3-overlay.json`, no byte-order mark, absolute original path in `go list` casing.

## Mutant runs (R6 step 3)

| Task | Command | Exit | `--- FAIL:` lines | Intended assertion |
|---|---|---|---|---|
| T1 | `go test -count=10 -run '^TestUT1_ConcurrentSameItemCounterContract$' -overlay <abs>\logs\_ct-mutation\t1-overlay.json ./internal\core` | 1 | 10 of 10 | 10 of 10 `duplicate counter 1 durably persisted across concurrent appendGateEvidence calls` |
| T3 | `go test -count=3 -run '^TestUT3_ClientLatestRespectsContextTimeout$' -overlay <abs>\logs\_ct-mutation\t3-overlay.json ./internal\release` | 1 | 3 of 3 | 3 of 3 `An error is expected but got nil.` at `release_test.go:102` |

No panic, build error, timeout, or `no tests to run` appeared. Observed kill ratio is all-fail for both tasks, so `R3_FINDINGS` is zero.

T1 first intended-assertion block, verbatim:

```text
--- FAIL: TestUT1_ConcurrentSameItemCounterContract (1.21s)
    gate_evidence_formal_test.go:499:
        Error Trace:  C:/Source/GitHub/backlogit/internal/core/gate_evidence_formal_test.go:499
        Error:        Should be false
        Test:         TestUT1_ConcurrentSameItemCounterContract
        Messages:     duplicate counter 1 durably persisted across concurrent appendGateEvidence calls
```

T3 intended-assertion block, verbatim (all three occurrences share this form):

```text
--- FAIL: TestUT3_ClientLatestRespectsContextTimeout (2.01s)
    release_test.go:102:
        Error Trace:  C:/Source/GitHub/backlogit/internal/release/release_test.go:102
        Error:        An error is expected but got nil.
        Test:         TestUT3_ClientLatestRespectsContextTimeout
```

## Unmutated controls and -list (R6 step 4)

| Command | Exit | Result |
|---|---|---|
| `go test -count=1 -v -run '^TestUT1_ConcurrentSameItemCounterContract$' ./internal\core` | 0 | `--- PASS: TestUT1_ConcurrentSameItemCounterContract (6.51s)` |
| `go test -count=1 -v -run '^TestUT3_ClientLatestRespectsContextTimeout$' ./internal\release` | 0 | `--- PASS: TestUT3_ClientLatestRespectsContextTimeout (0.03s)` |
| `go test -list` (T1 anchored name, `./internal\core`) | 0 | lists exactly `TestUT1_ConcurrentSameItemCounterContract` |
| `go test -list` (T3 anchored name, `./internal\release`) | 0 | lists exactly `TestUT3_ClientLatestRespectsContextTimeout` |

## No production delta and unchanged module (R6 step 5)

* R2 (`git --no-pager diff --name-only c5758918 -- '*.go' go.mod go.sum ':(exclude)*_test.go'`, and `git ls-files --others --exclude-standard -- '*.go' ':(exclude)*_test.go'`): both print nothing and exit 0.
* R2 positive control (`git --no-pager diff --name-only c5758918 -- '*.go'`): lists `internal/core/gate_evidence_formal_test.go`, `internal/core/log_state_restore_test.go`, `internal/release/release_test.go`.
* R4 (`git --no-pager diff --quiet 9762e75f -- internal/core/gate_evidence_formal_test.go internal/release/release_test.go`): exit 0.
* `go list -mod=readonly ./...`: 41 lines before and after, identical, with no `_ct-mutation` entry.
* `git status --short` after the run: empty.

## R5 record

Task 198.001-T (T1):

* Compilation: PASS
* Red Phase: MUTATION-KILL-CONFIRMED (P-004/FC-1/FC-2.4/FC-3 relaxed by the operator, a1_4_operator_ruling 2026-10-12T17:35:08Z)
* Delivered-tree result: GREEN (expected; tests-only stabilization)

Task 198.003-T (T3):

* Compilation: PASS
* Red Phase: MUTATION-KILL-CONFIRMED (P-004/FC-1/FC-2.4/FC-3 relaxed by the operator, a1_4_operator_ruling 2026-10-12T17:35:08Z)
* Delivered-tree result: GREEN (expected; tests-only stabilization)

Correction: harness-architect's earlier comments on both tasks (commit `9762e75f`, 2026-10-11) used the bare token `Red Phase: CONFIRMED via mutation-kill`. Under this ruling that token means mutation-kill, not a red observed on the committed harness. Ship appended a correcting comment to each task, as R5 requires.

Compliance: this note is audit evidence, not an approval gate. The PR body carries the compliance annotation (198.001-T, 198.003-T, A1.4, INC-P002-131S-148F, INC-P002-152F-134S, and the two open Stage process events in A1.7 item 4).

R3_FINDINGS: 0

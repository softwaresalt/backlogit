---
title: "212-S U6 replacement retry evidence: corrected fixture and lint blocker"
shipment_id: "212-S"
feature_id: "211-F"
task_id: "211.006-T"
---

## Boundary and implementation

The M22 Stage amendment authorized only `internal/cli/mcp_daemon.go`,
`internal/cli/mcp_session_shim_test.go`, and this note. Shipment HEAD remains
`5d8ff38034f5241731767f88924b3f4bd5dde937` on the existing shipment branch.
No task lifecycle, backlog, checkpoint, baseline, or suite counter changed.
No full suite, staging, commit, push, PR, or merge occurred.

The uncommitted production patch retries only regular-file identity changes,
with four attempts and 2 ms between attempts, closing replaced handles before
retry. Link and nonregular validation remains immediate. Prewarm routes only
the exhausted `changed while opening` validation suffix to its existing poll.

## Original RED and GREEN observations

Every Go invocation used `bin/tmp/212-go-env.ps1`: workspace-local Go 1.24.0,
module/build/lint caches and TEMP/TMP, readonly modules, and offline resolution.
Engram CLI workspace binding was verified. Indexed graph coverage was incomplete;
the exact declared source was inspected directly. Intercom was unavailable.

Focused command:

```text
go test -timeout=30m -count=1 -race -v -run '^TestU6_ConcurrentRegularSidecarReplacement$' ./internal/cli
```

The first pre-production fixture run failed its writer assertion on Windows
access denial (5.192 s package time); this was not accepted as reader RED.
Before production changed, the fixture was refined to retry classified
`ErrWriteNotApplied` publications within its original deadline.
The next run genuinely failed `U6_REGULAR_REPLACEMENT_RETRY` with
`ErrValidation: ...mcp-daemon.json changed while opening` (3.853 s package,
0.55 s named test). No production hook, skip, tag, or replacement stub was used.

After the production patch, a focused run concurrent with compile-only timed
out at the fixture's 15-second deadline (18.827 s package).
The unchanged isolated rerun passed (12.381 s package, 10.85 s named test):
256 replacements, 3,531 successful reads, 51 writer-contention retries.

```text
go test -timeout=30m -count=1 -race -v -run '^TestU6_' ./internal/cli
```

This exact task harness passed nonvacuously in 47.098 s. Named passes included
`TestU6_ConcurrentRegularSidecarReplacement` (256 replacements, 4,266 reads,
86 contention retries, 14.79 s), all three
`TestU6_PrewarmAndLazyAutoStartColdRaceConvergence` subtests,
`TestU6_MissingPinnedBinaryRefusalAndCLIWiring`,
`TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival`,
and both matched `TestU6_S1_` tests. The mixed-cold-start witness reported four
callers converging to PID 5612; disconnect survival and owned-daemon stop passed.

```text
go test -timeout=30m -count=1 -race -run '^(TestU6_|TestU5a_|TestU4_CrossProcess|TestRegistryParity_)' ./internal/cli
```

The required combined regression failed in 53.467 s. The new replacement test
finished only 255 of its required 256 publications before the deadline
(15.05 s named test). This is not a reader validation failure or reliable GREEN.

## Original stop disposition

One production implementation was attempted. There were two pre-production
focused probes and four post-production probes: two focused runs, one exact
task harness, and one required combined regression. Repository compile-only
`go test -run=^$ -count=1 ./...` passed before and after production edits across
all 46 packages. `git diff --check` passed.

The new fixture's sustained readers can starve Windows atomic publication;
its fixed publication count plus deadline adds an unintended writer-throughput
requirement. Execution stopped rather than weakening a post-RED assertion or
changing writer behavior outside the approved delta. A separately authorized
fixture correction and renewed RED/GREEN evidence are needed before acceptance.
Cold-race reliability is not established by the single successful task harness.

Vet, lint execution, `gofmt -l .`, and the baseline EOL test were not reached.
Exact lookup resolved `TestU175_001_BaselineEol` to `./tests`.
PATH and `bin/golangci-lint.exe` both reported v1.64.8 built with go1.26.5;
no lint result or compatible-pinned-lint claim is made.
The two Go files and this note remain unstaged and uncommitted.

## Authorized fixture correction

The narrow follow-up used one fixture correction cycle. No production contract
or final production bytes changed from the reviewed patch. The fixture now
requires one genuine atomic identity replacement rather than 256 publications:
one real replacement is the semantic contract, not writer performance within
15 seconds. The existing deadline and classified unpublished-write retries
remain. Readers yield for 1 ms after closing their handles, leaving Windows a
publication opportunity. Reads before, during, and after publication validate
the complete record; the named identity-change witness remains mandatory.

A deterministic source-shape supplement parses the real production functions
without comments. It asserts four attempts, handle closure before the guarded
2 ms sleep, regular-only identity retry, immediate link/nonregular returns,
and Prewarm's exhausted-mismatch suffix exception. A real directory-sidecar
test also requires `ErrValidation`. This supplements, rather than replaces,
real concurrent atomic publication. There are no hooks, new exported
production symbols, skips, tags, or replacement stubs.

## Renewed RED and GREEN evidence

All commands remained sequential and used the existing workspace-local Go
wrapper. Engram verified the existing branch/workspace binding; its exact-file
symbol query returned no indexed U6 symbols, so the declared source was read
directly. Intercom remains unavailable.

The reviewed production file was safely copied to
`bin/tmp/212-u6-reviewed-mcp_daemon.go.backup`. An `apply_patch` reversal
temporarily restored exact pre-fix production; `git diff --exit-code` for that
file returned zero before the RED probe:

```text
go test -timeout=30m -count=1 -race -v -run '^TestU6_(ConcurrentRegularSidecarReplacement|RegularReplacementRetrySourceContract|NonregularSidecarRefusal)$' ./internal/cli
```

Exit 1, package time 3.524 s. The genuine publication test passed in 0.05 s:
one replacement, five successful reads, zero writer-contention retries.
Directory refusal passed. `TestU6_RegularReplacementRetrySourceContract` failed
in 0.01 s on the named assertion:

```text
U6_REGULAR_REPLACEMENT_RETRY: regular identity mismatch needs four bounded attempts
```

This renewed RED is explicitly a source-contract assertion, not a newly
observed behavioral reader failure. The original behavioral reader RED above
remains `changed while opening`, not writer failure.

Production was restored using `apply_patch`; both the backup and restored
file had SHA-256
`CC6D715E12100D5681D6BB52F972AE1F286DB22E7E226BF3CCEBE4A43B12DBC1`.
No history or configuration mutation was used.

```text
go test -timeout=30m -count=1 -race -v -run '^TestU6_' ./internal/cli
```

Exit 0, package time 29.648 s, nonvacuously. The genuine replacement test passed
in 0.03 s with one replacement, four successful reads, and zero contention
retries. Both new contract/refusal tests passed. All original U6 tests and
subtests passed, including mixed cold convergence to PID 42736 and real stdio
disconnect survival with daemon PID 24048.

```text
go test -timeout=30m -count=1 -race -run '^(TestU6_|TestU5a_|TestU4_CrossProcess|TestRegistryParity_)' ./internal/cli
```

Exit 0, package time 45.134 s.

```text
go test -timeout=30m -count=3 -race -v -run '^TestU6_(ConcurrentRegularSidecarReplacement|RegularReplacementRetrySourceContract|NonregularSidecarRefusal|PrewarmAndLazyAutoStartColdRaceConvergence)$' ./internal/cli
```

Exit 0, package time 39.932 s. All three fresh cold-convergence fixture runs
passed (13.58 s, 12.64 s, 11.93 s); their mixed-caller singleton PIDs were
60072, 45756, and 34916. Each real replacement run published once with four
successful reads and zero contention retries (0.03 s, 0.06 s, 0.04 s).
All source-contract and nonregular-refusal repetitions passed under race.

## Current verification blocker and disposition

`go test -run=^$ -count=1 ./...` passed before and after the correction across
all 46 packages. Final CLI package compile-only time was 0.978 s.
`go vet ./...` passed with no findings.

The wrapper invocation of `golangci-lint run` exited 4:

```text
level=error msg="Timeout exceeded: try increasing it by passing --timeout option"
```

No finding or duration was reported. No timeout or budget was increased.
The wrapper's PATH linter reports v1.64.8 built with Go 1.26.5, not the pinned
Go 1.24.0 toolchain. This mismatch was checked, but is not proof that the
timeout was caused by incompatibility. The existing-cache-only fallback probe
was:

```text
go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 version
```

It exited 1 with `module lookup disabled by GOPROXY=off`; the offline fallback
is unavailable. No packages were installed and no configuration changed.
Execution stopped at this quality-gate blocker. `gofmt -l .` and
`TestU175_001_BaselineEol` were not reached in this follow-up; no success is
claimed for them or for lint.

No full suite, driver, receipts, Ordinal11 budget/counter change, backlog or
task lifecycle mutation, staging, commit, push, PR, or merge occurred.
Only the two authorized U6 files and this evidence note are changed, unstaged,
and uncommitted on the original branch.

## Ship quality-gate completion

Ship ran the existing linter with an explicit analysis timeout:
`golangci-lint run --timeout=5m`, through the same Go environment wrapper.
It exited zero with no findings. This did not change the Ordinal 11 suite
budget or any historical counter.

`gofmt -l .` traversed ignored module/toolchain cache content and exited 2.
No cache content was changed. A separate batched `gofmt -l` check over all
1,400 Git-tracked Go files passed with empty output, covering the complete
first-party format surface without treating downloaded dependencies as source.
`TestU175_001_BaselineEol` passed in `./tests` (1.476 s), checking 1,746 files
with zero index or worktree EOL violations.

The scoped report-only review covered the complete U6 diff against
`5d8ff38034f5241731767f88924b3f4bd5dde937`. Outcome: READY; P0=0,
P1=0, P2=0, P3=0. No review artifact or backlog item was created.
Runtime follow-up remains the authorized unfiltered convergence and later
branch-wide runtime verification, not a substitute for either gate.

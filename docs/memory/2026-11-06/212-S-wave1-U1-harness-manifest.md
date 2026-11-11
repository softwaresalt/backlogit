---
title: Shipment 212-S wave 1 U1 harness handoff
date: 2026-11-06
shipment_id: 212-S
feature_id: 211-F
task_id: 211.001-T
wave: 1
compilation: PASS
red_phase: CONFIRMED
served_root_handoff: PASS
commit_status: verified-for-scaffold-commit
---

## Scope and scaffold

Leaf `harness-architect` only. Ready set: `211.001-T` (U1).
The operator supplied seven frozen serial waves:
U1 -> U2 -> U3 -> U4 -> U5 -> U6 -> U7.
Waves 2-7 remain unscaffolded and unstarted by this leaf.
The closed exempt member `211.007-T` is outside this invocation.

Intake reused:
`.backlogit/reconcile/212-S-pre-20261106T000529Z.md` (`PROCEED`).
The confirmed resume note supersedes the earlier lifecycle summary:
seven tasks are claim-assigned, with zero valid starts.
No claim, move, or start was performed by this leaf.
Prior compilation and `WAVE_SIM_OK200/200` evidence were not substitutes
for the fresh Go 1.24 compile and RED observations below.

Harness: `internal/mcp/multi_session_shape_test.go`.
SHA-256:
`3015EAF11D57B6F28B6ED0B1E60F55E8282AEA58197C5C01C2996CBCF6C840EB`.
Three AST scenarios assert five initialized sentinels, four coordinator
types and busy-response JSON tags, and four exact function/method type
signatures. No test references an undeclared production identifier.
No temporary stub body or coordinator behavior is frozen.
The feature-root witness shares these U1 assertions; it is not evidence
of completed multi-session behavior or coverage for later tasks.

## Exact executable task boundary

Run from the repository root in PowerShell:

```powershell
& ./scripts/verify-u1-harness.ps1
```

This is the task's complete runnable command, including environment setup
and a named-test vacuity guard. Its native Go invocation is exactly:

```powershell
go test -timeout=30m -count=1 -v -run '^TestU1_' ./internal/mcp
```

The wrapper sets the cached workspace Go 1.24.0 `GOROOT`, prepends its
`bin` to `PATH`, sets `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`,
`GOPROXY=off`, and `GOSUMDB=off`, and uses the existing workspace
`bin/_authoring_gocache`, `bin/_authoring_gomodcache`,
`bin/_authoring_gopath`, `bin/_authoringtmp` (TEMP and TMP), and
`bin/_authoring_golangci_cache`. It invokes that toolchain's `go.exe`
directly, not the host Go executable.

The guard requires the exact top-level test and all three named subtests
to RUN and finish PASS or FAIL, plus the explicit MCP package summary.
Missing or skipped tests, no-tests output, build failure, panic,
unexpected package, or unexpected native exit are rejected with exit 2.
Native exit 0 additionally requires every named test to PASS.
Native exit 1 requires the named top-level FAIL and remains exit 1.
`NAMED_TEST_GUARD=PASS` means non-vacuous execution, not GREEN.
This command can succeed on genuine GREEN after declarations land;
it is not a RED-only observer and never converts RED to exit 0.
No production changes were made to exercise its future GREEN path.

The feature-root declaration witness uses the same guard and environment:

```powershell
& ./scripts/verify-u1-harness.ps1 -FeatureRoot
```

Its native selector is `^TestFeature211_`, with the same explicit
`./internal/mcp`, `-timeout=30m`, `-count=1`, and `-v` flags.

## Fresh observed verification

| Boundary | Exact command | Native exit | Result |
| --- | --- | --- | --- |
| Compile only | `go test -run=^$ -count=1 ./...` | 0 | PASS |
| U1 RED | `& ./scripts/verify-u1-harness.ps1` | 1 | CONFIRMED, guard PASS |
| Feature-root U1 RED | `& ./scripts/verify-u1-harness.ps1 -FeatureRoot` | 1 | CONFIRMED, guard PASS |
| Vet | `go vet ./...` | 0 | PASS |
| Supported pinned lint | `& ./bin/golangci-lint.exe run --timeout=5m --out-format=json` | 0 | PASS, zero issues |

All Go gates used the wrapper's cached workspace Go 1.24 environment.
`go version` reported `go version go1.24.0 windows/amd64`.
The compile-only MCP output was:

```text
ok   github.com/softwaresalt/backlogit/internal/mcp  0.583s [no tests to run]
COMPILE_NATIVE_EXIT=0
VET_NATIVE_EXIT=0
```

No-tests output is intentional only for the compile-only gate, never
accepted by the task or feature-root guard.

U1 executed `TestU1_MultiSessionCoordinatorSourceShape`; the feature-root
selector executed `TestFeature211_MultiSessionCoordinatorSourceShape`.
Both produced all three expected scenario failures:

* `daemon_and_admission_sentinels`
* `coordinator_types_and_busy_json_tags`
* `exact_coordinator_signatures`

Actual assertion output included:

```text
ErrDaemonBinaryMissing sentinel initialized with errors.New is not declared in internal/errors/errors.go
OperationCoordinator defined type is not declared in coordinator.go
Execute receiver="*OperationCoordinator" (context.Context,*mcpserver.MCPServer,string,mcplib.CallToolRequest,mcpserver.ToolHandlerFunc) (*mcplib.CallToolResult,error) is not declared in coordinator.go
```

The missing production file was reported through `testing.T.Errorf`,
followed by missing-type and missing-signature assertions. Both native
runs compiled and assertion-failed, without build failure or panic.
Their actual terminal summaries were:

```text
--- FAIL: TestU1_MultiSessionCoordinatorSourceShape (0.00s)
FAIL github.com/softwaresalt/backlogit/internal/mcp 0.496s
GO_TEST_NATIVE_EXIT=1
NAMED_TEST_GUARD=PASS

--- FAIL: TestFeature211_MultiSessionCoordinatorSourceShape (0.00s)
FAIL github.com/softwaresalt/backlogit/internal/mcp 0.492s
GO_TEST_NATIVE_EXIT=1
NAMED_TEST_GUARD=PASS
```

Both wrappers exited 1, preserving genuine RED for downstream builds.
No repository-wide test-running RED command was used.

## Served-root handoff

Prior verified PASS, reused without rebinding: workspace `.`; storage `.backlogit`;
database `.backlogit/backlogit.db`.
Two independent MCP attestations agree:

* `backlogit_get_metadata_catalog` returned the current repository workspace,
  its `.backlogit` storage root, and its database under that storage
* `backlogit_query_sql` ran
  `SELECT name, file FROM pragma_database_list WHERE name = 'main'`
  and returned `main` at the same `.backlogit/backlogit.db`

No rebinding, alternate storage, or direct database mutation occurred.

## Superseded lint blocker and verified remedy

The earlier `blocked-lint` handoff is superseded, not a current blocker.
CI pins Go 1.24 and golangci-lint v1.64.8. The supported cached Go 1.24.0
with `bin/golangci-lint.exe` passed again in this invocation:

```text
{"Issues":[],"Report":{...}}
LINT_NATIVE_EXIT=0
```

This corroborates the parent's full captured result in
`logs/diagnostics/212-S-ci-v1-go124-attempt1.json`.
The prior host Go 1.27/v1 export-data failure and v2/Go 1.27 findings
were not rerun. Both linter versions enable errcheck and staticcheck;
we make no blanket claim that the v2 findings are false positives.
No suppressions, unrelated lint fixes, or production edits were needed.
Deferred captures `C6E224B8`, `46BD6AEE`, and `259A4F71` remain untouched.

Read-only formatting verification covered 1387 unique tracked/new Go
source files in bounded batches: zero findings, exit 0.
An initial all-files invocation hit a Windows process-launch error;
bounded batches resolved that command transport issue.
Ignored generated/cache-tree gofmt findings are not source drift.
No generated files were formatted, and no blanket repository-wide
format PASS is asserted.

Parent-proved Engram daemon/status health and empty bound memory were
reused. Exact-file reads and literal U1 heading lookup used direct-tool
exemptions; no structural discovery or raw broad search was needed.
No managed Engram state was edited. Internal material stayed local.
Intercom tools were unavailable; milestones were reported here.

## Persisted state and stop boundary

Official `backlogit_update_item` repairs only U1's implementation notes:
the executable command, guard contract, fresh evidence, and superseded
lint blocker. The existing `harness-ready` label is retained.
Acceptance criteria, planning, and lifecycle fields are not changed.

The scaffold commit is restricted to the unchanged AST test, this
manifest, and `scripts/verify-u1-harness.ps1`. The task update remains
unstaged for the parent, along with all pre-existing claim, stash,
hook, checkpoint, memory, and recovery-note state.
The protected test SHA-256 above was reverified unchanged.
No production declarations, stubs, U2+ scaffolding, other-task labels,
nested delegation, branch change, push, or PR was performed.
Stop at assertion RED; the parent owns implementation and recovery state.
This leaf does not invoke `build-feature` or full `verify-workspace`.

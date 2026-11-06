---
title: 212-S Wave 1 U1b harness manifest
date: 2026-11-06
shipment_id: 212-S
feature_id: 211-F
task_id: 211.008-T
status: paused-before-production
---

## Scope and provenance

* Leaf execution of `.github/skills/harness-architect/SKILL.md`; no agents launched
* Ready set: only `211.008-T` / U1b, harness-required and non-exempt
* Governing evidence: official task acceptance criteria, amended plan U1b/D10/M3,
  and `stage-212-s-c2cfe06e-u1b-amendment.md` in this directory
* Stage artifacts were already committed unchanged at `a3fed78f`; the caller
  attested all 14 final-handoff hashes and current served roots
* M8 remains frozen; U1 is done; U1b remains queued with no dependencies;
  U2-U7 remain claim-assigned with zero starts, per the caller's admission
* No claim, start, move, push, PR, plan edit, or future-task scaffolding
* Existing U2/U3/U4 failures are not used as U1b evidence

## Harness mapping

`internal/cli/mcp_daemon_lock_test.go` adds only
`TestU1b_NestedRuntimeRootUsesCanonicalAbsolutePathForReparseValidation`.
All pre-existing assertions are preserved. No production file or stub changed.

The test allocates all temporary directories before sequential `t.Chdir`.
The lifecycle subcase exercises exported default-options Acquire, Inspect,
release, and Stop with stale metadata, first without CWD runtime files and then
with unrelated benign decoys. Nested phase tests allow the expected initial
assertion failure without suppressing subsequent fixture checks.

The security subcase creates both lock-leaf and held-lock sidecar-leaf fixtures.
Windows uses non-privileged `cmd /c mklink /J`, with required successful creation,
`Lstat` ModeSymlink zero, and `core.IsSymlinkOrReparsePoint` true. POSIX uses
`os.Symlink`; no Windows-only imports were added. There are no skip paths.
Cleanup removes fixtures and explicitly asserts release errors.

Both entrypoints refuse each malicious leaf with `ErrValidation`, not
`ErrDaemonAlreadyRunning`. Directory contents and sentinel bytes remain intact.
Observed `executedRefusals=4 skippedFixtures=0`; both fixture subtests passed.

## Exact runnable scoped command

Run from the repository root in PowerShell:

```powershell
pwsh -NoProfile -File ./scripts/verify-u1b-harness.ps1
```

The wrapper sets every pinned environment value below and runs:

```powershell
& "$env:GOROOT/bin/go.exe" test -json ./internal/cli -count=1 -timeout=30m -run '^TestU1b_'
```

GREEN requires native exit zero, exactly one named top-level test PASS and
package PASS, the four-refusal/zero-skip coverage event, and no fail or skip
events. A missing selector match, missing test, malformed JSON, build failure,
or skipped fixture cannot produce GREEN. The guard remains RED (exit 1) until
the production repair makes the actual named test pass.

## Verification environment and results

Every Go invocation used the cached Go 1.24.0 executable and this environment:

```powershell
$workspace = (Get-Location).Path
$env:GOROOT = Join-Path $workspace 'bin/_authoring_gomodcache/golang.org/toolchain@v0.0.1-go1.24.0.windows-amd64'
$env:PATH = (Join-Path $env:GOROOT 'bin') + [IO.Path]::PathSeparator + $env:PATH
$env:GOTOOLCHAIN = 'local'
$env:GOFLAGS = '-mod=readonly'
$env:GOPROXY = 'off'
$env:GOSUMDB = 'off'
$env:GOCACHE = Join-Path $workspace 'bin/_authoring_gocache'
$env:GOMODCACHE = Join-Path $workspace 'bin/_authoring_gomodcache'
$env:GOPATH = Join-Path $workspace 'bin/_authoring_gopath'
$env:TEMP = Join-Path $workspace 'bin/_authoringtmp'
$env:TMP = $env:TEMP
$env:GOLANGCI_LINT_CACHE = Join-Path $workspace 'bin/_authoring_golangci_cache'
```

| Gate | Exact command or boundary | Observed exit |
|---|---|---|
| Compiler | `& "$env:GOROOT/bin/go.exe" version` | 0, go1.24.0 windows/amd64 |
| Compilation | `& "$env:GOROOT/bin/go.exe" test '-run=^$' -count=1 ./...` | 0 |
| Scoped RED | Wrapper command above, native `go test` | 1 |
| Nonvacuous guard | Wrapper command above | 1 |
| Vet | `& "$env:GOROOT/bin/go.exe" vet ./...` | 0 |
| Pinned lint | `& ./bin/golangci-lint.exe run --timeout=5m --out-format=json` | 0, `Issues=[]` |
| Format | Cached Go 1.24 `gofmt -l` over all tracked/new Go files | 0, 1,388 files, zero findings |
| Whitespace | `git diff --check` | 0 |

Pinned linter reports v1.64.8, built with go1.26.5. Its build compiler is not the
selected execution toolchain; no host Go 1.27 or linter v2 was used.

Compilation: **PASS**. Red Phase: **CONFIRMED** on Windows.
The failing assertion is the required success check in `without_runtime`:

```text
U1b: acquire must validate the canonical absolute runtime path
inspect runtime\mcp-daemon.lock: get file attributes runtime\mcp-daemon.lock:
The system cannot find the path specified.
```

This is a genuine task-bound assertion failure, not panic, build failure,
missing fixture, or empty selector. The benign-decoy lifecycle phase passed,
as did both security fixture subtests. No repository-wide test-running command
was executed; only the repository-wide compile-only selector was used.

## Safety and handoff

* ProposedAction: test-only reparse regression harness and verification metadata
* ActionRisk: high, bounded to isolated temporary workspaces under
  `careful` + `freeze-scope`
* Approval: operator explicitly authorized fixtures, cleanup, metadata, and commit
* ActionResult: applied; external targets untouched; no production mutation
* Rollback: revert only the harness commit; preserve unrelated operator changes
* Official backlog mutation: only `harness-ready` plus implementation notes
  after compilation and genuine RED; leave these changes unstaged

Engram structural fallback is carried forward without retry: previous MCP daemon
health/binding was green with zero symbols; CLI symbols, map-code, and impact
each reached Ready then timed out at 30 seconds. Exact known source paths were
read directly under the supplied exemption. Intercom was unavailable; progress
was reported locally.

Read-only inspection found GCC on PATH in the existing WinGet WinLibs package.
No global installation occurred. Mandatory task-AC race coverage and unfiltered
Wave 1 convergence GREEN are deferred to the production build phase, not waived.
Stop here at compilable assertion RED, before production changes or admitting U2.

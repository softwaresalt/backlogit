---
description: Bounded PR 645 cycle 2 diagnosis, inherited Linux stdin shutdown repair, and focused validation handoff.
doc_type: reference
source: "docs/memory/2026-11-10/212-S-pr645-ci-cycle2-shim-stdin-memory.md"
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: PR 645 cycle 2 inherited stdin shutdown repair
---

## Scope and baseline

* Recorded at 2026-11-10T21:07:16Z
* Source baseline: `62064e89cf3b00d1daa6f5a1b0bffe040806e6bf`
* Existing 212-S feature branch; all 22 tasks remain done
* Parent-owned `.backlogit/stash.jsonl`, including C2 entry `1CED4AD6`,
  was not edited, staged, restored, or otherwise mutated by this executor
* No commit, push, PR/backlog writes, full repository suite, full-build gate,
  configuration changes, shipment disposition, downloads, or subagents

## Failure and C1 disposition

Hosted run `42898195683`, job `130116953807`, failed only
`TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival`.
At `mcp_session_shim_test.go:382`, the surviving stdio child did not exit
after daemon stop while its parent retained the stdin writer.

C1 passes against plan R2/R11 and U5a/U6: completing the existing shim
detach/shutdown contract requires interrupting and joining the owned input
copy worker. No new transport, lifecycle policy, or public API is introduced.

Linux inherited stdin is a blocking `os.File`, unlike Go-created pollable
pipes. The existing teardown calls `Close` while `io.Copy` has a blocking
read outstanding; closing the file can wait behind that read indefinitely.
The new child-process regression reproduced both peer-EOF and cancellation
hangs before implementation: native exit 1, two 15-second bounded failures.

## Repair

* `internal/cli/mcp_daemon.go`: cancel a session-local context before closing
  owned streams; prepare input before starting the existing two workers
* `internal/cli/mcp_ipc_unix.go`: wrap only file-backed pipe inputs; poll
  readiness in 50 ms intervals and check cancellation before reading
* `internal/cli/mcp_ipc_windows.go`: retain existing Windows input behavior
  through the platform helper
* `internal/cli/mcp_session_input_unix_test.go`: real inherited-stdin child
  regressions for peer EOF, cancellation, and ordinary stdin EOF

No descriptor flags change, no extra workers are introduced, and both
existing workers remain joined. The wrapper deliberately does not embed
`os.File`, whose `WriterTo` fast path would bypass cancellable reads.
Single-reader ownership remains required. Other blocking streams still
require an interruptible `Close`, as documented on `RunMCPSessionShim`.

The existing Windows `u6BuildBinary` private-wrapper dependency was not
touched; this repair did not require editing that build helper.

## Focused evidence

Windows commands use `& bin\tmp\212-go-env.ps1`.
Linux commands run through WSL with the existing workspace-contained
`_ci645-linux-go`, `_ci645-linux-cache`, and `_ci645-linux-home` directories,
the existing module cache, offline dependency resolution, and local toolchain.

* Linux `go test -timeout=90s ./internal/cli -run
  "^TestU6_InheritedStdinShutdown$" -count=1`: RED before repair, exit 1
* Same Linux command after repair: PASS, exit 0, 0.491s
* Linux regression `-count=10`, peer EOF and cancellation: PASS,
  exit 0, 3.606s
* Final Linux regression `-count=10`, all three cases: PASS,
  exit 0, 3.555s
* Windows `go test -timeout=5m ./internal/cli -run
  '^(TestU4_|TestU4a_|TestU5_|TestU5a_|TestU6_)' -count=1`:
  PASS, exit 0, 34.822s
* Windows and Linux `go vet ./internal/cli`: PASS, exit 0
* Existing `bin\golangci-lint.exe run ./internal/cli`, Windows and
  `GOOS=linux` analysis: PASS, exit 0
* `gofmt -l` on all four changed Go files: no output, exit 0
* Scoped `git diff --check`: PASS, exit 0

## Limits and next owner

WSL's repository mount cannot bind Unix-domain sockets. Linux regression
proof uses actual inherited child stdin plus `net.Pipe`, independent of UDS.
It proves the blocking-read teardown repair, not end-to-end Linux daemon
integration. Hosted rerun remains necessary.

Parent owns full-suite receipts, review, full build, commit, push, and CI
gates. Existing C2 residuals, core Git-ceiling behavior, and task states
remain untouched.

## Parent review disposition and preserved evidence

Ordinal 19 is a consumed PASS on the source baseline, not evidence for
this new production delta: 46 packages, 3,922 top-level passes, zero
failures, exits 0 and valid evidence. Its result is
`bin/_rrc1645a19/pr645-ci1-ordinal19/result.json`, SHA-256
`79825a5724e5cf9202650e0f91a0870420776349cc8e3deb811a0a01d1ad0222`.
Historical failed count remains 14 without reset. The current fix-CI
loop has two failed hosted `test` runs and is on remediation cycle 2.

Copilot review `5849123381` on the baseline reported one linked finding
in `internal/core/gate/baseref.go`: an ambient `TEMP`/`TMPDIR` ceiling
can exclude a valid repository root. Read-only classification confirmed
that correcting it would change the explicitly mandated M22/U1k
contract, so P-021 C1 fails for that separate expansion.

Capture-only stash entry **1CED4AD6** was created and re-read before
reply **4530424016** cited it; bot thread
`PRRT_kwDORzozKM615Onq` was then resolved and the PR residual-risk
record updated. The entry was not edited after creation.
It records `DISCOVERY-STATUS: AMBIGUOUS 60D799D9` and
`DISCOVERY-STATUS: LOOKUP-UNAVAILABLE` for archived-stash discovery.
The earlier U1b candidate was not positively confirmed as the later
U1k expansion and was not reused.

Both review rounds were inspected for threadless body findings.
There were none: the baseline's one open item linked to its thread.
No trusted body-disposition marker was fabricated or required.
P-018 returned SATISFIED after resolution on the baseline; the upcoming
repair commit must obtain a fresh current-HEAD result.

Commit the scoped source/test repair, this execution memory and the
unchanged single-write stash capture together. Before pushing, require
fresh full-suite/build/static evidence and local review, then fresh
hosted CI and Copilot. No merge is approved, no task is restarted,
and protected historical checkpoints must remain untouched.

## Cycle 2 review-fix 1 socket-backed stdin evidence

Recorded at 2026-11-10T21:25:32Z against HEAD `62064e89` and the
parent-staged tree `b4532ea1023de4cb7925f0d8af401224bd0cf5fe`.
This is the bounded follow-up to the adversarial MEDIUM 2/3 P1 candidate,
not a new CI cycle or an attribution based only on reviewer agreement.

The regression uses `unix.Socketpair(AF_UNIX, SOCK_STREAM|SOCK_CLOEXEC, 0)`.
One owned endpoint is assigned directly to `exec.Cmd.Stdin`; the helper
asserts that inherited fd 0 has `os.ModeSocket`. The parent closes its
duplicate child endpoint after start, but retains the writing endpoint
through peer-EOF and cancellation checks. Ordinary stdin EOF closes that
writing endpoint. Existing command-context cancellation reaps only the
owned child. No filesystem UDS bind or Node installation is required;
`net.Pipe` remains only the simulated daemon connection, not child stdin.

### Observed RED and minimal GREEN

* RED: Linux `go test -timeout=90s ./internal/cli -run
  '^TestU6_InheritedStdinShutdown/socket_' -count=1 -v`, native exit 1,
  30.282s. `socket_peer_eof` and `socket_cancel` each hit the unchanged
  15-second child deadline; `socket_stdin_eof` passed.
* Only after observing RED, `internal/cli/mcp_ipc_unix.go:33` changed the
  file-type gate from `ModeNamedPipe` to `ModeNamedPipe|ModeSocket`.
* GREEN: Linux `go test -timeout=90s ./internal/cli -run
  '^TestU6_InheritedStdinShutdown$' -count=10`, native exit 0, 7.121s.
  All six pipe/socket peer-EOF, cancellation, and ordinary-EOF cases
  passed ten repetitions.
* Windows `& bin\tmp\212-go-env.ps1 go test -timeout=5m ./internal/cli
  -run '^(TestU4_|TestU4a_|TestU5_|TestU5a_|TestU6_)' -count=1`:
  native exit 0, 33.214s.
* Windows `& bin\tmp\212-go-env.ps1 go vet ./internal/cli` and native
  Linux `go vet ./internal/cli`: exit 0.
* Windows `& bin\tmp\212-go-env.ps1 .\bin\golangci-lint.exe run
  ./internal/cli` and the same adapter with `GOOS=linux`: exit 0.
  The first Windows invocation exited 3 because the concurrent Linux
  analysis held golangci-lint's process lock. After that analysis
  completed, the single sequential Windows retry passed.
* `& bin\tmp\212-go-env.ps1 gofmt -l
  internal\cli\mcp_ipc_unix.go
  internal\cli\mcp_session_input_unix_test.go`: exit 0, no output.

Linux commands used `wsl --exec env` and the existing
`bin/tmp/_ci645-linux-go/go/bin/go`, with `GOROOT` at that toolchain,
`GOCACHE=bin/tmp/_ci645-linux-cache`, `HOME` and `TMPDIR` at
`bin/tmp/_ci645-linux-home`, and the existing `_authoring_gopath` and
`_authoring_gomodcache` under `bin`. All paths were workspace-absolute
WSL equivalents. `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and
`GOFLAGS=-mod=readonly` prevented toolchain/dependency downloads.

### Contract and handoff

C1 passes: this completes the same U5a/U6 owned, file-backed stdin
shutdown/drain contract, including its single-reader assumption. No C2
deferral is appropriate for this reproduced incomplete repair.
Production changes comprise exactly one file-type predicate; no
descriptor flags, worker counts, signatures, ownership, poll intervals,
or deadlines changed. Test-only close-on-exec socket creation does not
change the inherited production descriptor's status flags.

The index's six staged blob identities were checked unchanged before
and after validation. The only unstaged changes from the parent-staged
input are this appended evidence, that predicate, and the Unix stdin
fixture. Existing frontmatter keys and values remain intact. No other
files were edited, staged, restored, or stashed; no backlog/PR/lifecycle
mutations, commit, push, full suite, downloads, or subagents occurred.
All 22 tasks remain done.

Actual residuals remain hosted end-to-end Linux verification (WSL mount
cannot bind filesystem UDS), parent-owned fresh full-suite/build/review
receipts for the eventual commit, C2 entry `1CED4AD6`, and the untouched
Windows private-wrapper P2. Focused GREEN is not a new whole-repository
receipt or an assertion of hosted CI success.

Final scoped `git diff --check` exited 0; `git diff --stat` confirmed
exactly the three authorized unstaged files. Compact-context was invoked
with this latest recovery evidence preserved: zero files compacted or
archived, no extra artifacts, and no broader mutation permitted.

## Cycle 3 coverage ownership diagnosis and blocked race prerequisite

Recorded at 2026-11-10T22:33:49Z against clean HEAD
`c574669b0a016040a5ffb0582e2482a6b505289d`. This is repair iteration
3 of 5; the initial f6 baseline is excluded from that count. Two prior
repair iterations failed hosted `test`. No third repaired tree was
submitted, so the next hosted failure must still trigger the three-failed-
repair-iterations halt. No iteration counter was reset.

### Hosted evidence and C1

Read-only `gh run view 42913035454 --job 130168437175 --log-failed`
confirmed the complete failure inventory: only
`TestU6_InheritedStdinShutdown` and its six cases `peer_eof`, `cancel`,
`stdin_eof`, `socket_peer_eof`, `socket_cancel`, and `socket_stdin_eof`.
Each child returned exit status 2 at the unchanged line 85 assertion:
`error generating coverage report: write /dev/stdout: file already closed`.
The original cross-process stdio/disconnect test was not in that inventory.
The read-only log command exited 0.

The exact workflow path `.github/workflows/ci.yml` and hosted command
agree: `go test -race -timeout=30m -coverprofile=coverage.out ./...`.
No explicit `-covermode` is configured; `-race` selects atomic coverage.
Focused reproduction retains instrumentation and narrows only the package
and test selector; generated profiles remain under the existing ignored
workspace-contained underscore-prefixed Linux home directory.

C1 passes for a private stdout duplicate in the new Unix test helper:
the helper currently transfers the Go test runtime's global `os.Stdout`
to `proxyMCPSession`, which correctly closes its owned output. After
the helper returns, test coverage reporting still needs that same
`os.File`. This is fixture ownership, not a newly established production
defect. A private duplicate would transfer only its new descriptor to
the shim, leaving fd 1 and global `os.Stdout` live for the test runtime.
It must retain actual output closure, all six assertions and deadlines,
and must not change descriptor flags on the original or add workers.

### Native Linux attempts and missing prerequisite

All commands used the existing WSL Go toolchain and caches described
above, with offline module/toolchain settings. `HOME`, `TMPDIR`, and
`GOCACHE` remained workspace-contained; no downloads or installs occurred.

* Actual CI instrumentation attempt 1: focused
  `go test -race -timeout=30m -coverprofile=bin/tmp/_ci645-linux-home/_cycle3-red.cover
  ./internal/cli -run '^TestU6_InheritedStdinShutdown$' -count=1 -v`,
  `CGO_ENABLED=1`, native exit 1. In addition to missing `gcc`, PowerShell
  argument parsing split the unquoted profile suffix into `.cover`,
  producing a secondary offline module-lookup error. No tests ran.
* Corrected actual CI instrumentation attempt 2: focused
  `go test -race -timeout=30m -coverprofile
  'bin/tmp/_ci645-linux-home/_cycle3-race-red.cover' ./internal/cli
  -run '^TestU6_InheritedStdinShutdown$' -count=1 -v`,
  `CGO_ENABLED=1`, native exit 1. The sole failure was
  `runtime/cgo: C compiler "gcc" not found`; no tests ran. This confirms
  the missing native compiler independently of the earlier argument
  transport defect. No third attempt was made.
* Diagnostic coverage-only command: focused
  `go test -timeout=30m -coverprofile
  'bin/tmp/_ci645-linux-home/_cycle3-red.cover' -covermode atomic
  ./internal/cli -run '^TestU6_InheritedStdinShutdown$' -count=1 -v`,
  `CGO_ENABLED=0`, native exit 1, package duration 3.970s. All six cases
  reproduced exactly the hosted exit-2 coverage/stdout error; none
  timed out. Reported package coverage was 0.0%. This is real Linux
  instrumented RED, but is explicitly NOT actual race-enabled proof.

Read-only environment inspection found only WSL `Ubuntu-26.04`, no
native `gcc`, `cc`, or `clang` on its PATH, no cached compiler package,
and no existing compiler binary in the workspace tool directories.
The Windows MinGW `gcc.exe` is not a native Linux C compiler. Installing
or downloading a compiler would exceed the explicit scope.

### Blocked handoff and preserved state

The required actual Linux race/coverage RED-before-fix prerequisite
cannot be met with the existing offline toolchain. Therefore no fixture
patch was applied, and no GREEN, repetition, vet, lint, formatting, or
Windows result is claimed for a repaired tree. The coverage-only
diagnostic is not silently substituted for the required race gate.
The only tracked write is this append; existing frontmatter and prior
chronology remain intact. No production code, test assertions, task
state, checkpoints, stash entry `1CED4AD6`, PR, or Git index changed.
No commit, push, full suite, or subagents occurred.

Parent handoff: provide an already available native Linux C compiler or
explicitly revise the validation prerequisite, then reproduce with the
quoted profile argument before applying the private-owned-stdout
fixture correction. Fresh Ordinal 21, review, hosted CI, and current-HEAD
Copilot handling remain parent-owned. Ordinal 20 is evidence only for
the unchanged C574 tree, not a future repair.

Compact-context was invoked at handoff. The current recovery evidence
is preserved in place; zero files compacted or archived, zero additional
artifacts, and no protected checkpoint changes under the bounded scope.

## Cycle 3 authorized fixture ownership correction

Recorded at 2026-11-10T22:38:55Z against unchanged HEAD `c574669b`.
This continues the CURRENT fix-CI cycle 3, not a new iteration; counters
and the next-hosted-failure halt rule above remain unchanged.

The parent clarified that the retained coverage-only atomic RED faithfully
reproduces the exact hosted stderr and authorizes this C1 fixture repair.
The existing `_cycle3-red.cover` profile was confirmed present; the
six-case native RED receipt recorded above was reused, not rerun.
Local race compilation is unavailable because native Linux `gcc` is
missing. No compiler download, install, or further race attempt occurred.
The unchanged race-enabled hosted CI job remains required after push,
not waived or replaced by coverage-only validation.

### Exact test-only correction and ownership

`internal/cli/mcp_session_input_unix_test.go` now duplicates stdout with
`unix.FcntlInt(os.Stdout.Fd(), unix.F_DUPFD_CLOEXEC, 0)`, checks the error,
wraps only that private descriptor in `os.NewFile`, and passes this file
instead of global `os.Stdout` to `proxyMCPSession`. A deferred file close
also covers early helper returns. The existing shim still actually closes
its owned output; no no-op closer or production contract change exists.

The atomic duplication adds close-on-exec only to the new descriptor and
does not change fd 1, its flags, or the global stdout file's closed state.
The shim closes the private file during teardown; deferred `File.Close`
on the same Go file object is harmless after closure and cannot close a
reused descriptor. No duplicate is allocated on duplication failure.
No workers or subprocesses were added. Existing command-context timeout,
exact owned-child cancellation/reaping, six actual inherited pipe/socket
cases, EOF/cancellation assertions, deadlines, and exit-0 guards remain
unchanged. C1 passes: this completes the existing regression fixture's
test-runtime ownership boundary, not a production defect or scope expansion.

### Focused GREEN receipts

All commands below returned native exit 0. Linux used the existing
`wsl --exec env` adapter, workspace-contained `_ci645-linux-go`,
`_ci645-linux-cache`, `_ci645-linux-home`, `_authoring_gopath`, and
`_authoring_gomodcache`, with `CGO_ENABLED=0`, `GOTOOLCHAIN=local`,
`GOPROXY=off`, `GOSUMDB=off`, and `GOFLAGS=-mod=readonly`.

* Linux `go test -timeout=90s -coverprofile
  'bin/tmp/_ci645-linux-home/_cycle3-green.cover' -covermode atomic
  ./internal/cli -run '^TestU6_InheritedStdinShutdown$' -count=10`:
  PASS, 14.564s; all six cases ten times, coverage instrumentation enabled
* Linux `go test -timeout=90s ./internal/cli -run
  '^TestU6_InheritedStdinShutdown$' -count=10`: PASS, 6.995s
* Linux `go vet ./internal/cli`: PASS
* Windows-hosted Linux-target analysis:
  `$env:GOOS='linux'; & bin\tmp\212-go-env.ps1
  .\bin\golangci-lint.exe run ./internal/cli`: PASS
* `& bin\tmp\212-go-env.ps1 gofmt -l
  internal\cli\mcp_session_input_unix_test.go`: PASS, no output
* Windows `& bin\tmp\212-go-env.ps1 go test -timeout=5m ./internal/cli
  -run '^(TestU4_|TestU4a_|TestU5_|TestU5a_|TestU6_)' -count=1`:
  PASS, 36.748s

The coverage percentage is 0.0% because the instrumented shim executes
in helper children rather than the measured parent. This receipt proves
the previously exit-2 helpers now satisfy the unchanged exit-0 guard
with coverage enabled; it is not a coverage-percentage improvement or
race PASS claim.

Only the fixture and this chronology were edited with `apply_patch`;
frontmatter and all previous chronology remain unchanged. No production,
other helper, backlog, PR, index, commit, push, full repository suite,
global tooling, download, install, or subagent action occurred.
Parent owns fresh whole-tree receipts, review, commit/push, hosted
race-enabled CI, and current-HEAD Copilot gates. This handoff is local
focused GREEN only; hosted race verification is explicitly PENDING.

Compact-context was invoked within the two-file write boundary. The
latest recovery record is preserved in place: zero files compacted,
zero archives, and no additional artifacts. Final scoped
`git diff --check` returned exit 0.

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
